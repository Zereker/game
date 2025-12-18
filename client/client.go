package main

import (
	"context"
	"log/slog"
	"net"
	"sync"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/socket"
	"github.com/Zereker/werewolf"
	"github.com/pkg/errors"
)

// ClientState 客户端状态
type ClientState struct {
	PlayerID     string
	Username     string
	RoomID       string
	MyRole       werewolf.RoleType
	MyCamp       werewolf.Camp
	GamePhase    werewolf.PhaseType
	Round        int
	Players      []protocol.PlayerInfo
	AlivePlayers []string
	Events       []string
	IsInGame     bool
}

// Client 客户端
type Client struct {
	conn    *socket.Conn
	state   *ClientState
	ui      *UI
	input   *InputHandler
	logger  *slog.Logger
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewClient 创建新客户端
func NewClient(logger *slog.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		state: &ClientState{
			Events: make([]string, 0),
		},
		ui:     NewUI(),
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	client.input = NewInputHandler(client)

	return client
}

// Connect 连接服务器
func (c *Client) Connect(addr string) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "resolve address")
	}

	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return errors.Wrap(err, "dial tcp")
	}

	// 配置连接选项
	codecOption := socket.CustomCodecOption(protocol.NewCodec())

	onErrorOption := socket.OnErrorOption(func(err error) bool {
		c.logger.Error("connection error", "error", err)
		return true // 断开连接
	})

	onMessageOption := socket.OnMessageOption(func(m socket.Message) error {
		msg := m.(*protocol.Message)
		return c.handleMessage(msg)
	})

	// 创建连接
	conn, err := socket.NewConn(tcpConn, codecOption, onErrorOption, onMessageOption)
	if err != nil {
		return errors.Wrap(err, "create connection")
	}

	c.conn = conn

	c.logger.Info("connected to server", "addr", addr)

	// 在后台运行连接
	go func() {
		if err := c.conn.Run(c.ctx); err != nil {
			c.logger.Error("connection run error", "error", err)
		}
	}()

	return nil
}

// SendMessage 发送消息
func (c *Client) SendMessage(msg *protocol.Message) error {
	if c.conn == nil {
		return errors.New("not connected")
	}

	return c.conn.Write(msg)
}

// handleMessage 处理服务器消息
func (c *Client) handleMessage(msg *protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("received message", "type", msg.Type)

	switch msg.Type {
	case protocol.MsgLoginSuccess:
		return c.handleLoginSuccess(msg)
	case protocol.MsgRoomCreated:
		return c.handleRoomCreated(msg)
	case protocol.MsgRoomJoined:
		return c.handleRoomJoined(msg)
	case protocol.MsgPlayerJoined:
		return c.handlePlayerJoined(msg)
	case protocol.MsgPlayerLeft:
		return c.handlePlayerLeft(msg)
	case protocol.MsgPlayerReady:
		return c.handlePlayerReady(msg)
	case protocol.MsgGameStarted:
		return c.handleGameStarted(msg)
	case protocol.MsgPhaseChanged:
		return c.handlePhaseChanged(msg)
	case protocol.MsgGameState:
		return c.handleGameState(msg)
	case protocol.MsgGameEvent:
		return c.handleGameEvent(msg)
	case protocol.MsgActionResult:
		return c.handleActionResult(msg)
	case protocol.MsgGameEnded:
		return c.handleGameEnded(msg)
	case protocol.MsgError:
		return c.handleError(msg)
	default:
		c.logger.Warn("unknown message type", "type", msg.Type)
	}

	return nil
}

// handleLoginSuccess 处理登录成功
func (c *Client) handleLoginSuccess(msg *protocol.Message) error {
	var data protocol.LoginSuccessData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.PlayerID = data.PlayerID
	c.addEvent("登录成功，玩家ID: " + data.PlayerID)
	c.Render()

	return nil
}

// handleRoomCreated 处理房间创建
func (c *Client) handleRoomCreated(msg *protocol.Message) error {
	var data protocol.RoomCreatedData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.RoomID = data.RoomID
	c.addEvent("房间创建成功，房间ID: " + data.RoomID)

	return nil
}

// handleRoomJoined 处理加入房间
func (c *Client) handleRoomJoined(msg *protocol.Message) error {
	var data protocol.RoomJoinedData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.RoomID = data.RoomID
	c.state.Players = data.Players
	c.addEvent("加入房间: " + data.RoomID)
	c.Render()

	return nil
}

// handlePlayerJoined 处理玩家加入
func (c *Client) handlePlayerJoined(msg *protocol.Message) error {
	var data protocol.PlayerJoinedData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.Players = append(c.state.Players, data.Player)
	c.addEvent("玩家加入: " + data.Player.Username)
	c.Render()

	return nil
}

