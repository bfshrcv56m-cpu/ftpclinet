package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

type FTPConfig struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	RemoteDir  string `json:"remoteDir"`
	LocalDir   string `json:"localDir"`
	FileFilter string `json:"fileFilter"`
}

type Config struct {
	Devices []FTPConfig `json:"devices"`
}

func main() {
	// 读取配置文件
	configFile := "config.json"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	if len(config.Devices) == 0 {
		log.Fatal("配置文件中没有设备信息")
	}

	log.Printf("共配置了 %d 个设备，开始并发下载...\n", len(config.Devices))

	// 使用 WaitGroup 并发下载多个设备的日志
	var wg sync.WaitGroup
	for _, device := range config.Devices {
		wg.Add(1)
		go func(dev FTPConfig) {
			defer wg.Done()
			log.Printf("[%s] 开始下载...", dev.Name)
			if err := downloadLogs(dev); err != nil {
				log.Printf("[%s] 下载失败: %v", dev.Name, err)
			} else {
				log.Printf("[%s] 下载完成!", dev.Name)
			}
		}(device)
	}

	wg.Wait()
	log.Println("\n所有设备日志下载完成!")
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

func downloadLogs(config FTPConfig) error {
	// 连接FTP服务器
	conn, err := ftp.Dial(fmt.Sprintf("%s:%s", config.Host, config.Port),
		ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("连接FTP服务器失败: %w", err)
	}
	defer conn.Quit()

	// 登录
	if err := conn.Login(config.Username, config.Password); err != nil {
		return fmt.Errorf("FTP登录失败: %w", err)
	}
	log.Printf("[%s] FTP登录成功", config.Name)

	// 切换到远程目录
	if err := conn.ChangeDir(config.RemoteDir); err != nil {
		return fmt.Errorf("切换到远程目录失败: %w", err)
	}
	log.Printf("[%s] 切换到远程目录: %s", config.Name, config.RemoteDir)

	// 获取文件列表
	entries, err := conn.List(".")
	if err != nil {
		return fmt.Errorf("获取文件列表失败: %w", err)
	}

	// 创建本地保存目录
	if err := os.MkdirAll(config.LocalDir, 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	// 下载文件
	downloadCount := 0
	for _, entry := range entries {
		if entry.Type == ftp.EntryTypeFile {
			// 检查文件过滤规则
			if config.FileFilter != "" && !matchFilter(entry.Name, config.FileFilter) {
				continue
			}

			log.Printf("[%s] 正在下载: %s (大小: %d bytes)", config.Name, entry.Name, entry.Size)

			if err := downloadFile(conn, entry.Name, config.LocalDir); err != nil {
				log.Printf("[%s] 下载文件 %s 失败: %v", config.Name, entry.Name, err)
				continue
			}

			downloadCount++
			log.Printf("[%s] ✓ 下载完成: %s", config.Name, entry.Name)
		}
	}

	log.Printf("[%s] 共下载 %d 个文件", config.Name, downloadCount)
	return nil
}

func downloadFile(conn *ftp.ServerConn, filename, localDir string) error {
	// 获取远程文件
	resp, err := conn.Retr(filename)
	if err != nil {
		return fmt.Errorf("获取远程文件失败: %w", err)
	}
	defer resp.Close()

	// 创建本地文件
	localPath := filepath.Join(localDir, filename)
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件失败: %w", err)
	}
	defer localFile.Close()

	// 复制文件内容
	if _, err := io.Copy(localFile, resp); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

func matchFilter(filename, filter string) bool {
	// 简单的文件名过滤
	if filter == "" {
		return true
	}
	return filepath.Ext(filename) == filter ||
		filepath.Base(filename) == filter
}
