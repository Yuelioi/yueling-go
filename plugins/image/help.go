package image

import (
	"strings"

	"github.com/Yuelioi/yueling-go/config"
)

// HelpCallUsage 列出 single/grid/external 的调用命令（每类一组）。
func HelpCallUsage() string {
	var single, grid, external []string
	for _, e := range activeEntries {
		switch kindOf(e) {
		case config.KindSingle:
			single = append(single, e.Call...)
		case config.KindGrid:
			if len(e.Call) > 0 {
				grid = append(grid, e.Call[0]) // grid 取首命令做代表
			}
		case config.KindExternal:
			external = append(external, e.Call...)
		}
	}
	var lines []string
	if len(single) > 0 {
		lines = append(lines, "  "+strings.Join(single, " / "))
	}
	if len(external) > 0 {
		lines = append(lines, "  "+strings.Join(external, " / "))
	}
	if len(grid) > 0 {
		lines = append(lines, "  "+strings.Join(grid, " / ")+"（4合1，发 2×2 网格）")
	}
	return strings.Join(lines, "\n")
}

// HelpAddUsage 列出所有添加命令。
func HelpAddUsage() string {
	var adds []string
	for _, e := range activeEntries {
		if e.Add != "" {
			adds = append(adds, e.Add)
		}
	}
	return "  " + strings.Join(adds, " / ") + "  + 图片"
}

// HelpCallCommands 返回所有调用命令（供 help 注册表的 Commands 列表用）。
func HelpCallCommands() []string {
	var cmds []string
	for _, e := range activeEntries {
		cmds = append(cmds, e.Call...)
	}
	return cmds
}

// HelpAddCommands 返回所有添加命令。
func HelpAddCommands() []string {
	var cmds []string
	for _, e := range activeEntries {
		if e.Add != "" {
			cmds = append(cmds, e.Add)
		}
	}
	return cmds
}
