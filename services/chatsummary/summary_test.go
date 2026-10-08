package chatsummary

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
)

type readerFunc func(context.Context, Query) (Source, error)

func (f readerFunc) Read(ctx context.Context, q Query) (Source, error) { return f(ctx, q) }

func TestNormalizeRetainsConfiguredCountDefault(t *testing.T) {
	// Normal startup supplies 50 from config defaults. Zero config historically
	// clamps to 10 via ai.ResolveCount, and remains unchanged for test embedders.
	for _, test := range []struct{ configured, want int }{{50, 50}, {0, 10}, {999, 100}} {
		query, err := Normalize(Query{}, test.configured)
		if err != nil || query.Count != test.want {
			t.Fatalf("configured=%d count=%d err=%v", test.configured, query.Count, err)
		}
	}
}

// These tests exercise the service boundary with synthetic history, without DB or network.
func TestReadDistinguishesEmptyFailureAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name   string
		source Source
		err    error
		want   error
	}{
		{"empty", Source{}, nil, ErrNoRecords},
		{"blank_text", Source{Records: []Record{{MessageID: 1, Text: " \n\t "}}}, nil, ErrNoRecords},
		{"unavailable", Source{}, ErrUnavailable, ErrUnavailable},
		{"canceled", Source{}, context.Canceled, context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Read(context.Background(), readerFunc(func(context.Context, Query) (Source, error) { return test.source, test.err }), Query{})
			if !errors.Is(err, test.want) || len(got.Records) != 0 {
				t.Fatalf("material=%+v error=%v", got, err)
			}
		})
	}
	_, err := Read(context.Background(), NewGroupReader(nil, 100, 3), Query{Period: "recent"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing platform connection: %v", err)
	}
}

func TestReadBoundsMaterialAndPreservesScope(t *testing.T) {
	records := make([]Record, 100)
	for i := range records {
		records[i] = Record{MessageID: int32(i + 1), UserID: 9, Name: "Alice", Text: strings.Repeat("讨论内容", 600)}
	}
	material, err := Read(context.Background(), readerFunc(func(_ context.Context, q Query) (Source, error) {
		if q.Mode != "actions" || q.Period != "yesterday" {
			t.Fatalf("wrong query: %+v", q)
		}
		return Source{Scope: "昨日本地前100条取样，可能不涵盖整个时间段", Records: records}, nil
	}), Query{Count: 100, Period: "yesterday", Mode: "actions", Focus: "发布"})
	if err != nil {
		t.Fatal(err)
	}
	raw := material.JSON()
	if !json.Valid([]byte(raw)) || len([]rune(raw)) > 10000 || !material.Truncated || len(material.Records) == 0 {
		t.Fatalf("bad bounded material: chars=%d records=%d truncated=%v", len([]rune(raw)), len(material.Records), material.Truncated)
	}
	if material.Scope != "昨日本地前100条取样，可能不涵盖整个时间段" || material.Focus != "发布" || material.Records[0].MessageID != 1 {
		t.Fatal("lost task or source information")
	}
}

func TestReadRetainsEscapedFirstRecordWithinJSONBudget(t *testing.T) {
	for _, content := range []string{strings.Repeat("<", 1500), strings.Repeat("\x00", 1500)} {
		material, err := Read(context.Background(), readerFunc(func(context.Context, Query) (Source, error) {
			return Source{Scope: "本群记录取样", Records: []Record{{MessageID: 42, Text: content}}}, nil
		}), Query{Focus: strings.Repeat("<", 200)})
		if err != nil || len(material.Records) != 1 {
			t.Fatalf("first escaped record disappeared: records=%d err=%v", len(material.Records), err)
		}
		if strings.TrimSuffix(material.Records[0].Text, "…") == "" || material.Records[0].MessageID != 42 {
			t.Fatal("budgeted record lost source text or identity")
		}
		if !material.Truncated || !json.Valid([]byte(material.JSON())) || len([]rune(material.JSON())) > 10000 {
			t.Fatalf("invalid budget: truncated=%v chars=%d", material.Truncated, len([]rune(material.JSON())))
		}
	}
}

func TestReadRejectsUnsupportedQueryBeforeFetching(t *testing.T) {
	reader := readerFunc(func(context.Context, Query) (Source, error) {
		t.Fatal("invalid query fetched data")
		return Source{}, nil
	})
	for _, q := range []Query{{Count: 1000}, {Count: -1}, {Period: "30days"}, {Mode: "create_tasks"}, {Focus: strings.Repeat("长", 201)}} {
		if _, err := Read(context.Background(), reader, q); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("query=%+v err=%v", q, err)
		}
	}
}

func TestReadHonorsCountAndMarksOmittedRecords(t *testing.T) {
	records := make([]Record, 11)
	for i := range records {
		records[i] = Record{MessageID: int32(i + 1), Text: "record"}
	}
	material, err := Read(context.Background(), readerFunc(func(context.Context, Query) (Source, error) { return Source{Scope: "前10条", Records: records}, nil }), Query{Count: 10})
	if err != nil || len(material.Records) != 10 || !material.Truncated || material.Records[9].MessageID != 10 {
		t.Fatalf("material=%+v err=%v", material, err)
	}
}

func TestHistoryExcludesCurrentRequestAndNonText(t *testing.T) {
	var messages []bot.HistoryMessage
	if err := json.Unmarshal([]byte(`[
 {"message_id":1,"user_id":2,"sender":{"nickname":"A"},"message":[{"type":"text","data":{"text":"周五发布"}}]},
 {"message_id":2,"user_id":3,"message":[{"type":"image","data":{}}]},
 {"message_id":3,"user_id":4,"message":[{"type":"text","data":{"text":"总结群聊"}}]}
]`), &messages); err != nil {
		t.Fatal(err)
	}
	records := historyRecords(messages, 3)
	if len(records) != 1 || records[0].Text != "周五发布" || records[0].Name != "A" {
		t.Fatalf("records=%+v", records)
	}
}
