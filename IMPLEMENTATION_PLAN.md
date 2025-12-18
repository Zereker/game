# 狼人杀终端游戏实现计划

## 项目概述

基于现有的 werewolf 游戏引擎和 socket 网络框架，实现一个支持终端游玩的多人狼人杀游戏。

## 技术栈

- **游戏引擎**: github.com/Zereker/werewolf (纯逻辑库)
- **网络框架**: github.com/Zereker/socket (TCP 连接管理)
- **协议格式**: JSON 消息
- **客户端**: 终端 TUI (Text-based User Interface)

## 项目结构

```
/Users/zereker/Documents/Go/src/github.com/Zereker/game/
├── go.mod                    # Go 模块定义
├── protocol/                 # 共享协议包
│   ├── message.go           # 消息定义和编解码
│   └── types.go             # 共享类型定义
├── server/                   # 后端服务器
│   ├── main.go              # 服务器入口
│   ├── server.go            # 服务器核心逻辑
│   ├── room.go              # 游戏房间管理
│   ├── player.go            # 玩家连接管理
│   └── handler.go           # 消息处理器
└── client/                   # 终端客户端
    ├── main.go              # 客户端入口
    ├── client.go            # 客户端核心
    ├── ui.go                # 终端 UI 渲染
    └── input.go             # 用户输入处理
```

## 核心设计

### 1. 消息协议设计 (protocol/)

#### 消息基础结构

```go
type Message struct {
    Type      string          `json:"type"`       // 消息类型
    Data      json.RawMessage `json:"data"`       // 消息数据
    Timestamp int64           `json:"timestamp"`  // 时间戳
}
```

#### 消息类型列表

**客户端 → 服务器**:
- `LOGIN` - 玩家登录 {username: string}
- `CREATE_ROOM` - 创建房间 {roomName: string, config: GameConfig}
- `JOIN_ROOM` - 加入房间 {roomID: string}
- `READY` - 准备开始
- `PERFORM_ACTION` - 执行游戏动作 {actionType: string, targetID: string, data: map}

**服务器 → 客户端**:
- `LOGIN_SUCCESS` - 登录成功 {playerID: string}
- `ROOM_CREATED` - 房间创建成功 {roomID: string}
- `ROOM_JOINED` - 加入房间成功 {roomID: string, players: []Player}
- `GAME_STARTED` - 游戏开始 {roleType: string, players: []Player}
- `PHASE_CHANGED` - 阶段变化 {phase: string, round: int}
- `GAME_STATE` - 游戏状态同步 {state: GameState}
- `GAME_EVENT` - 游戏事件 {event: Event}
- `ACTION_RESULT` - 动作结果 {success: bool, message: string}
- `GAME_ENDED` - 游戏结束 {winner: string, players: []Player}
- `ERROR` - 错误消息 {message: string}

#### Codec 实现

实现 socket.Message 和 socket.Codec 接口，支持 JSON 序列化。

### 2. 服务器架构 (server/)

#### Room 结构

```go
type Room struct {
    ID          string
    Name        string
    Players     map[string]*Player  // playerID -> Player
    Engine      *werewolf.Engine    // 游戏引擎实例
    State       RoomState           // WAITING, PLAYING, FINISHED
    mu          sync.RWMutex
}
```

**房间状态**:
- `WAITING` - 等待玩家加入
- `PLAYING` - 游戏进行中
- `FINISHED` - 游戏结束

**房间管理**:
- 创建房间
- 玩家加入/退出
- 游戏开始/结束
- 事件广播给房间内所有玩家

#### Player 结构

```go
type Player struct {
    ID       string
    Username string
    Conn     *socket.Conn        // 网络连接
    RoomID   string              // 所在房间ID
    IsReady  bool                // 是否准备
}
```

#### Server 结构

```go
type Server struct {
    Rooms      map[string]*Room     // roomID -> Room
    Players    map[string]*Player   // playerID -> Player
    mu         sync.RWMutex
}
```

