package protocol

import (
	"encoding/json"
	"time"

	"github.com/Zereker/socket"
	"github.com/pkg/errors"
)

// Message 游戏消息
type Message struct {
	Type      MessageType     `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// NewMessage 创建新消息
func NewMessage(msgType MessageType, data interface{}) (*Message, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(err, "marshal message data")
	}

	return &Message{
		Type:      msgType,
		Data:      dataBytes,
		Timestamp: time.Now().Unix(),
	}, nil
}

// UnmarshalData 解析消息数据
func (m *Message) UnmarshalData(v interface{}) error {
	if err := json.Unmarshal(m.Data, v); err != nil {
		return errors.Wrap(err, "unmarshal message data")
	}
	return nil
}

// Length 实现 socket.Message 接口
func (m *Message) Length() int {
	data, _ := json.Marshal(m)
	return len(data)
}

// Body 实现 socket.Message 接口
func (m *Message) Body() []byte {
	data, _ := json.Marshal(m)
	return data
}

// Codec 消息编解码器
type Codec struct{}

// NewCodec 创建新的编解码器
func NewCodec() *Codec {
	return &Codec{}
}

// Decode 实现 socket.Codec 接口
func (c *Codec) Decode(data []byte) (socket.Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, errors.Wrap(err, "decode message")
	}
	return &msg, nil
}

// Encode 实现 socket.Codec 接口
func (c *Codec) Encode(message socket.Message) ([]byte, error) {
	return message.Body(), nil
}

// 辅助函数：创建各种类型的消息

// NewLoginMessage 创建登录消息
func NewLoginMessage(username string) (*Message, error) {
	return NewMessage(MsgLogin, LoginData{Username: username})
}

// NewCreateRoomMessage 创建房间消息
func NewCreateRoomMessage(roomName string, roles []interface{}) (*Message, error) {
	// roles 从 werewolf.RoleType 转换而来
	return NewMessage(MsgCreateRoom, map[string]interface{}{
		"roomName": roomName,
		"roles":    roles,
	})
}

// NewJoinRoomMessage 加入房间消息
func NewJoinRoomMessage(roomID string) (*Message, error) {
	return NewMessage(MsgJoinRoom, JoinRoomData{RoomID: roomID})
}

// NewReadyMessage 准备消息
func NewReadyMessage() (*Message, error) {
	return NewMessage(MsgReady, map[string]interface{}{})
}

// NewPerformActionMessage 执行动作消息
func NewPerformActionMessage(actionType, targetID string, data map[string]interface{}) (*Message, error) {
	return NewMessage(MsgPerformAction, map[string]interface{}{
		"actionType": actionType,
		"targetID":   targetID,
		"data":       data,
	})
}

// NewErrorMessage 错误消息
func NewErrorMessage(message string) (*Message, error) {
	return NewMessage(MsgError, ErrorData{Message: message})
}
