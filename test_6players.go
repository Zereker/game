package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/Zereker/game/protocol"
	"github.com/Zereker/socket"
)

// TestClient represents a test player client
type TestClient struct {
	Name     string
	PlayerID string
	RoomID   string
	Conn     *socket.Conn
	Messages chan *protocol.Message
	Role     string
	Camp     string
}

func NewTestClient(name string) *TestClient {
	return &TestClient{
		Name:     name,
		Messages: make(chan *protocol.Message, 50),
	}
}

func (c *TestClient) Connect() error {
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

func (c *TestClient) Login() error {
	msg, _ := protocol.NewLoginMessage(c.Name)
	return c.Conn.Write(msg)
}

func (c *TestClient) CreateRoom() error {
	msg, _ := protocol.NewCreateRoomMessage("TestRoom", []interface{}{
		"werewolf", "werewolf", "villager", "villager", "seer", "witch",
	})
	return c.Conn.Write(msg)
}

func (c *TestClient) JoinRoom(roomID string) error {
	msg, _ := protocol.NewJoinRoomMessage(roomID)
	return c.Conn.Write(msg)
}

func (c *TestClient) Ready() error {
	msg, _ := protocol.NewReadyMessage()
	return c.Conn.Write(msg)
}

func (c *TestClient) PerformAction(actionType, targetID string) error {
	msg, _ := protocol.NewPerformActionMessage(actionType, targetID, nil)
	return c.Conn.Write(msg)
}

func (c *TestClient) WaitFor(msgType protocol.MessageType, timeout time.Duration) (*protocol.Message, error) {
	for {
		select {
		case msg := <-c.Messages:
			if msg.Type == msgType {
				return msg, nil
			}
			// Handle other important messages
			c.handleMessage(msg)
		case <-time.After(timeout):
			return nil, fmt.Errorf("timeout waiting for %s", msgType)
		}
	}
}

func (c *TestClient) handleMessage(msg *protocol.Message) {
	switch msg.Type {
	case protocol.MsgLoginSuccess:
		var data protocol.LoginSuccessData
		msg.UnmarshalData(&data)
		c.PlayerID = data.PlayerID
		fmt.Printf("[%s] Logged in with ID: %s\n", c.Name, c.PlayerID[:8])

	case protocol.MsgRoomCreated:
		var data protocol.RoomCreatedData
		msg.UnmarshalData(&data)
		c.RoomID = data.RoomID
		fmt.Printf("[%s] Room created: %s\n", c.Name, c.RoomID)

	case protocol.MsgRoomJoined:
		var data protocol.RoomJoinedData
		msg.UnmarshalData(&data)
		c.RoomID = data.RoomID
		fmt.Printf("[%s] Joined room: %s (players: %d)\n", c.Name, c.RoomID, len(data.Players))

	case protocol.MsgPlayerJoined:
		var data protocol.PlayerJoinedData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] Player joined: %s\n", c.Name, data.Player.Username)

	case protocol.MsgPlayerReady:
		var data protocol.PlayerReadyData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] Player ready: %s = %v\n", c.Name, data.PlayerID[:8], data.IsReady)

	case protocol.MsgGameStarted:
		var data protocol.GameStartedData
		msg.UnmarshalData(&data)
		c.Role = string(data.RoleType)
		c.Camp = string(data.Camp)
		fmt.Printf("[%s] Game started! Role: %s, Camp: %s\n", c.Name, c.Role, c.Camp)

	case protocol.MsgPhaseChanged:
		var data protocol.PhaseChangedData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] Phase: %s (Round %d)\n", c.Name, data.Phase, data.Round)

	case protocol.MsgGameState:
		var data protocol.GameStateData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] State: Phase=%s, Round=%d, Alive=%d\n",
			c.Name, data.Phase, data.Round, len(data.AlivePlayers))

	case protocol.MsgGameEvent:
		var data protocol.GameEventData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] Event: %s\n", c.Name, data.Message)

	case protocol.MsgActionResult:
		var data protocol.ActionResultData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] Action result: %v - %s\n", c.Name, data.Success, data.Message)

	case protocol.MsgGameEnded:
		var data protocol.GameEndedData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] Game ended! Winner: %s\n", c.Name, data.Winner)

	case protocol.MsgError:
		var data protocol.ErrorData
		msg.UnmarshalData(&data)
		fmt.Printf("[%s] Error: %s\n", c.Name, data.Message)

	default:
		raw, _ := json.Marshal(msg)
		fmt.Printf("[%s] Unknown: %s\n", c.Name, string(raw))
	}
}

