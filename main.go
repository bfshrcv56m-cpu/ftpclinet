package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jlaffaye/ftp"
)

type FTPConfig struct {
	Host       string
	Port       string
	Username   string
	Password   string
	RemoteDir  string
	LocalDir   string
	FileFilter string // 文件过滤规则，如 "*.log"
}

func main() {
	// 配置FTP连接信息
	config := FTPConfig{
		Host:       "192.168.1.100", // 修改为你的设备IP
		Port:       "21",
		Username:   "your_username", // 修改为你的FTP用户名
		Password:   "your_password", // 修改为你的FTP密码
		RemoteDir:  "/mnt/mmcblk0k1/syslogs",
		LocalDir:   "./logs", // 本地保存目录
		FileFilter: ".log",   // 只下载.log文件
	}

	log.Printf("开始连接FTP服务器: %s:%s", config.Host, config.Port)
	
	if err := downloadLogs(config); err != nil {
		log.Fatalf("下载日志失败: %v", err)
	}
	
	log.Println("日志下载完成!")
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
	log.Println("FTP登录成功")

	// 切换到远程目录
	if err := conn.ChangeDir(config.RemoteDir); err != nil {
		return fmt.Errorf("切换到远程目录失败: %w", err)
	}
	log.Printf("切换到远程目录: %s", config.RemoteDir)

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

			log.Printf("正在下载: %s (大小: %d bytes)", entry.Name, entry.Size)
			
			if err := downloadFile(conn, entry.Name, config.LocalDir); err != nil {
				log.Printf("下载文件 %s 失败: %v", entry.Name, err)
				continue
			}
			
			downloadCount++
			log.Printf("✓ 下载完成: %s", entry.Name)
		}
	}

	log.Printf("共下载 %d 个文件", downloadCount)
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
