package main

import (
	"context"
	"time"

	"github.com/Zereker/socket"
	"github.com/google/uuid"
)

// Player 玩家
type Player struct {
	ID       string
	Username string
	Conn     *socket.Conn
	RoomID   string
	IsReady  bool
}

// NewPlayer 创建新玩家
func NewPlayer(username string, conn *socket.Conn) *Player {
	return &Player{
		ID:       uuid.New().String(),
		Username: username,
		Conn:     conn,
		IsReady:  false,
	}
}

// SendMessage 发送消息给玩家 (通过channel异步发送)
func (p *Player) SendMessage(msg socket.Message) error {
	if p.Conn == nil {
		return nil
	}
	return p.Conn.Write(msg)
}

// SendMessageDirect 直接同步发送消息 (阻塞直到发送完成)
func (p *Player) SendMessageDirect(msg socket.Message) error {
	if p.Conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return p.Conn.WriteBlocking(ctx, msg)
}