func (c *TestClient) DrainMessages(duration time.Duration) {
	timeout := time.After(duration)
	for {
		select {
		case msg := <-c.Messages:
			c.handleMessage(msg)
		case <-timeout:
			return
		}
	}
}

func main() {
	fmt.Println("=== 6-Player Werewolf Game Test ===")
	fmt.Println()

	// Create 6 clients
	clients := make([]*TestClient, 6)
	names := []string{"Alice", "Bob", "Carol", "Dave", "Eve", "Frank"}
	for i := 0; i < 6; i++ {
		clients[i] = NewTestClient(names[i])
	}

	// Connect all clients
	fmt.Println("Step 1: Connecting all clients...")
	for _, c := range clients {
		if err := c.Connect(); err != nil {
			fmt.Printf("Failed to connect %s: %v\n", c.Name, err)
			return
		}
	}
	time.Sleep(500 * time.Millisecond)

	// Login all clients
	fmt.Println("Step 2: Logging in all clients...")
	for _, c := range clients {
		if err := c.Login(); err != nil {
			fmt.Printf("Failed to login %s: %v\n", c.Name, err)
			return
		}
	}
	time.Sleep(1 * time.Second)

	// Process login responses
	for _, c := range clients {
		c.DrainMessages(500 * time.Millisecond)
	}

	// First client creates room
	fmt.Println("\nStep 3: Alice creates room...")
	if err := clients[0].CreateRoom(); err != nil {
		fmt.Printf("Failed to create room: %v\n", err)
		return
	}
	time.Sleep(1 * time.Second)
	clients[0].DrainMessages(500 * time.Millisecond)

	roomID := clients[0].RoomID
	if roomID == "" {
		fmt.Println("Failed: Room ID not received")
		return
	}
	fmt.Printf("Room ID: %s\n", roomID)

	// Other clients join room
	fmt.Println("\nStep 4: Other players join room...")
	for i := 1; i < 6; i++ {
		if err := clients[i].JoinRoom(roomID); err != nil {
			fmt.Printf("Failed to join room: %v\n", err)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(1 * time.Second)

	// Drain messages
	for _, c := range clients {
		c.DrainMessages(500 * time.Millisecond)
	}

	// All clients ready - send sequentially to avoid race conditions
	fmt.Println("\nStep 5: All players ready...")
	for _, c := range clients {
		if err := c.Ready(); err != nil {
			fmt.Printf("Failed to ready %s: %v\n", c.Name, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(3 * time.Second)

	// Process game start messages
	fmt.Println("\nStep 6: Processing game start...")
	time.Sleep(3 * time.Second)
	for _, c := range clients {
		c.DrainMessages(3 * time.Second)
	}

	// Show roles
	fmt.Println("\n=== Role Assignment ===")
	werewolves := []*TestClient{}
	villagers := []*TestClient{}
	seer := (*TestClient)(nil)
	witch := (*TestClient)(nil)

	for _, c := range clients {
		fmt.Printf("%s: %s (%s)\n", c.Name, c.Role, c.Camp)
		switch c.Role {
		case "werewolf":
			werewolves = append(werewolves, c)
		case "villager":
			villagers = append(villagers, c)
		case "seer":
			seer = c
		case "witch":
			witch = c
		}
	}

	// Test night phase actions if game started
	if len(werewolves) > 0 && seer != nil {
		fmt.Println("\n=== Night Phase Actions ===")

		// Werewolf kills a villager
		if len(villagers) > 0 {
			target := villagers[0]
			fmt.Printf("Werewolf %s tries to kill %s...\n", werewolves[0].Name, target.Name)
			werewolves[0].PerformAction("werewolf_kill", target.PlayerID)
			time.Sleep(500 * time.Millisecond)
			werewolves[0].DrainMessages(1 * time.Second)
		}

		// Seer checks a player
		if len(werewolves) > 0 {
			target := werewolves[0]
			fmt.Printf("Seer %s checks %s...\n", seer.Name, target.Name)
			seer.PerformAction("seer_check", target.PlayerID)
			time.Sleep(500 * time.Millisecond)
			seer.DrainMessages(1 * time.Second)
		}

		// Witch uses potion
		if witch != nil && len(villagers) > 0 {
			fmt.Printf("Witch %s considers using save potion...\n", witch.Name)
			witch.PerformAction("witch_save", villagers[0].PlayerID)
			time.Sleep(500 * time.Millisecond)
			witch.DrainMessages(1 * time.Second)
		}
	}

	// Final message drain
	fmt.Println("\n=== Final Status ===")
	for _, c := range clients {
		c.DrainMessages(2 * time.Second)
	}

	fmt.Println("\n=== Test Complete ===")
}
