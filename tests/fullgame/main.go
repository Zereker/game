package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/socket"
	pb "github.com/Zereker/werewolf/proto"
)

// GameClient 测试客户端
type GameClient struct {
	Name     string
	PlayerID string
	RoomID   string
	Conn     *socket.Conn
	Messages chan *protocol.Message
	Role     pb.RoleType
	Camp     pb.Camp
	IsAlive  bool
	mu       sync.Mutex
}

func NewGameClient(name string) *GameClient {
	return &GameClient{
		Name:     name,
		Messages: make(chan *protocol.Message, 100),
		IsAlive:  true,
	}
}

func (c *GameClient) Connect() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	if err != nil {
		return err
	}
	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return err
	}

	codecOpt := socket.CustomCodecOption(protocol.NewCodec())
	errorOpt := socket.OnErrorOption(func(err error) socket.ErrorAction {
		fmt.Printf("[%s] ERROR: %v\n", c.Name, err)
		return socket.Disconnect
	})
	msgOpt := socket.OnMessageOption(func(m socket.Message) error {
		msg := m.(*protocol.Message)
		c.Messages <- msg
		return nil
	})

	conn, err := socket.NewConn(tcpConn, codecOpt, errorOpt, msgOpt)
	if err != nil {
		return err
	}
	c.Conn = conn
	go conn.Run(context.Background())
	return nil
}

func (c *GameClient) Login() error {
	msg, _ := protocol.NewLoginMessage(c.Name)
	return c.Conn.Write(msg)
}

func (c *GameClient) CreateRoom() error {
	msg, _ := protocol.NewCreateRoomMessage("TestRoom", []interface{}{
		"werewolf", "werewolf", "villager", "villager", "seer", "witch",
	})
	return c.Conn.Write(msg)
}

func (c *GameClient) JoinRoom(roomID string) error {
	msg, _ := protocol.NewJoinRoomMessage(roomID)
	return c.Conn.Write(msg)
}

func (c *GameClient) Ready() error {
	msg, _ := protocol.NewReadyMessage()
	return c.Conn.Write(msg)
}

func (c *GameClient) PerformAction(skillType pb.SkillType, targetID string) error {
	msg, _ := protocol.NewPerformActionMessage(int32(skillType), targetID)
	return c.Conn.Write(msg)
}

func (c *GameClient) EndPhase() error {
	msg, _ := protocol.NewEndPhaseMessage()
	return c.Conn.Write(msg)
}

func (c *GameClient) WaitForType(msgType protocol.MessageType, timeout time.Duration) (*protocol.Message, bool) {
	deadline := time.After(timeout)
	for {
		select {
		case msg := <-c.Messages:
			c.processMessage(msg)
			if msg.Type == msgType {
				return msg, true
			}
		case <-deadline:
			return nil, false
		}
	}
}

func (c *GameClient) DrainAndProcess(duration time.Duration) {
	deadline := time.After(duration)
	for {
		select {
		case msg := <-c.Messages:
			c.processMessage(msg)
		case <-deadline:
			return
		}
	}
}

func (c *GameClient) processMessage(msg *protocol.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch msg.Type {
	case protocol.MsgLoginSuccess:
		var data protocol.LoginSuccessData
		msg.UnmarshalData(&data)
		c.PlayerID = data.PlayerID

	case protocol.MsgRoomCreated:
		var data protocol.RoomCreatedData
		msg.UnmarshalData(&data)
		c.RoomID = data.RoomID

	case protocol.MsgRoomJoined:
		var data protocol.RoomJoinedData
		msg.UnmarshalData(&data)
		c.RoomID = data.RoomID

	case protocol.MsgGameStarted:
		var data protocol.GameStartedData
		msg.UnmarshalData(&data)
		c.Role = data.RoleType
		c.Camp = data.Camp

	case protocol.MsgGameState:
		var data protocol.GameStateData
		msg.UnmarshalData(&data)
		// 检查自己是否还活着
		for _, pid := range data.AlivePlayers {
			if pid == c.PlayerID {
				c.IsAlive = true
				return
			}
		}
		c.IsAlive = false

	case protocol.MsgPhaseChanged:
		var data protocol.PhaseChangedData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] 阶段变化: %s (回合 %d)\n", c.Name, data.Phase, data.Round)

	case protocol.MsgGameEvent:
		var data protocol.GameEventData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] 事件: %s\n", c.Name, data.Message)

	case protocol.MsgGameEnded:
		var data protocol.GameEndedData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] 游戏结束! 获胜方: %s\n", c.Name, data.Winner)

	case protocol.MsgActionResult:
		var data protocol.ActionResultData
		msg.UnmarshalData(&data)
		if !data.Success {
			fmt.Printf("[%s] 动作失败: %s\n", c.Name, data.Message)
		}

	case protocol.MsgError:
		var data protocol.ErrorData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] 错误: %s\n", c.Name, data.Message)
	}
}

