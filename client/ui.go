package main

import (
	"fmt"
	"strings"

	"github.com/Zereker/game/protocol"
	pb "github.com/Zereker/werewolf/proto"
)

// ANSI 颜色代码
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// UI 终端用户界面
type UI struct {
	width int // 终端宽度
}

// NewUI 创建新的 UI
func NewUI() *UI {
	return &UI{
		width: 80,
	}
}

// Clear 清屏
func (ui *UI) Clear() {
	fmt.Print("\033[2J\033[H")
}

// PrintHeader 打印标题
func (ui *UI) PrintHeader(roomID string, round int, phase pb.PhaseType) {
	ui.printSeparator()
	title := "狼人杀游戏"
	padding := (ui.width - len(title)) / 2
	fmt.Printf("%s%s%s%s\n", ColorBold, strings.Repeat(" ", padding), title, ColorReset)

	if roomID != "" {
		info := fmt.Sprintf("房间: %s | 回合: %d | 阶段: %s", roomID, round, ui.phaseName(phase))
		fmt.Printf("%s%s%s\n", ColorCyan, info, ColorReset)
	}

	ui.printSeparator()
	fmt.Println()
}

// PrintPlayers 打印玩家列表
func (ui *UI) PrintPlayers(players []protocol.PlayerInfo, myID string) {
	fmt.Printf("%s玩家列表:%s\n", ColorBold, ColorReset)

	for i, player := range players {
		status := ui.formatPlayerStatus(player)
		marker := "  "
		if player.ID == myID {
			marker = ColorYellow + "➤ " + ColorReset
		}

		fmt.Printf("%s%d. %-20s %s\n", marker, i+1, player.Username, status)
	}

	fmt.Println()
}

// PrintEvents 打印事件日志
func (ui *UI) PrintEvents(events []string) {
	if len(events) == 0 {
		return
	}

	fmt.Printf("%s事件日志:%s\n", ColorBold, ColorReset)

	// 只显示最近10条事件
	start := 0
	if len(events) > 10 {
		start = len(events) - 10
	}

	for _, event := range events[start:] {
		fmt.Printf("  %s\n", event)
	}

	fmt.Println()
}

// PrintRoleInfo 打印角色信息
func (ui *UI) PrintRoleInfo(roleType pb.RoleType, camp pb.Camp) {
	fmt.Printf("%s你的角色:%s ", ColorBold, ColorReset)

	roleName := ui.roleName(roleType)
	campName := ui.campName(camp)

	campColor := ColorGreen
	if camp == pb.Camp_CAMP_EVIL {
		campColor = ColorRed
	}

	fmt.Printf("%s%s%s (%s%s%s)\n", ColorCyan, roleName, ColorReset, campColor, campName, ColorReset)

	// 显示角色技能
	skills := ui.roleSkills(roleType)
	if skills != "" {
		fmt.Printf("%s可用技能:%s %s\n", ColorBold, ColorReset, skills)
	}

	fmt.Println()
}

// PrintPrompt 打印输入提示
func (ui *UI) PrintPrompt(phase pb.PhaseType, roleType pb.RoleType) {
	fmt.Printf("%s请输入命令:%s\n", ColorBold, ColorReset)

	// 根据阶段和角色提示可用操作
	hints := ui.getActionHints(phase, roleType)
	if hints != "" {
		fmt.Printf("%s提示: %s%s\n", ColorYellow, hints, ColorReset)
	}

	fmt.Print(ColorGreen + "> " + ColorReset)
}

// PrintMessage 打印普通消息
func (ui *UI) PrintMessage(msg string) {
	fmt.Printf("%s%s%s\n", ColorBlue, msg, ColorReset)
}

// PrintError 打印错误消息
func (ui *UI) PrintError(msg string) {
	fmt.Printf("%s错误: %s%s\n", ColorRed, msg, ColorReset)
}

// PrintSuccess 打印成功消息
func (ui *UI) PrintSuccess(msg string) {
	fmt.Printf("%s成功: %s%s\n", ColorGreen, msg, ColorReset)
}

// PrintHelp 打印帮助信息
func (ui *UI) PrintHelp() {
	ui.Clear()
	ui.printSeparator()
	fmt.Printf("%s狼人杀游戏 - 帮助信息%s\n", ColorBold, ColorReset)
	ui.printSeparator()
	fmt.Println()

	commands := []struct {
		cmd  string
		desc string
	}{
		{"login <用户名>", "登录游戏"},
		{"create <房间名>", "创建房间（默认6人局）"},
		{"join <房间ID>", "加入房间"},
		{"ready", "准备/取消准备"},
		{"", ""},
		{"kill <玩家编号>", "狼人击杀目标"},
		{"check <玩家编号>", "预言家查验目标"},
		{"protect <玩家编号>", "守卫保护目标"},
		{"antidote", "女巫使用解药"},
		{"poison <玩家编号>", "女巫使用毒药"},
		{"vote <玩家编号>", "投票"},
		{"speak <内容>", "发言"},
		{"", ""},
		{"help", "显示此帮助信息"},
		{"quit", "退出游戏"},
	}

	for _, cmd := range commands {
		if cmd.cmd == "" {
			fmt.Println()
			continue
		}
		fmt.Printf("  %s%-25s%s %s\n", ColorCyan, cmd.cmd, ColorReset, cmd.desc)
	}

	fmt.Println()
	ui.printSeparator()
	fmt.Printf("\n按回车键继续...")
}

