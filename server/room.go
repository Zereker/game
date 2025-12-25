package main

import (
	"log/slog"
	"math/rand"
	"sync"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/werewolf"
	pb "github.com/Zereker/werewolf/proto"
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
	Roles   []pb.RoleType
	mu      sync.RWMutex
	logger  *slog.Logger
}

// NewRoom 创建新房间
func NewRoom(name string, roles []pb.RoleType, logger *slog.Logger) *Room {
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

// getRoleCamp 获取角色对应的阵营
func getRoleCamp(role pb.RoleType) pb.Camp {
	switch role {
	case pb.RoleType_ROLE_TYPE_WEREWOLF:
		return pb.Camp_CAMP_EVIL
	default:
		return pb.Camp_CAMP_GOOD
	}
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
	config := werewolf.DefaultGameConfig()
	r.Engine = werewolf.NewEngine(config)

	// 打乱角色顺序
	shuffledRoles := make([]pb.RoleType, len(r.Roles))
	copy(shuffledRoles, r.Roles)
	rand.Shuffle(len(shuffledRoles), func(i, j int) {
		shuffledRoles[i], shuffledRoles[j] = shuffledRoles[j], shuffledRoles[i]
	})

	// 添加玩家到引擎（需要指定角色和阵营）
	i := 0
	for playerID := range r.Players {
		role := shuffledRoles[i]
		camp := getRoleCamp(role)
		r.Engine.AddPlayer(playerID, role, camp)
		i++
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
	r.Engine.OnEvent(func(event *pb.Event) {
		r.handleEvent(event)
	})
}

// handleEvent 处理游戏事件
func (r *Room) handleEvent(event *pb.Event) {
	switch event.Type {
	case pb.EventType_EVENT_TYPE_GAME_ENDED:
		r.handleGameEnded(event)
	case pb.EventType_EVENT_TYPE_KILL, pb.EventType_EVENT_TYPE_POISON:
		r.handlePlayerDied(event)
	}

	// 发送游戏状态更新
	r.SendGameState()
}

// handlePlayerDied 处理玩家死亡事件
func (r *Room) handlePlayerDied(event *pb.Event) {
	msg := protocol.MustNewMessage(protocol.MsgGameEvent, protocol.GameEventData{
		EventType: event.Type,
		Message:   "玩家死亡",
	})

	r.BroadcastMessage(msg)
}

// handleGameEnded 处理游戏结束事件
func (r *Room) handleGameEnded(event *pb.Event) {
	r.mu.Lock()
	r.State = RoomStateFinished
	r.mu.Unlock()

	state := r.Engine.GetState()
	_, winner := state.CheckVictory()
	players := r.convertPlayersInfo(true)

	msg := protocol.MustNewMessage(protocol.MsgGameEnded, protocol.GameEndedData{
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
		ps, ok := state.GetPlayer(playerID)
		if !ok {
			continue
		}

		// 发送游戏开始消息（包含该玩家的角色信息）
		players := r.convertPlayersInfo(false)
		msg := protocol.MustNewMessage(protocol.MsgGameStarted, protocol.GameStartedData{
			RoleType: ps.Role,
			Camp:     ps.Camp,
			Players:  players,
		})

		player.SendMessageDirect(msg)
	}
}

// SendGameState 发送游戏状态给所有玩家
func (r *Room) SendGameState() {
	state := r.Engine.GetState()
	players := r.convertPlayersInfo(false)

	alivePlayers := make([]string, 0)
	for id, ps := range state.Players {
		if ps.Alive {
			alivePlayers = append(alivePlayers, id)
		}
	}

	msg := protocol.MustNewMessage(protocol.MsgGameState, protocol.GameStateData{
		Phase:        state.Phase,
		Round:        state.Round,
		Players:      players,
		AlivePlayers: alivePlayers,
		IsEnded:      state.Phase == pb.PhaseType_PHASE_TYPE_END,
	})

	r.BroadcastMessage(msg)
}

// BroadcastMessage 广播消息给房间内所有玩家
func (r *Room) BroadcastMessage(msg *protocol.Message) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, player := range r.Players {
		player.SendMessageDirect(msg)
	}
}

// convertPlayersInfo 转换玩家信息（控制是否包含角色信息）
func (r *Room) convertPlayersInfo(includeRole bool) []protocol.PlayerInfo {
	state := r.Engine.GetState()
	result := make([]protocol.PlayerInfo, 0, len(r.Players))

	for id, player := range r.Players {
		ps, ok := state.GetPlayer(id)
		if !ok {
			continue
		}

		info := protocol.PlayerInfo{
			ID:       id,
			Username: player.Username,
			IsAlive:  ps.Alive,
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
