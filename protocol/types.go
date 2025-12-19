package protocol

import "github.com/Zereker/werewolf"

// MessageType 定义所有消息类型
type MessageType string

const (
	// 客户端 -> 服务器
	MsgLogin         MessageType = "LOGIN"
	MsgCreateRoom    MessageType = "CREATE_ROOM"
	MsgJoinRoom      MessageType = "JOIN_ROOM"
	MsgReady         MessageType = "READY"
	MsgPerformAction MessageType = "PERFORM_ACTION"
	MsgAdvancePhase  MessageType = "ADVANCE_PHASE" // 推进游戏阶段

	// 服务器 -> 客户端
	MsgLoginSuccess  MessageType = "LOGIN_SUCCESS"
	MsgRoomCreated   MessageType = "ROOM_CREATED"
	MsgRoomJoined    MessageType = "ROOM_JOINED"
	MsgPlayerJoined  MessageType = "PLAYER_JOINED"
	MsgPlayerLeft    MessageType = "PLAYER_LEFT"
	MsgPlayerReady   MessageType = "PLAYER_READY"
	MsgGameStarted   MessageType = "GAME_STARTED"
	MsgPhaseChanged  MessageType = "PHASE_CHANGED"
	MsgGameState     MessageType = "GAME_STATE"
	MsgGameEvent     MessageType = "GAME_EVENT"
	MsgActionResult  MessageType = "ACTION_RESULT"
	MsgGameEnded     MessageType = "GAME_ENDED"
	MsgError         MessageType = "ERROR"
)

// LoginData 登录消息数据
type LoginData struct {
	Username string `json:"username"`
}

// CreateRoomData 创建房间消息数据
type CreateRoomData struct {
	RoomName string           `json:"roomName"`
	Roles    []werewolf.RoleType `json:"roles"`
}

// JoinRoomData 加入房间消息数据
type JoinRoomData struct {
	RoomID string `json:"roomID"`
}

// PerformActionData 执行动作消息数据
type PerformActionData struct {
	ActionType werewolf.ActionType `json:"actionType"`
	TargetID   string              `json:"targetID,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// AdvancePhaseData 推进阶段消息数据
type AdvancePhaseData struct {
	Phase werewolf.PhaseType `json:"phase"`
}

// LoginSuccessData 登录成功消息数据
type LoginSuccessData struct {
	PlayerID string `json:"playerID"`
}

// RoomCreatedData 房间创建成功消息数据
type RoomCreatedData struct {
	RoomID string `json:"roomID"`
}

// RoomJoinedData 加入房间成功消息数据
type RoomJoinedData struct {
	RoomID  string       `json:"roomID"`
	Players []PlayerInfo `json:"players"`
}

// PlayerJoinedData 玩家加入消息数据
type PlayerJoinedData struct {
	Player PlayerInfo `json:"player"`
}

// PlayerLeftData 玩家离开消息数据
type PlayerLeftData struct {
	PlayerID string `json:"playerID"`
}

// PlayerReadyData 玩家准备消息数据
type PlayerReadyData struct {
	PlayerID string `json:"playerID"`
	IsReady  bool   `json:"isReady"`
}

// GameStartedData 游戏开始消息数据
type GameStartedData struct {
	RoleType werewolf.RoleType `json:"roleType"`
	Camp     werewolf.Camp     `json:"camp"`
	Players  []PlayerInfo      `json:"players"`
}

// PhaseChangedData 阶段变化消息数据
type PhaseChangedData struct {
	Phase werewolf.PhaseType `json:"phase"`
	Round int                `json:"round"`
}

// GameStateData 游戏状态消息数据
type GameStateData struct {
	Phase        werewolf.PhaseType `json:"phase"`
	Round        int                `json:"round"`
	Players      []PlayerInfo       `json:"players"`
	AlivePlayers []string           `json:"alivePlayers"`
	IsEnded      bool               `json:"isEnded"`
}

// GameEventData 游戏事件消息数据
type GameEventData struct {
	EventType werewolf.EventType `json:"eventType"`
	Message   string             `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ActionResultData 动作结果消息数据
type ActionResultData struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// GameEndedData 游戏结束消息数据
type GameEndedData struct {
	Winner  werewolf.Camp `json:"winner"`
	Players []PlayerInfo  `json:"players"`
}

// ErrorData 错误消息数据
type ErrorData struct {
	Message string `json:"message"`
}

// PlayerInfo 玩家信息
type PlayerInfo struct {
	ID       string            `json:"id"`
	Username string            `json:"username"`
	IsAlive  bool              `json:"isAlive"`
	IsReady  bool              `json:"isReady"`
	RoleType werewolf.RoleType `json:"roleType,omitempty"` // 只在特定情况下发送
}
