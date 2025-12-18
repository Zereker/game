package main

import (
	"flag"
	"log"
	"log/slog"
	"os"
)

func main() {
	// 解析命令行参数
	addr := flag.String("addr", "127.0.0.1:8888", "server address")
	flag.Parse()

	// 创建日志
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // 客户端只显示错误日志，避免干扰UI
	}))

	// 创建客户端
	client := NewClient(logger)
	defer client.Close()

	// 连接服务器
	if err := client.Connect(*addr); err != nil {
		log.Fatalf("连接服务器失败: %v", err)
	}

	// 运行客户端
	client.Run()
}
