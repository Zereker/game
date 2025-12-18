package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/Zereker/game/protocol"
	"github.com/pkg/errors"
)

// InputHandler 输入处理器
type InputHandler struct {
	scanner *bufio.Scanner
	client  *Client
}

// NewInputHandler 创建输入处理器
func NewInputHandler(client *Client) *InputHandler {
	return &InputHandler{
		scanner: bufio.NewScanner(os.Stdin),
		client:  client,
	}
}

// ReadCommand 读取命令
func (h *InputHandler) ReadCommand() (string, error) {
	if !h.scanner.Scan() {
		return "", errors.New("failed to read input")
	}

	return strings.TrimSpace(h.scanner.Text()), nil
}

// HandleCommand 处理命令
func (h *InputHandler) HandleCommand(cmd string) error {
	if cmd == "" {
		return nil
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "help":
		return h.handleHelp()
	case "login":
		return h.handleLogin(parts)
	case "create":
		return h.handleCreate(parts)
	case "join":
		return h.handleJoin(parts)
	case "ready":
		return h.handleReady()
	case "kill":
		return h.handleAction("kill", parts)
	case "check":
		return h.handleAction("check", parts)
	case "protect":
		return h.handleAction("protect", parts)
	case "antidote":
		return h.handleAction("antidote", parts)
	case "poison":
		return h.handleAction("poison", parts)
	case "vote":
		return h.handleAction("vote", parts)
	case "speak":
		return h.handleSpeak(parts)
	case "quit", "exit":
		return h.handleQuit()
	default:
		return errors.Errorf("未知命令: %s，输入 help 查看帮助", command)
	}
}

// handleHelp 处理帮助命令
func (h *InputHandler) handleHelp() error {
	h.client.ui.PrintHelp()
	h.scanner.Scan() // 等待用户按回车
	h.client.Render()
	return nil
}

// handleLogin 处理登录命令
func (h *InputHandler) handleLogin(parts []string) error {
	if len(parts) < 2 {
		return errors.New("用法: login <用户名>")
	}

	username := parts[1]
	msg, err := protocol.NewLoginMessage(username)
	if err != nil {
		return err
	}

	return h.client.SendMessage(msg)
}

// handleCreate 处理创建房间命令
func (h *InputHandler) handleCreate(parts []string) error {
	roomName := "游戏房间"
	if len(parts) >= 2 {
		roomName = parts[1]
	}

	// 使用默认6人局配置
	msg, err := protocol.NewCreateRoomMessage(roomName, []interface{}{
		"werewolf", "werewolf",
		"villager", "villager",
		"seer", "witch",
	})
	if err != nil {
		return err
	}

	return h.client.SendMessage(msg)
}

// handleJoin 处理加入房间命令
func (h *InputHandler) handleJoin(parts []string) error {
	if len(parts) < 2 {
		return errors.New("用法: join <房间ID>")
	}

	roomID := parts[1]
	msg, err := protocol.NewJoinRoomMessage(roomID)
	if err != nil {
		return err
	}

	return h.client.SendMessage(msg)
}

// handleReady 处理准备命令
func (h *InputHandler) handleReady() error {
	msg, err := protocol.NewReadyMessage()
	if err != nil {
		return err
	}

	return h.client.SendMessage(msg)
}

// handleAction 处理游戏动作命令
func (h *InputHandler) handleAction(actionType string, parts []string) error {
	targetID := ""

	// 某些动作需要目标
	needsTarget := actionType != "antidote"

	if needsTarget {
		if len(parts) < 2 {
			return errors.Errorf("用法: %s <玩家编号>", actionType)
		}

		// 解析玩家编号
		playerNum, err := strconv.Atoi(parts[1])
		if err != nil {
			return errors.New("玩家编号必须是数字")
		}

		// 将编号转换为玩家ID
		players := h.client.state.Players
		if playerNum < 1 || playerNum > len(players) {
			return errors.Errorf("无效的玩家编号: %d", playerNum)
		}

		targetID = players[playerNum-1].ID
	}

	msg, err := protocol.NewPerformActionMessage(actionType, targetID, nil)
	if err != nil {
		return err
	}

	return h.client.SendMessage(msg)
}

// handleSpeak 处理发言命令
func (h *InputHandler) handleSpeak(parts []string) error {
	if len(parts) < 2 {
		return errors.New("用法: speak <内容>")
	}

	content := strings.Join(parts[1:], " ")

	data := map[string]interface{}{
		"content": content,
	}

	msg, err := protocol.NewPerformActionMessage("speak", "", data)
	if err != nil {
		return err
	}

	return h.client.SendMessage(msg)
}

// handleQuit 处理退出命令
func (h *InputHandler) handleQuit() error {
	h.client.ui.PrintMessage("再见！")
	os.Exit(0)
	return nil
}