**核心功能**:
- 管理所有房间和玩家
- 处理客户端消息
- 将 werewolf 引擎事件转换为网络消息

#### 游戏引擎集成

**事件订阅**:
- 订阅 werewolf 引擎的所有事件
- 将事件转换为协议消息
- 根据事件类型决定广播范围（全体/单个玩家）

**事件映射**:
```go
EventGameStarted    → GAME_STARTED (广播给所有玩家，不同玩家看到不同的角色信息)
EventPhaseStarted   → PHASE_CHANGED (广播)
EventPlayerDied     → GAME_EVENT (广播)
EventActionExecuted → ACTION_RESULT (发给执行者)
EventGameEnded      → GAME_ENDED (广播)
```

### 3. 客户端架构 (client/)

#### Client 结构

```go
type Client struct {
    PlayerID    string
    Username    string
    Conn        *socket.Conn
    State       *ClientState
    UI          *UI
    mu          sync.RWMutex
}

type ClientState struct {
    RoomID      string
    MyRole      werewolf.RoleType
    GamePhase   werewolf.PhaseType
    Round       int
    Players     []PlayerInfo
    Events      []string            // 事件日志
}
```

#### UI 设计

**显示区域**:
1. **标题栏** - 显示房间ID、回合数、当前阶段
2. **玩家列表** - 显示所有玩家的状态（存活/死亡、编号）
3. **事件日志** - 显示游戏事件（死亡、投票结果等）
4. **角色信息** - 显示自己的角色和可用技能
5. **输入提示** - 当前可执行的操作提示
6. **命令输入** - 用户输入区

**示例界面**:
```
========== 狼人杀游戏 ==========
房间: ROOM_001 | 回合: 2 | 阶段: 夜晚

玩家列表:
  1. Alice   [存活]
  2. Bob     [存活]
  3. Charlie [死亡]
  4. David   [存活]
  5. Eve     [存活]

事件日志:
  [夜晚] 第2回合开始
  [夜晚] 请狼人选择击杀目标...

你的角色: 狼人 (邪恶阵营)
可用技能: 击杀 (kill)

请输入命令:
> kill 2

================================
```

#### 输入处理

**命令格式**:
- `login <username>` - 登录
- `create <roomName>` - 创建房间
- `join <roomID>` - 加入房间
- `ready` - 准备
- `kill <playerNumber>` - 狼人击杀
- `check <playerNumber>` - 预言家查验
- `protect <playerNumber>` - 守卫保护
- `poison <playerNumber>` - 女巫毒杀
- `antidote` - 女巫解药
- `vote <playerNumber>` - 投票
- `speak <message>` - 发言
- `help` - 帮助信息
- `quit` - 退出

### 4. 游戏流程设计

#### 连接流程

1. 客户端连接服务器
2. 发送 LOGIN 消息（用户名）
3. 服务器分配 playerID，返回 LOGIN_SUCCESS
4. 客户端创建或加入房间
5. 等待其他玩家加入并准备
6. 所有人准备后，游戏开始

#### 游戏阶段流程

**夜晚阶段**:
1. 服务器发送 PHASE_CHANGED (Night)
2. 客户端根据角色显示可用操作
3. 狼人选择击杀目标 → 发送 PERFORM_ACTION
4. 预言家选择查验目标 → 接收查验结果
5. 女巫选择使用解药/毒药
6. 守卫选择保护目标
7. 服务器等待所有夜晚动作完成
8. 进入白天阶段

**白天阶段**:
1. 服务器发送 PHASE_CHANGED (Day)
2. 广播夜晚死亡信息
3. 玩家可以发言讨论
4. 进入投票阶段

**投票阶段**:
1. 服务器发送 PHASE_CHANGED (Vote)
2. 每个存活玩家投票
3. 服务器统计票数
4. 广播投票结果和被投死的玩家
5. 检查胜利条件
6. 如果游戏未结束，回到夜晚阶段

#### 消息同步机制