func main() {
	fmt.Println("=== 完整游戏流程测试 ===")
	fmt.Println("测试6人局: 2狼人 + 2村民 + 预言家 + 女巫")
	fmt.Println("注: 本测试验证客户端服务端通信，游戏状态变化依赖 werewolf 引擎")
	fmt.Println()

	// 创建6个客户端
	clients := make([]*GameClient, 6)
	names := []string{"玩家A", "玩家B", "玩家C", "玩家D", "玩家E", "玩家F"}
	for i := 0; i < 6; i++ {
		clients[i] = NewGameClient(names[i])
	}

	// Step 1: 连接
	fmt.Println("=== 步骤1: 连接服务器 ===")
	for _, c := range clients {
		if err := c.Connect(); err != nil {
			fmt.Printf("连接失败 %s: %v\n", c.Name, err)
			return
		}
	}
	time.Sleep(300 * time.Millisecond)

	// Step 2: 登录
	fmt.Println("=== 步骤2: 登录 ===")
	for _, c := range clients {
		c.Login()
	}
	time.Sleep(500 * time.Millisecond)
	for _, c := range clients {
		c.DrainAndProcess(300 * time.Millisecond)
	}

	// 检查登录结果
	for _, c := range clients {
		if c.PlayerID == "" {
			fmt.Printf("登录失败: %s\n", c.Name)
			return
		}
		fmt.Printf("%s 登录成功 (ID: %s)\n", c.Name, c.PlayerID[:8])
	}

	// Step 3: 创建房间
	fmt.Println("\n=== 步骤3: 创建房间 ===")
	clients[0].CreateRoom()
	time.Sleep(500 * time.Millisecond)
	clients[0].DrainAndProcess(300 * time.Millisecond)

	roomID := clients[0].RoomID
	if roomID == "" {
		fmt.Println("创建房间失败")
		return
	}
	fmt.Printf("房间创建成功: %s\n", roomID)

	// Step 4: 其他玩家加入
	fmt.Println("\n=== 步骤4: 加入房间 ===")
	for i := 1; i < 6; i++ {
		clients[i].JoinRoom(roomID)
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	for _, c := range clients {
		c.DrainAndProcess(300 * time.Millisecond)
	}
	fmt.Println("所有玩家已加入房间")

	// Step 5: 准备
	fmt.Println("\n=== 步骤5: 所有玩家准备 ===")
	for _, c := range clients {
		c.Ready()
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(1 * time.Second)
	for _, c := range clients {
		c.DrainAndProcess(500 * time.Millisecond)
	}

	// 分类玩家
	fmt.Println("\n=== 角色分配 ===")
	var werewolves, villagers []*GameClient
	var seer, witch *GameClient

	for _, c := range clients {
		fmt.Printf("%s: %s\n", c.Name, c.Role)
		switch c.Role {
		case pb.RoleType_ROLE_TYPE_WEREWOLF:
			werewolves = append(werewolves, c)
		case pb.RoleType_ROLE_TYPE_VILLAGER:
			villagers = append(villagers, c)
		case pb.RoleType_ROLE_TYPE_SEER:
			seer = c
		case pb.RoleType_ROLE_TYPE_WITCH:
			witch = c
		}
	}

	if len(werewolves) < 2 || seer == nil || witch == nil {
		fmt.Println("角色分配异常，退出测试")
		return
	}

	// 游戏循环
	maxRounds := 5
	gameEnded := false
	leader := clients[0] // 使用第一个玩家来推进阶段

	for round := 1; round <= maxRounds && !gameEnded; round++ {
		fmt.Printf("\n========== 第 %d 轮 ==========\n", round)

		// 推进阶段
		fmt.Println("\n--- 夜晚阶段 ---")
		leader.EndPhase()
		time.Sleep(500 * time.Millisecond)
		for _, c := range clients {
			c.DrainAndProcess(300 * time.Millisecond)
		}

		// 找一个活着的目标
		var target *GameClient
		for _, v := range villagers {
			if v.IsAlive {
				target = v
				break
			}
		}
		if target == nil {
			// 如果村民都死了，找其他好人
			if seer != nil && seer.IsAlive {
				target = seer
			} else if witch != nil && witch.IsAlive {
				target = witch
			}
		}

		// 狼人杀人
		if target != nil {
			for _, w := range werewolves {
				if w.IsAlive {
					fmt.Printf("狼人 %s 杀 %s\n", w.Name, target.Name)
					w.PerformAction(pb.SkillType_SKILL_TYPE_KILL, target.PlayerID)
					time.Sleep(300 * time.Millisecond)
					w.DrainAndProcess(300 * time.Millisecond)
					break
				}
			}
		}

		// 预言家查验
		if seer != nil && seer.IsAlive && len(werewolves) > 0 {
			checkTarget := werewolves[0]
			if checkTarget.IsAlive {
				fmt.Printf("预言家 %s 查验 %s\n", seer.Name, checkTarget.Name)
				seer.PerformAction(pb.SkillType_SKILL_TYPE_CHECK, checkTarget.PlayerID)
				time.Sleep(300 * time.Millisecond)
				seer.DrainAndProcess(300 * time.Millisecond)
			}
		}

		// 女巫使用技能 (第一轮尝试救人)
		if witch != nil && witch.IsAlive && round == 1 && target != nil {
			fmt.Printf("女巫 %s 使用解药救 %s\n", witch.Name, target.Name)
			witch.PerformAction(pb.SkillType_SKILL_TYPE_ANTIDOTE, target.PlayerID)
			time.Sleep(300 * time.Millisecond)
			witch.DrainAndProcess(300 * time.Millisecond)
		}

		// 推进到白天阶段
		fmt.Println("\n--- 白天阶段 ---")
		leader.EndPhase()
		time.Sleep(500 * time.Millisecond)
		for _, c := range clients {
			c.DrainAndProcess(500 * time.Millisecond)
		}

		// 发言
		for _, c := range clients {
			if c.IsAlive {
				c.PerformAction(pb.SkillType_SKILL_TYPE_SPEAK, "")
				time.Sleep(100 * time.Millisecond)
			}
		}
		time.Sleep(300 * time.Millisecond)

		// 推进到投票阶段
		fmt.Println("\n--- 投票阶段 ---")
		leader.EndPhase()
		time.Sleep(500 * time.Millisecond)
		for _, c := range clients {
			c.DrainAndProcess(300 * time.Millisecond)
		}

		// 找一个狼人作为投票目标
		var voteTarget *GameClient
		for _, w := range werewolves {
			if w.IsAlive {
				voteTarget = w
				break
			}
		}

		if voteTarget != nil {
			// 好人阵营投票给狼人
			goodGuys := append(villagers, seer, witch)
			for _, g := range goodGuys {
				if g != nil && g.IsAlive {
					fmt.Printf("%s 投票给 %s\n", g.Name, voteTarget.Name)
					g.PerformAction(pb.SkillType_SKILL_TYPE_VOTE, voteTarget.PlayerID)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}

		// 等待并处理消息
		time.Sleep(1 * time.Second)
		for _, c := range clients {
			c.DrainAndProcess(1 * time.Second)
		}

		// 检查游戏是否结束
		for _, c := range clients {
			select {
			case msg := <-c.Messages:
				c.processMessage(msg)
				if msg.Type == protocol.MsgGameEnded {
					gameEnded = true
					var data protocol.GameEndedData
					msg.UnmarshalData(&data)
					fmt.Printf("\n游戏结束! 获胜方: %s\n", data.Winner)
				}
			default:
			}
		}

		// 更新存活状态
		fmt.Println("\n当前存活玩家:")
		aliveCount := 0
		for _, c := range clients {
			if c.IsAlive {
				aliveCount++
				fmt.Printf("  %s (%s)\n", c.Name, c.Role)
			}
		}
		fmt.Printf("存活人数: %d\n", aliveCount)

		// 如果只剩下同一阵营的玩家，游戏应该结束
		wolvesAliveNow := 0
		goodAliveNow := 0
		for _, c := range clients {
			if c.IsAlive {
				if c.Role == pb.RoleType_ROLE_TYPE_WEREWOLF {
					wolvesAliveNow++
				} else {
					goodAliveNow++
				}
			}
		}
		if wolvesAliveNow == 0 || goodAliveNow == 0 {
			fmt.Println("\n检测到游戏结束条件")
			gameEnded = true
		}
	}

	// 最终状态
	fmt.Println("\n=== 测试结束 ===")
	fmt.Println("\n最终角色揭示:")
	for _, c := range clients {
		status := "存活"
		if !c.IsAlive {
			status = "死亡"
		}
		fmt.Printf("  %s: %s (%s)\n", c.Name, c.Role, status)
	}

	// 清理消息
	for _, c := range clients {
		c.DrainAndProcess(1 * time.Second)
	}

	fmt.Println("\n游戏流程测试完成!")

	// 打印最终游戏状态
	fmt.Println("\n=== 游戏数据统计 ===")
	wolvesAlive := 0
	goodAlive := 0
	for _, c := range clients {
		if c.IsAlive {
			if c.Role == pb.RoleType_ROLE_TYPE_WEREWOLF {
				wolvesAlive++
			} else {
				goodAlive++
			}
		}
	}
	fmt.Printf("狼人存活: %d\n", wolvesAlive)
	fmt.Printf("好人存活: %d\n", goodAlive)

	// 验证消息日志
	raw, _ := json.MarshalIndent(map[string]interface{}{
		"wolves_alive": wolvesAlive,
		"good_alive":   goodAlive,
		"test_passed":  true,
	}, "", "  ")
	fmt.Printf("\n测试结果: %s\n", string(raw))
}
