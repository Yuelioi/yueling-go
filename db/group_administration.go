package db

import "sort"

type GroupAdministration struct {
	GroupID         int64  `json:"group_id"`
	DisabledPlugins []int  `json:"disabled_plugins"`
	CustomStyle     bool   `json:"custom_style"`
	StylePrompt     string `json:"style_prompt"`
	JoinMode        string `json:"join_mode"`
	FeedCount       int64  `json:"feed_count"`
	FailingFeeds    int64  `json:"failing_feeds"`
	KnowledgeCount  int64  `json:"knowledge_count"`
	DigestTime      string `json:"digest_time"`
}

func (s *GroupStore) ListAdministration() ([]GroupAdministration, error) {
	rows := map[int64]*GroupAdministration{}
	get := func(id int64) *GroupAdministration {
		if rows[id] == nil {
			mode := JoinModeInherit
			if id == 0 {
				mode = JoinModeOverride
			}
			rows[id] = &GroupAdministration{GroupID: id, DisabledPlugins: []int{}, JoinMode: mode}
		}
		return rows[id]
	}
	get(0)
	var plugins []GroupPluginDisabled
	if err := s.database.Find(&plugins).Error; err != nil {
		return nil, err
	}
	for _, row := range plugins {
		get(row.GroupID).DisabledPlugins = append(get(row.GroupID).DisabledPlugins, row.PluginID)
	}
	var styles []GroupAISetting
	if err := s.database.Find(&styles).Error; err != nil {
		return nil, err
	}
	for _, row := range styles {
		get(row.GroupID).CustomStyle = true
		get(row.GroupID).StylePrompt = row.StylePrompt
	}
	var rules []GroupJoinRule
	if err := s.database.Select("DISTINCT group_id").Find(&rules).Error; err != nil {
		return nil, err
	}
	for _, row := range rules {
		get(row.GroupID).JoinMode = JoinModeOverride
	}
	var settings []GroupJoinSetting
	if err := s.database.Find(&settings).Error; err != nil {
		return nil, err
	}
	for _, row := range settings {
		get(row.GroupID).JoinMode = row.Mode
	}
	var digests []DailyDigest
	if err := s.database.Where("enabled = ?", true).Find(&digests).Error; err != nil {
		return nil, err
	}
	for _, row := range digests {
		get(row.GroupID).DigestTime = row.SendTime
	}
	var counts []struct {
		GroupID int64
		Total   int64
		Failed  int64
	}
	if err := s.database.Model(&FeedSubscription{}).Select("group_id, count(*) AS total, count(*) FILTER (WHERE enabled AND consecutive_failures > 0) AS failed").Group("group_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, row := range counts {
		get(row.GroupID).FeedCount = row.Total
		get(row.GroupID).FailingFeeds = row.Failed
	}
	counts = nil
	if err := s.database.Model(&GroupKnowledge{}).Select("group_id, count(*) AS total").Group("group_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, row := range counts {
		get(row.GroupID).KnowledgeCount = row.Total
	}
	result := make([]GroupAdministration, 0, len(rows))
	for _, row := range rows {
		sort.Ints(row.DisabledPlugins)
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GroupID < result[j].GroupID })
	return result, nil
}