- 服务器在每个阶段开始时发送完整的 GAME_STATE
- 客户端在接收到 GAME_STATE 后刷新 UI
- 关键事件（死亡、投票结果）通过 GAME_EVENT 单独通知

## 实现步骤

### 阶段1: 基础设施
1. 创建 go.mod，引入依赖
2. 实现 protocol 包（消息定义和编解码）
3. 编写单元测试验证协议序列化

### 阶段2: 服务器核心
1. 实现 Player 和 Room 结构
2. 实现 Server 结构和连接管理
3. 实现消息路由和处理器
4. 集成 werewolf 引擎，订阅事件

### 阶段3: 客户端核心
1. 实现 Client 结构和连接管理
2. 实现消息接收和状态更新
3. 实现基本的 UI 渲染
4. 实现命令解析和输入处理

### 阶段4: 游戏流程
1. 实现房间创建和加入逻辑
2. 实现游戏开始和角色分配
3. 实现夜晚阶段的动作处理
4. 实现白天和投票阶段
5. 实现胜利条件判断

### 阶段5: 测试和优化
1. 端到端测试（多客户端）
2. 错误处理和边界情况
3. UI 优化和用户体验改进
4. 添加日志和调试信息

## 实现决策

基于用户需求，以下是确定的实现方案：

### 游戏配置
- **默认角色**: 6人标准局（2狼人 + 2平民 + 1预言家 + 1女巫）
- **快速测试**: 适合快速开发和测试
- **后续扩展**: 可配置支持9人/12人局

### UI 风格
- **ANSI 彩色终端**: 使用转义序列实现颜色高亮
  - 红色: 死亡、狼人阵营
  - 绿色: 存活、好人阵营
  - 黄色: 警告、提示
  - 蓝色: 系统消息
  - 青色: 角色信息
- **清屏刷新**: 使用 `\033[2J\033[H` 清屏

### 实现顺序
1. **阶段1**: protocol 包（消息协议）
2. **阶段2**: server 包（服务器实现）
3. **阶段3**: client 包（客户端实现）
4. **阶段4**: 端到端测试和优化

## 技术细节

### 并发安全

- Room 和 Server 使用 sync.RWMutex 保护共享状态
- 每个连接在独立 goroutine 中处理
- 使用 channel 进行 goroutine 间通信

### 错误处理

- 网络错误 → 断开连接，清理玩家状态
- 非法操作 → 返回 ERROR 消息给客户端
- 游戏逻辑错误 → 记录日志，返回错误提示

### 配置选项

```go
type GameConfig struct {
    Roles []werewolf.RoleType  // 角色配置
    EnableLastWords bool        // 是否启用遗言
}
```

### 日志和调试

- 服务器和客户端都使用结构化日志
- 记录关键事件：连接、断开、游戏开始、阶段变化、玩家动作
- 支持调试模式输出详细信息

## 用户体验优化

1. **清屏和刷新**: 每次状态更新时清屏重新渲染
2. **颜色高亮**: 使用 ANSI 颜色区分不同信息
3. **操作提示**: 根据当前阶段和角色显示可用命令
4. **错误提示**: 友好的错误信息和帮助提示
5. **自动完成**: 支持 Tab 自动完成命令

## 扩展性考虑

1. **持久化**: 未来可添加游戏记录保存
2. **观战模式**: 支持观众加入房间观看
3. **更多角色**: 扩展支持更多狼人杀角色
4. **Web UI**: 可以基于相同的服务器实现 Web 客户端
5. **AI 玩家**: 可以添加 AI 玩家填充空位

## 预期代码量

- protocol/: ~200 行
- server/: ~600 行
- client/: ~500 行
- 总计: ~1300 行代码

## 依赖管理

```
module github.com/Zereker/game

go 1.21

require (
    github.com/Zereker/werewolf v0.0.0
    github.com/Zereker/socket v0.0.0
    github.com/pkg/errors v0.9.1
)
```

使用 replace 指令指向本地路径。