// 辅助函数

func (ui *UI) printSeparator() {
	fmt.Println(strings.Repeat("=", ui.width))
}

func (ui *UI) formatPlayerStatus(player protocol.PlayerInfo) string {
	status := ""

	if player.IsAlive {
		status += ColorGreen + "[存活]" + ColorReset
	} else {
		status += ColorRed + "[死亡]" + ColorReset
	}

	if player.IsReady {
		status += " " + ColorYellow + "[准备]" + ColorReset
	}

	return status
}

func (ui *UI) phaseName(phase pb.PhaseType) string {
	switch phase {
	case pb.PhaseType_PHASE_TYPE_START:
		return "开始"
	case pb.PhaseType_PHASE_TYPE_NIGHT, pb.PhaseType_PHASE_TYPE_NIGHT_GUARD,
		pb.PhaseType_PHASE_TYPE_NIGHT_WOLF, pb.PhaseType_PHASE_TYPE_NIGHT_WITCH,
		pb.PhaseType_PHASE_TYPE_NIGHT_SEER:
		return "夜晚"
	case pb.PhaseType_PHASE_TYPE_DAY:
		return "白天"
	case pb.PhaseType_PHASE_TYPE_VOTE:
		return "投票"
	case pb.PhaseType_PHASE_TYPE_END:
		return "结束"
	default:
		return phase.String()
	}
}

func (ui *UI) roleName(roleType pb.RoleType) string {
	switch roleType {
	case pb.RoleType_ROLE_TYPE_WEREWOLF:
		return "狼人"
	case pb.RoleType_ROLE_TYPE_SEER:
		return "预言家"
	case pb.RoleType_ROLE_TYPE_WITCH:
		return "女巫"
	case pb.RoleType_ROLE_TYPE_GUARD:
		return "守卫"
	case pb.RoleType_ROLE_TYPE_HUNTER:
		return "猎人"
	case pb.RoleType_ROLE_TYPE_VILLAGER:
		return "平民"
	default:
		return roleType.String()
	}
}

func (ui *UI) campName(camp pb.Camp) string {
	switch camp {
	case pb.Camp_CAMP_GOOD:
		return "好人阵营"
	case pb.Camp_CAMP_EVIL:
		return "狼人阵营"
	default:
		return "无阵营"
	}
}

func (ui *UI) roleSkills(roleType pb.RoleType) string {
	switch roleType {
	case pb.RoleType_ROLE_TYPE_WEREWOLF:
		return "kill <编号> - 击杀玩家"
	case pb.RoleType_ROLE_TYPE_SEER:
		return "check <编号> - 查验玩家身份"
	case pb.RoleType_ROLE_TYPE_WITCH:
		return "antidote - 解救被杀玩家 | poison <编号> - 毒杀玩家"
	case pb.RoleType_ROLE_TYPE_GUARD:
		return "protect <编号> - 保护玩家"
	case pb.RoleType_ROLE_TYPE_HUNTER:
		return "被动技能：死亡时可开枪"
	case pb.RoleType_ROLE_TYPE_VILLAGER:
		return "vote <编号> - 投票（白天/投票阶段）"
	default:
		return ""
	}
}

func (ui *UI) getActionHints(phase pb.PhaseType, roleType pb.RoleType) string {
	if phase.IsNightPhase() {
		switch roleType {
		case pb.RoleType_ROLE_TYPE_WEREWOLF:
			return "轮到你行动了，使用 kill <编号> 选择击杀目标"
		case pb.RoleType_ROLE_TYPE_SEER:
			return "使用 check <编号> 查验一名玩家"
		case pb.RoleType_ROLE_TYPE_WITCH:
			return "使用 antidote 解救被杀玩家，或 poison <编号> 毒杀玩家"
		case pb.RoleType_ROLE_TYPE_GUARD:
			return "使用 protect <编号> 保护一名玩家"
		default:
			return "等待其他玩家行动..."
		}
	}

	switch phase {
	case pb.PhaseType_PHASE_TYPE_DAY:
		return "白天讨论阶段，使用 speak <内容> 发言"
	case pb.PhaseType_PHASE_TYPE_VOTE:
		return "投票阶段，使用 vote <编号> 投票"
	default:
		return "输入 help 查看可用命令"
	}
}

func (ui *UI) skillName(skill pb.SkillType) string {
	switch skill {
	case pb.SkillType_SKILL_TYPE_KILL:
		return "击杀"
	case pb.SkillType_SKILL_TYPE_CHECK:
		return "查验"
	case pb.SkillType_SKILL_TYPE_ANTIDOTE:
		return "解药"
	case pb.SkillType_SKILL_TYPE_POISON:
		return "毒药"
	case pb.SkillType_SKILL_TYPE_PROTECT:
		return "守护"
	case pb.SkillType_SKILL_TYPE_VOTE:
		return "投票"
	case pb.SkillType_SKILL_TYPE_SPEAK:
		return "发言"
	case pb.SkillType_SKILL_TYPE_SHOOT:
		return "开枪"
	default:
		return skill.String()
	}
}
