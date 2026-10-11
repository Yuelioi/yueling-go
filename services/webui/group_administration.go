package webui

import (
	"github.com/Yuelioi/yueling-go/db"
	"github.com/gin-gonic/gin"
	"net/http"
	"sort"
)

func (s *Server) handleGroupAdministration(c *gin.Context) {
	rows, err := s.groups.ListAdministration()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "群配置概览读取失败")
		return
	}
	type entry struct {
		db.GroupAdministration
		GroupName string `json:"group_name"`
		Connected bool   `json:"connected"`
	}
	entries := map[int64]entry{}
	for _, row := range rows {
		entries[row.GroupID] = entry{GroupAdministration: row}
	}
	warning := ""
	lister := s.resolveGroupLister()
	if lister == nil {
		warning = "Bot 未连接；显示已保存配置的群，群名与未配置群暂不可用。"
	} else {
		groups, err := lister.GetGroupList()
		if err != nil {
			warning = "群列表读取失败；显示已保存配置的群。"
		} else {
			for _, group := range groups {
				row, exists := entries[group.GroupID]
				if !exists {
					row.GroupAdministration = db.GroupAdministration{GroupID: group.GroupID, DisabledPlugins: []int{}, JoinMode: db.JoinModeInherit}
				}
				row.GroupName, row.Connected = group.GroupName, true
				entries[group.GroupID] = row
			}
		}
	}
	result := make([]entry, 0, len(entries))
	for _, row := range entries {
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GroupID < result[j].GroupID })
	c.JSON(http.StatusOK, gin.H{"ok": true, "groups": result, "warning": warning})
}
