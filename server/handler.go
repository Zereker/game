package main

import (
	"log/slog"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/werewolf"
	"github.com/pkg/errors"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	server *Server
	logger *slog.Logger
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(server *Server, logger *slog.Logger) *MessageHandler {
	return &MessageHandler{
		server: server,
		logger: logger,
	}
}

// HandleMessage 处理消息
func (h *MessageHandler) HandleMessage(playerID string, msg *protocol.Message) error {
	h.logger.Info("handle message",
		"playerID", playerID,
		"type", msg.Type)

	switch msg.Type {
	case protocol.MsgLogin:
		return h.handleLogin(playerID, msg)
	case protocol.MsgCreateRoom:
		return h.handleCreateRoom(playerID, msg)
	case protocol.MsgJoinRoom:
		return h.handleJoinRoom(playerID, msg)
	case protocol.MsgReady:
		return h.handleReady(playerID, msg)
	case protocol.MsgPerformAction:
		return h.handlePerformAction(playerID, msg)
	default:
		return errors.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleLogin 处理登录
func (h *MessageHandler) handleLogin(playerID string, msg *protocol.Message) error {
	var data protocol.LoginData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	player := h.server.GetPlayer(playerID)
	if player == nil {
		return errors.New("player not found")
	}

	player.Username = data.Username

	// 发送登录成功消息
	respMsg := protocol.MustNewMessage(protocol.MsgLoginSuccess, protocol.LoginSuccessData{
		PlayerID: playerID,
	})

	return player.SendMessageDirect(respMsg)
}

// handleCreateRoom 处理创建房间
func (h *MessageHandler) handleCreateRoom(playerID string, msg *protocol.Message) error {
	var data map[string]interface{}
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	// 安全类型断言
	roomNameRaw, ok := data["roomName"]
	if !ok {
		return errors.New("missing roomName field")
	}
	roomName, ok := roomNameRaw.(string)
	if !ok {
		return errors.New("roomName must be a string")
	}
	// 输入验证
	if len(roomName) == 0 || len(roomName) > 50 {
		return errors.New("roomName must be 1-50 characters")
	}

	// 解析角色配置
	var roles []werewolf.RoleType
	if rolesData, ok := data["roles"].([]interface{}); ok && len(rolesData) > 0 {
		for _, r := range rolesData {
			roles = append(roles, werewolf.RoleType(r.(string)))
		}
	} else {
		// 默认6人局配置
		roles = []werewolf.RoleType{
			werewolf.RoleTypeWerewolf,
			werewolf.RoleTypeWerewolf,
			werewolf.RoleTypeVillager,
			werewolf.RoleTypeVillager,
			werewolf.RoleTypeSeer,
			werewolf.RoleTypeWitch,
		}
	}

	room, err := h.server.CreateRoom(roomName, roles)
	if err != nil {
		return err
	}

	// 创建者自动加入房间
	player := h.server.GetPlayer(playerID)
	if err := room.AddPlayer(player); err != nil {
		return err
	}

	// 发送房间创建成功消息
	respMsg := protocol.MustNewMessage(protocol.MsgRoomCreated, protocol.RoomCreatedData{
		RoomID: room.ID,
	})

	h.logger.Info("sending room created message", "roomID", room.ID)
	if err := player.SendMessageDirect(respMsg); err != nil {
		h.logger.Error("failed to send room created message", "error", err)
		return err
	}
	h.logger.Info("room created message sent")

	// 发送房间加入成功消息
	joinedMsg := protocol.MustNewMessage(protocol.MsgRoomJoined, protocol.RoomJoinedData{
		RoomID:  room.ID,
		Players: room.GetPlayerList(),
	})

	h.logger.Info("sending room joined message", "roomID", room.ID)
	err = player.SendMessageDirect(joinedMsg)
	if err != nil {
		h.logger.Error("failed to send room joined message", "error", err)
	} else {
		h.logger.Info("room joined message sent")
	}
	return err
}

// handleJoinRoom 处理加入房间
func (h *MessageHandler) handleJoinRoom(playerID string, msg *protocol.Message) error {
	var data protocol.JoinRoomData
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	room := h.server.GetRoom(data.RoomID)
	if room == nil {
		return errors.New("room not found")
	}

	player := h.server.GetPlayer(playerID)
	if err := room.AddPlayer(player); err != nil {
		return err
	}

	// 发送加入成功消息给该玩家 (使用同步发送)
	joinedMsg := protocol.MustNewMessage(protocol.MsgRoomJoined, protocol.RoomJoinedData{
		RoomID:  room.ID,
		Players: room.GetPlayerList(),
	})

	if err := player.SendMessageDirect(joinedMsg); err != nil {
		return err
	}

	// 通知房间内其他玩家 (使用同步发送)
	playerJoinedMsg := protocol.MustNewMessage(protocol.MsgPlayerJoined, protocol.PlayerJoinedData{
		Player: protocol.PlayerInfo{
			ID:       player.ID,
			Username: player.Username,
			IsReady:  player.IsReady,
			IsAlive:  true,
		},
	})

	for _, p := range room.Players {
		if p.ID != playerID {
			p.SendMessageDirect(playerJoinedMsg)
		}
	}

	return nil
}

// handleReady 处理准备
func (h *MessageHandler) handleReady(playerID string, msg *protocol.Message) error {
	player := h.server.GetPlayer(playerID)
	if player == nil {
		return errors.New("player not found")
	}

	if player.RoomID == "" {
		return errors.New("player not in room")
	}

	room := h.server.GetRoom(player.RoomID)
	if room == nil {
		return errors.New("room not found")
	}

	// 切换准备状态
	newReadyState := !player.IsReady
	if err := room.SetPlayerReady(playerID, newReadyState); err != nil {
		return err
	}

	// 通知房间内所有玩家
	readyMsg := protocol.MustNewMessage(protocol.MsgPlayerReady, protocol.PlayerReadyData{
		PlayerID: playerID,
		IsReady:  newReadyState,
	})

	room.BroadcastMessage(readyMsg)

	// 如果所有人都准备好了，尝试开始游戏
	// 由于可能有多个goroutine同时到达这里，Start()内部会检查状态
	if room.CanStart() {
		if err := room.Start(); err != nil {
			// 忽略 "room is not in waiting state" 错误，这表示游戏已经被其他goroutine启动了
			if err.Error() != "room is not in waiting state" {
				h.logger.Error("failed to start game", "error", err)
				return err
			}
		}
	}

	return nil
}

// handlePerformAction 处理游戏动作
func (h *MessageHandler) handlePerformAction(playerID string, msg *protocol.Message) error {
	var data map[string]interface{}
	if err := msg.UnmarshalData(&data); err != nil {
		return err
	}

	player := h.server.GetPlayer(playerID)
	if player == nil {
		return errors.New("player not found")
	}

	room := h.server.GetRoom(player.RoomID)
	if room == nil {
		return errors.New("room not found")
	}

	if room.Engine == nil {
		return errors.New("game not started")
	}

	// 安全类型断言 - 解析动作类型
	actionTypeRaw, ok := data["actionType"]
	if !ok {
		return errors.New("missing actionType field")
	}
	actionTypeStr, ok := actionTypeRaw.(string)
	if !ok {
		return errors.New("actionType must be a string")
	}
	actionType := werewolf.ActionType(actionTypeStr)

	targetID := ""
	if tid, ok := data["targetID"].(string); ok {
		targetID = tid
	}

	actionData := make(map[string]interface{})
	if ad, ok := data["data"].(map[string]interface{}); ok {
		actionData = ad
	}

	// 执行动作
	err := room.Engine.PerformAction(playerID, actionType, targetID, actionData)

	// 发送动作结果 (使用同步发送)
	var resultMsg *protocol.Message
	if err != nil {
		resultMsg = protocol.MustNewMessage(protocol.MsgActionResult, protocol.ActionResultData{
			Success: false,
			Message: err.Error(),
		})
	} else {
		resultMsg = protocol.MustNewMessage(protocol.MsgActionResult, protocol.ActionResultData{
			Success: true,
			Message: "动作执行成功",
			Data:    actionData,
		})
	}

	player.SendMessageDirect(resultMsg)

	// 更新游戏状态
	room.SendGameState()

	return err
}
