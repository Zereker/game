package main

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/werewolf"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// RoomState 房间状态
type RoomState string

const (
	RoomStateWaiting  RoomState = "WAITING"
	RoomStatePlaying  RoomState = "PLAYING"
	RoomStateFinished RoomState = "FINISHED"
)

// Room 游戏房间
type Room struct {
	ID      string
	Name    string
	Players map[string]*Player // playerID -> Player
	Engine  *werewolf.Engine
	State   RoomState
	Roles   []werewolf.RoleType
	mu      sync.RWMutex
	logger  *slog.Logger
}

// NewRoom 创建新房间
func NewRoom(name string, roles []werewolf.RoleType, logger *slog.Logger) *Room {
	room := &Room{
		ID:      uuid.New().String()[:8], // 使用短ID方便输入
		Name:    name,
		Players: make(map[string]*Player),
		State:   RoomStateWaiting,
		Roles:   roles,
		logger:  logger,
	}
	return room
}

// AddPlayer 添加玩家到房间
func (r *Room) AddPlayer(player *Player) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State != RoomStateWaiting {
		return errors.New("room is not in waiting state")
	}

	if len(r.Players) >= len(r.Roles) {
		return errors.New("room is full")
	}

	r.Players[player.ID] = player
	player.RoomID = r.ID

	r.logger.Info("player joined room",
		"playerID", player.ID,
		"username", player.Username,
		"roomID", r.ID)

	return nil
}

// RemovePlayer 从房间移除玩家
func (r *Room) RemovePlayer(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.Players, playerID)

	r.logger.Info("player left room",
		"playerID", playerID,
		"roomID", r.ID)
}

// SetPlayerReady 设置玩家准备状态
func (r *Room) SetPlayerReady(playerID string, isReady bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, exists := r.Players[playerID]
	if !exists {
		return errors.New("player not in room")
	}

	player.IsReady = isReady

	r.logger.Info("player ready status changed",
		"playerID", playerID,
		"isReady", isReady)

	return nil
}

// CanStart 检查是否可以开始游戏
func (r *Room) CanStart() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.Players) != len(r.Roles) {
		return false
	}

	for _, player := range r.Players {
		if !player.IsReady {
			return false
		}
	}

	return true
}

// Start 开始游戏
func (r *Room) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State != RoomStateWaiting {
		return errors.New("room is not in waiting state")
	}

	if len(r.Players) != len(r.Roles) {
		return errors.Errorf("need %d players, got %d", len(r.Roles), len(r.Players))
	}

	// 创建游戏引擎
	config := werewolf.Config{
		Roles:           r.Roles,
		EnableLastWords: false,
	}

	engine, err := werewolf.NewEngine(config, werewolf.WithLogger(r.logger))
	if err != nil {
		return errors.Wrap(err, "create engine")
	}

	r.Engine = engine

	// 添加玩家到引擎
	for playerID := range r.Players {
		if err := r.Engine.AddPlayer(playerID); err != nil {
			return errors.Wrap(err, "add player to engine")
		}
	}

	// 订阅游戏事件
	r.subscribeEvents()

	// 启动游戏
	if err := r.Engine.Start(); err != nil {
		return errors.Wrap(err, "start engine")
	}

	r.State = RoomStatePlaying

	r.logger.Info("game started", "roomID", r.ID)

	// 通知所有玩家游戏开始（每个玩家看到自己的角色）
	r.notifyGameStarted()

	return nil
}

// subscribeEvents 订阅游戏引擎事件
func (r *Room) subscribeEvents() {
	// 阶段变化
	r.Engine.Subscribe(werewolf.EventPhaseStarted, func(e werewolf.Event) {
		r.handlePhaseStarted(e)
	})

	// 玩家死亡
	r.Engine.Subscribe(werewolf.EventPlayerDied, func(e werewolf.Event) {
		r.handlePlayerDied(e)
	})

	// 游戏结束
	r.Engine.Subscribe(werewolf.EventGameEnded, func(e werewolf.Event) {
		r.handleGameEnded(e)
	})
}

