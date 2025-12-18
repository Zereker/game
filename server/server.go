package main

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/socket"
	"github.com/Zereker/werewolf"
)

// Server 游戏服务器
type Server struct {
	rooms      map[string]*Room    // roomID -> Room
	players    map[string]*Player  // playerID -> Player
	connID     int64              // 连接ID计数器
	mu         sync.RWMutex
	handler    *MessageHandler
	logger     *slog.Logger
}

// NewServer 创建新服务器
func NewServer(logger *slog.Logger) *Server {
	server := &Server{
		rooms:   make(map[string]*Room),
		players: make(map[string]*Player),
		logger:  logger,
	}

	server.handler = NewMessageHandler(server, logger)

	return server
}

// CreateRoom 创建房间
func (s *Server) CreateRoom(name string, roles []werewolf.RoleType) (*Room, error) {
	room := NewRoom(name, roles, s.logger)

	s.mu.Lock()
	s.rooms[room.ID] = room
	s.mu.Unlock()

	s.logger.Info("room created",
		"roomID", room.ID,
		"name", name,
		"roles", roles)

	return room, nil
}

// GetRoom 获取房间
func (s *Server) GetRoom(roomID string) *Room {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rooms[roomID]
}

// GetPlayer 获取玩家
func (s *Server) GetPlayer(playerID string) *Player {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.players[playerID]
}

// AddPlayer 添加玩家
func (s *Server) AddPlayer(player *Player) {
	s.mu.Lock()
	s.players[player.ID] = player
	s.mu.Unlock()

	s.logger.Info("player added", "playerID", player.ID)
}

// RemovePlayer 移除玩家
func (s *Server) RemovePlayer(playerID string) {
	s.mu.Lock()
	player, exists := s.players[playerID]
	if !exists {
		s.mu.Unlock()
		return
	}

	// 从房间中移除
	if player.RoomID != "" {
		if room := s.rooms[player.RoomID]; room != nil {
			room.RemovePlayer(playerID)

			// 通知房间内其他玩家
			leftMsg := protocol.MustNewMessage(protocol.MsgPlayerLeft, protocol.PlayerLeftData{
				PlayerID: playerID,
			})
			room.BroadcastMessage(leftMsg)
		}
	}

	delete(s.players, playerID)
	s.mu.Unlock()

	s.logger.Info("player removed", "playerID", playerID)
}

// HandleConnection 处理客户端连接
func (s *Server) HandleConnection(conn *net.TCPConn) {
	connID := atomic.AddInt64(&s.connID, 1)

	s.logger.Info("new connection",
		"connID", connID,
		"addr", conn.RemoteAddr())

	// 创建临时玩家（等待登录）
	tempPlayerID := ""
	var socketConn *socket.Conn

	// 配置连接选项
	codecOption := socket.CustomCodecOption(protocol.NewCodec())

	onErrorOption := socket.OnErrorOption(func(err error) bool {
		s.logger.Error("connection error",
			"connID", connID,
			"error", err)
		return true // 断开连接
	})

	onMessageOption := socket.OnMessageOption(func(m socket.Message) error {
		msg := m.(*protocol.Message)

		// 如果是登录消息，创建玩家
		if msg.Type == protocol.MsgLogin {
			var loginData protocol.LoginData
			if err := msg.UnmarshalData(&loginData); err != nil {
				return err
			}

			// 创建玩家（先不设置Conn，因为socketConn还未初始化）
			player := NewPlayer(loginData.Username, nil)
			tempPlayerID = player.ID

			// 在添加到服务器后，立即更新Conn（此时socketConn已经有值了）
			player.Conn = socketConn
			s.AddPlayer(player)

			// 发送登录成功消息 (使用同步发送确保消息立即发出)
			respMsg := protocol.MustNewMessage(protocol.MsgLoginSuccess, protocol.LoginSuccessData{
				PlayerID: player.ID,
			})

			return socketConn.WriteDirect(respMsg)
		}

		// 处理其他消息
		if tempPlayerID == "" {
			errMsg := protocol.MustNewMessage(protocol.MsgError, protocol.ErrorData{Message: "please login first"})
			socketConn.WriteDirect(errMsg)
			return nil
		}

		// 委托给消息处理器
		if err := s.handler.HandleMessage(tempPlayerID, msg); err != nil {
			s.logger.Error("handle message error",
				"playerID", tempPlayerID,
				"type", msg.Type,
				"error", err)

			// 发送错误消息 (使用同步发送)
			errMsg := protocol.MustNewMessage(protocol.MsgError, protocol.ErrorData{Message: err.Error()})
			if player := s.GetPlayer(tempPlayerID); player != nil {
				player.SendMessageDirect(errMsg)
			}
		}

		return nil
	})

	// 创建连接
	var err error
	socketConn, err = socket.NewConn(conn, codecOption, onErrorOption, onMessageOption)
	if err != nil {
		s.logger.Error("create connection error", "error", err)
		conn.Close()
		return
	}

	// 运行连接（阻塞直到连接关闭）
	if err := socketConn.Run(context.Background()); err != nil {
		s.logger.Error("connection run error", "error", err)
	}

	// 清理玩家
	if tempPlayerID != "" {
		s.RemovePlayer(tempPlayerID)
	}

	s.logger.Info("connection closed", "connID", connID)
}
