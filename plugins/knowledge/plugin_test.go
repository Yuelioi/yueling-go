package knowledge

import (
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/db"
)

func TestSplitKnowledgeInput(t *testing.T) {
	title, content := splitKnowledgeInput("入群规则 | 新成员请修改群名片")
	if title != "入群规则" || content != "新成员请修改群名片" {
		t.Fatalf("title=%q content=%q", title, content)
	}
	title, content = splitKnowledgeInput("只有正文")
	if title != "" || content != "只有正文" {
		t.Fatalf("title=%q content=%q", title, content)
	}
}

func TestFormatKnowledgeListCapsRows(t *testing.T) {
	rows := make([]db.GroupKnowledge, 21)
	for index := range rows {
		rows[index] = db.GroupKnowledge{ID: uint(index + 1), GroupID: 100, Title: "条目"}
	}
	got := formatKnowledgeList(rows)
	if !strings.Contains(got, "本群可用知识（21 条，含共享）") || !strings.Contains(got, "ID 1 · 本群") || !strings.Contains(got, "另有 1 条") {
		t.Fatalf("formatted=%q", got)
	}
}

func TestFormatKnowledgeListMarksSharedEntries(t *testing.T) {
	got := formatKnowledgeList([]db.GroupKnowledge{{ID: 1, GroupID: db.SharedKnowledgeGroupID, Title: "公共说明"}})
	if !strings.Contains(got, "ID 1 · 共享 · 公共说明") {
		t.Fatalf("formatted=%q", got)
	}
}
