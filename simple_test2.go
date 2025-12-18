package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/socket"
)

func main() {
	tcpAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	tcpConn, _ := net.DialTCP("tcp", nil, tcpAddr)

	received := make(chan *protocol.Message, 10)

	codecOpt := socket.CustomCodecOption(protocol.NewCodec())
	errorOpt := socket.OnErrorOption(func(err error) bool {
		fmt.Printf("ERROR: %v\n", err)
		return true
	})
	msgOpt := socket.OnMessageOption(func(m socket.Message) error {
		msg := m.(*protocol.Message)
		fmt.Printf("收到: %s\n", msg.Type)
		received <- msg
		return nil
	})

	conn, _ := socket.NewConn(tcpConn, codecOpt, errorOpt, msgOpt)
	go conn.Run(context.Background())

	// 登录
	fmt.Println("发送登录...")
	loginMsg, _ := protocol.NewLoginMessage("Test")
	conn.Write(loginMsg)

	time.Sleep(1 * time.Second) // 等待登录响应

	// 创建房间
	fmt.Println("发送创建房间...")
	createMsg, _ := protocol.NewCreateRoomMessage("Room", []interface{}{
		"werewolf", "werewolf", "villager", "villager", "seer", "witch",
	})
	conn.Write(createMsg)

	time.Sleep(1 * time.Second) // 等待创建房间响应

	// 等待消息
	fmt.Println("等待消息...")
	for i := 0; i < 5; i++ {
		select {
		case msg := <-received:
			fmt.Printf("✓ %s\n", msg.Type)
		case <-time.After(2 * time.Second):
			fmt.Println("超时")
			return
		}
	}
}