// handlePlayerLeft 处理玩家离开
func (c *Client) handlePlayerLeft(msg *protocol.Message) error {
	var data protocol.PlayerLeftData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	// 从玩家列表中移除
	for i, p := range c.state.Players {
		if p.ID == data.PlayerID {
			c.state.Players = append(c.state.Players[:i], c.state.Players[i+1:]...)
			break
		}
	}

	c.addEvent("玩家离开: " + data.PlayerID)
	c.Render()

	return nil
}

// handlePlayerReady 处理玩家准备
func (c *Client) handlePlayerReady(msg *protocol.Message) error {
	var data protocol.PlayerReadyData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	// 更新玩家准备状态
	for i, p := range c.state.Players {
		if p.ID == data.PlayerID {
			c.state.Players[i].IsReady = data.IsReady
			break
		}
	}

	status := "准备"
	if !data.IsReady {
		status = "取消准备"
	}

	c.addEvent("玩家" + data.PlayerID + status)
	c.Render()

	return nil
}

// handleGameStarted 处理游戏开始
func (c *Client) handleGameStarted(msg *protocol.Message) error {
	var data protocol.GameStartedData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.MyRole = data.RoleType
	c.state.MyCamp = data.Camp
	c.state.Players = data.Players
	c.state.IsInGame = true
	c.state.Round = 1
	c.addEvent("游戏开始！")
	c.Render()

	return nil
}

// handlePhaseChanged 处理阶段变化
func (c *Client) handlePhaseChanged(msg *protocol.Message) error {
	var data protocol.PhaseChangedData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.GamePhase = data.Phase
	c.state.Round = data.Round

	phaseName := c.ui.phaseName(data.Phase)
	c.addEvent("阶段变化: " + phaseName)
	c.Render()

	return nil
}

// handleGameState 处理游戏状态
func (c *Client) handleGameState(msg *protocol.Message) error {
	var data protocol.GameStateData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.GamePhase = data.Phase
	c.state.Round = data.Round
	c.state.Players = data.Players
	c.state.AlivePlayers = data.AlivePlayers

	c.Render()

	return nil
}

// handleGameEvent 处理游戏事件
func (c *Client) handleGameEvent(msg *protocol.Message) error {
	var data protocol.GameEventData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.addEvent(data.Message)
	c.Render()

	return nil
}

// handleActionResult 处理动作结果
func (c *Client) handleActionResult(msg *protocol.Message) error {
	var data protocol.ActionResultData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	if data.Success {
		c.addEvent("✓ " + data.Message)
	} else {
		c.addEvent("✗ " + data.Message)
	}

	c.Render()

	return nil
}

// handleGameEnded 处理游戏结束
func (c *Client) handleGameEnded(msg *protocol.Message) error {
	var data protocol.GameEndedData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.state.IsInGame = false
	c.state.Players = data.Players

	winnerName := c.ui.campName(data.Winner)
	c.addEvent("游戏结束！获胜阵营: " + winnerName)
	c.Render()

	return nil
}

// handleError 处理错误消息
func (c *Client) handleError(msg *protocol.Message) error {
	var data protocol.ErrorData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	c.addEvent("错误: " + data.Message)
	c.Render()

	return nil
}

// addEvent 添加事件到日志
func (c *Client) addEvent(event string) {
	c.state.Events = append(c.state.Events, event)
}

// Render 渲染UI
func (c *Client) Render() {
	c.ui.Clear()

	// 打印标题
	c.ui.PrintHeader(c.state.RoomID, c.state.Round, c.state.GamePhase)

	// 如果在游戏中，显示玩家列表
	if len(c.state.Players) > 0 {
		c.ui.PrintPlayers(c.state.Players, c.state.PlayerID)
	}

	// 显示事件日志
	c.ui.PrintEvents(c.state.Events)

	// 如果在游戏中，显示角色信息
	if c.state.IsInGame {
		c.ui.PrintRoleInfo(c.state.MyRole, c.state.MyCamp)
	}
}

// Run 运行客户端主循环
func (c *Client) Run() {
	// 初始渲染
	c.Render()

	// 主输入循环
	for {
		c.ui.PrintPrompt(c.state.GamePhase, c.state.MyRole)

		cmd, err := c.input.ReadCommand()
		if err != nil {
			c.logger.Error("read command error", "error", err)
			continue
		}

		if err := c.input.HandleCommand(cmd); err != nil {
			c.ui.PrintError(err.Error())
		}
	}
}

// Close 关闭客户端
func (c *Client) Close() {
	c.cancel()
	if c.conn != nil {
		// socket 包没有提供 Close 方法，通过 cancel context 来关闭
	}
}