// handlePhaseStarted 处理阶段开始事件
func (r *Room) handlePhaseStarted(e werewolf.Event) {
	data := e.Data.(map[string]interface{})
	phase := data["phase"].(werewolf.PhaseType)

	state := r.Engine.GetState()

	// 广播阶段变化
	msg, _ := protocol.NewMessage(protocol.MsgPhaseChanged, protocol.PhaseChangedData{
		Phase: phase,
		Round: state.Round,
	})

	r.BroadcastMessage(msg)

	// 发送游戏状态
	r.SendGameState()
}

// handlePlayerDied 处理玩家死亡事件
func (r *Room) handlePlayerDied(e werewolf.Event) {
	data := e.Data.(map[string]interface{})
	playerID := data["playerID"].(string)
	reason := data["reason"].(string)

	msg, _ := protocol.NewMessage(protocol.MsgGameEvent, protocol.GameEventData{
		EventType: werewolf.EventPlayerDied,
		Message:   fmt.Sprintf("玩家 %s 死亡: %s", playerID, reason),
		Data:      data,
	})

	r.BroadcastMessage(msg)
}

// handleGameEnded 处理游戏结束事件
func (r *Room) handleGameEnded(e werewolf.Event) {
	r.mu.Lock()
	r.State = RoomStateFinished
	r.mu.Unlock()

	data := e.Data.(map[string]interface{})
	winner := data["winner"].(werewolf.Camp)

	state := r.Engine.GetState()
	players := r.convertPlayersInfo(state.Players, true)

	msg, _ := protocol.NewMessage(protocol.MsgGameEnded, protocol.GameEndedData{
		Winner:  winner,
		Players: players,
	})

	r.BroadcastMessage(msg)

	r.logger.Info("game ended", "roomID", r.ID, "winner", winner)
}

// notifyGameStarted 通知所有玩家游戏开始
func (r *Room) notifyGameStarted() {
	state := r.Engine.GetState()

	for playerID, player := range r.Players {
		// 找到该玩家的角色
		var roleType werewolf.RoleType
		var camp werewolf.Camp

		for _, ps := range state.Players {
			if ps.ID == playerID {
				roleType = ps.Role
				role := r.Engine.GetPlayerRole(playerID)
				if role != nil {
					camp = role.GetCamp()
				}
				break
			}
		}

		// 发送游戏开始消息（包含该玩家的角色信息）
		players := r.convertPlayersInfo(state.Players, false)
		msg, _ := protocol.NewMessage(protocol.MsgGameStarted, protocol.GameStartedData{
			RoleType: roleType,
			Camp:     camp,
			Players:  players,
		})

		player.SendMessage(msg)
	}
}

// SendGameState 发送游戏状态给所有玩家
func (r *Room) SendGameState() {
	state := r.Engine.GetState()
	players := r.convertPlayersInfo(state.Players, false)

	msg, _ := protocol.NewMessage(protocol.MsgGameState, protocol.GameStateData{
		Phase:        state.Phase,
		Round:        state.Round,
		Players:      players,
		AlivePlayers: state.AlivePlayers,
		IsEnded:      state.IsEnded,
	})

	r.BroadcastMessage(msg)
}

// BroadcastMessage 广播消息给房间内所有玩家
func (r *Room) BroadcastMessage(msg *protocol.Message) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, player := range r.Players {
		player.SendMessage(msg)
	}
}

// convertPlayersInfo 转换玩家信息（控制是否包含角色信息）
func (r *Room) convertPlayersInfo(players []werewolf.PlayerState, includeRole bool) []protocol.PlayerInfo {
	result := make([]protocol.PlayerInfo, 0, len(players))

	for _, ps := range players {
		player, exists := r.Players[ps.ID]
		if !exists {
			continue
		}

		info := protocol.PlayerInfo{
			ID:       ps.ID,
			Username: player.Username,
			IsAlive:  ps.IsAlive,
			IsReady:  player.IsReady,
		}

		if includeRole {
			info.RoleType = ps.Role
		}

		result = append(result, info)
	}

	return result
}

// GetPlayerList 获取房间内玩家列表
func (r *Room) GetPlayerList() []protocol.PlayerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]protocol.PlayerInfo, 0, len(r.Players))
	for _, player := range r.Players {
		result = append(result, protocol.PlayerInfo{
			ID:       player.ID,
			Username: player.Username,
			IsReady:  player.IsReady,
			IsAlive:  true,
		})
	}

	return result
}
