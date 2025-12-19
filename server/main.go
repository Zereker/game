package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/Zereker/socket"
)

func main() {
	// 解析命令行参数
	addr := flag.String("addr", "127.0.0.1:8888", "server address")
	flag.Parse()

	// 创建日志
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// 创建服务器
	server := NewServer(logger)

	// 解析地址
	tcpAddr, err := net.ResolveTCPAddr("tcp", *addr)
	if err != nil {
		log.Fatalf("resolve address error: %v", err)
	}

	// 创建 TCP 服务器
	tcpServer, err := socket.New(tcpAddr)
	if err != nil {
		log.Fatalf("create server error: %v", err)
	}

	logger.Info("server started", "addr", *addr)
	logger.Info("waiting for players to connect...")

	// 启动服务器（阻塞）
	tcpServer.Serve(context.Background(), server)
}

// Handle 实现 socket.Handler 接口
func (s *Server) Handle(conn *net.TCPConn) {
	s.HandleConnection(conn)
}
