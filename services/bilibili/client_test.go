package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	bpi "github.com/Yuelioi/bpi-go"
	"github.com/Yuelioi/bpi-go/message"
	"github.com/Yuelioi/yueling-go/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

const navFixture = `{"code":0,"data":{"wbi_img":{"img_url":"https://i0.hdslb.com/bfs/wbi/abcdefghijklmnopqrstuvwxyz123456.png","sub_url":"https://i0.hdslb.com/bfs/wbi/ABCDEFGHIJKLMNOPQRSTUVWXYZ654321.png"}}}`

func testClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()
	c, err := New(config.BilibiliConfig{Cookie: "DedeUserID=10; SESSDATA=test-session; bili_jct=test-csrf; buvid3=test-device", UID: 999, RecentCount: 2}, bpi.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPrivateTransportContract(t *testing.T) {
	var sent bool
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		if cookie, _ := r.Cookie("SESSDATA"); cookie == nil || cookie.Value != "test-session" {
			t.Fatalf("missing login cookie")
		}
		switch r.URL.Path {
		case "/x/web-interface/nav":
			return jsonResponse(navFixture), nil
		case "/session_svr/v1/session_svr/get_sessions":
			q := r.URL.Query()
			if (q.Get("session_type") != "1" && q.Get("session_type") != "2") || q.Get("begin_ts") != "123000000" || q.Get("sort_rule") != "2" {
				t.Fatalf("session query=%v", q)
			}
			return jsonResponse(`{"code":0,"data":{"session_list":[{"talker_id":20,"session_type":1,"session_ts":123000001,"last_msg":{"msg_seqno":3,"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""},"top_ts":0,"ack_seqno":0,"ack_ts":0,"max_seqno":0,"unread_count":0,"system_msg_type":0,"is_follow":0,"is_dnd":0}],"has_more":0,"is_address_list_empty":0,"anti_disturb_cleaning":false,"show_level":false}}`), nil
		case "/svr_sync/v1/svr_sync/fetch_session_msgs":
			if r.URL.Query().Get("begin_seqno") != "1" || r.URL.Query().Get("talker_id") != "20" {
				t.Fatalf("message query=%v", r.URL.Query())
			}
			return jsonResponse(`{"code":0,"data":{"messages":[{"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"msg_seqno":3,"content":"{\"content\":\"你好\"}","msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""}],"has_more":0,"min_seqno":0,"max_seqno":0}}`), nil
		case "/web_im/v1/web_im/send_msg":
			sent = true
			if r.Method != http.MethodPost {
				t.Fatal("wrong send method")
			}
			q := r.URL.Query()
			if q.Get("w_rid") == "" || q.Get("wts") == "" || q.Get("w_sender_uid") != "10" || q.Get("w_receiver_id") != "20" {
				t.Fatalf("unsigned or wrong sender: %v", q)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{"msg[sender_uid]": "10", "msg[receiver_id]": "20", "msg[receiver_type]": "1", "msg[msg_type]": "1", "csrf": "test-csrf", "csrf_token": "test-csrf", "msg[content]": `{"content":" 你好\nhello "}`} {
				if got := r.PostForm.Get(key); got != want {
					t.Fatalf("%s=%q want %q", key, got, want)
				}
			}
			if r.PostForm.Get("msg[dev_id]") != q.Get("w_dev_id") {
				t.Fatal("device identity differs")
			}
			return jsonResponse(`{"code":0,"data":{"msg_key":123}}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})
	ctx := context.Background()
	sessions, err := c.Sessions(ctx, 123000000)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%v err=%v", sessions, err)
	}
	msgs, err := c.Messages(ctx, 20, 1)
	if err != nil || len(msgs) != 1 || msgs[0].Text() != "你好" {
		t.Fatalf("messages=%v err=%v", msgs, err)
	}
	if err := c.SendText(ctx, 20, " 你好\nhello "); err != nil {
		t.Fatal(err)
	}
	if !sent {
		t.Fatal("send endpoint not reached")
	}
}

func TestMessagesPageInOrderAndDeduplicate(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return jsonResponse(`{"code":0,"data":{"messages":[{"msg_seqno":5,"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""},{"msg_seqno":4,"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""}],"has_more":1,"min_seqno":0,"max_seqno":0}}`), nil
		}
		if r.URL.Query().Get("end_seqno") != "4" {
			t.Fatalf("end cursor=%s", r.URL.Query().Get("end_seqno"))
		}
		return jsonResponse(`{"code":0,"data":{"messages":[{"msg_seqno":4,"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""},{"msg_seqno":3,"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""},{"msg_seqno":2,"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""}],"has_more":0,"min_seqno":0,"max_seqno":0}}`), nil
	})
	msgs, err := c.Messages(context.Background(), 20, 2)
	if err != nil || len(msgs) != 3 || msgs[0].Seq != 3 || msgs[2].Seq != 5 {
		t.Fatalf("page result=%v err=%v", msgs, err)
	}
}

func TestSessionsPaginationAndStalledPage(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Get("session_type") == "2" {
			return jsonResponse(`{"code":0,"data":{"session_list":null,"has_more":0,"is_address_list_empty":0,"anti_disturb_cleaning":false,"show_level":false}}`), nil
		}
		if calls == 1 {
			return jsonResponse(`{"code":0,"data":{"session_list":[{"talker_id":20,"session_type":1,"session_ts":200,"top_ts":0,"ack_seqno":0,"ack_ts":0,"max_seqno":0,"unread_count":0,"system_msg_type":0,"is_follow":0,"is_dnd":0,"last_msg":{"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_seqno":0,"msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""}}],"has_more":1,"is_address_list_empty":0,"anti_disturb_cleaning":false,"show_level":false}}`), nil
		}
		if r.URL.Query().Get("end_ts") != "200" {
			t.Fatalf("session page end=%s", r.URL.Query().Get("end_ts"))
		}
		return jsonResponse(`{"code":0,"data":{"session_list":[{"talker_id":21,"session_type":1,"session_ts":150,"top_ts":0,"ack_seqno":0,"ack_ts":0,"max_seqno":0,"unread_count":0,"system_msg_type":0,"is_follow":0,"is_dnd":0,"last_msg":{"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_seqno":0,"msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""}}],"has_more":0,"is_address_list_empty":0,"anti_disturb_cleaning":false,"show_level":false}}`), nil
	})
	items, err := c.Sessions(context.Background(), 100)
	if err != nil || len(items) != 2 || calls != 3 {
		t.Fatalf("session pages=%v err=%v calls=%d", items, err, calls)
	}
	stalled := testClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"code":0,"data":{"session_list":[{"session_ts":200,"talker_id":20,"session_type":1,"top_ts":0,"ack_seqno":0,"ack_ts":0,"max_seqno":0,"unread_count":0,"system_msg_type":0,"is_follow":0,"is_dnd":0,"last_msg":{"sender_uid":20,"receiver_id":10,"receiver_type":1,"msg_type":1,"content":"{\"content\":\"hello\"}","msg_seqno":0,"msg_key":1,"timestamp":1700000000,"msg_status":0,"msg_source":0,"notify_code":""}}],"has_more":1,"is_address_list_empty":0,"anti_disturb_cleaning":false,"show_level":false}}`), nil
	})
	if _, err := stalled.Sessions(context.Background(), 100); err == nil {
		t.Fatal("stalled page accepted")
	}
}

func TestSendRejectsMissingAuthAndOversizedContentWithoutIO(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, fmt.Errorf("unexpected I/O") })
	c := testClient(t, transport)
	if err := c.SendText(context.Background(), 20, strings.Repeat("字", 1000)); err == nil {
		t.Fatal("oversized send accepted")
	}
	anonymous, err := New(config.BilibiliConfig{}, bpi.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	if err := anonymous.SendText(context.Background(), 20, "hello"); err == nil {
		t.Fatal("anonymous send accepted")
	}
	if calls != 0 {
		t.Fatalf("unexpected network calls=%d", calls)
	}
}

func TestVideoPartAndOnlineFailureKeepsCounters(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/x/web-interface/view":
			return jsonResponse(`{"code":0,"data":{"bvid":"BV1xx411c7mD","title":"示例","stat":{"view":123,"like":4},"pages":[{"cid":101,"page":1},{"cid":102,"page":2}]}}`), nil
		case "/x/player/online/total":
			if r.URL.Query().Get("cid") != "102" {
				t.Fatalf("wrong part cid=%s", r.URL.Query().Get("cid"))
			}
			return jsonResponse(`{"code":-400,"message":"invalid"}`), nil
		default:
			t.Fatal(r.URL.Path)
			return nil, nil
		}
	})
	stats, err := c.Video(context.Background(), "https://www.bilibili.com/video/BV1xx411c7mD", 2)
	if err != nil || stats.View.Stat.View != 123 || stats.OnlineError == nil {
		t.Fatalf("stats=%v err=%v", stats, err)
	}
	if _, err = c.Video(context.Background(), "BV1xx411c7mD", 3); err == nil {
		t.Fatal("invalid part accepted")
	}
}

func TestRecentUsesUploadOrderAndConfiguredUID(t *testing.T) {
	views := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/x/web-interface/nav":
			return jsonResponse(navFixture), nil
		case "/x/space/wbi/arc/search":
			if r.URL.Query().Get("mid") != "999" || r.URL.Query().Get("order") != "pubdate" || r.URL.Query().Get("w_rid") == "" {
				t.Fatalf("upload query=%v", r.URL.Query())
			}
			return jsonResponse(`{"code":0,"data":{"list":{"vlist":[{"bvid":"BV1xx411c7mD","title":"new"},{"bvid":"BV1xx411c7mE","title":"old"}]}}}`), nil
		case "/x/web-interface/view":
			views++
			return jsonResponse(fmt.Sprintf(`{"code":0,"data":{"bvid":%q,"title":"video","cid":123,"stat":{"view":4}}}`, r.URL.Query().Get("bvid"))), nil
		case "/x/player/online/total":
			return jsonResponse(`{"code":0,"data":{"total":"12","count":"4","show_switch":{"total":true,"count":true}}}`), nil
		default:
			t.Fatal(r.URL.Path)
			return nil, nil
		}
	})
	items, err := c.Recent(context.Background(), 0, 1)
	if err != nil || len(items) != 1 || items[0].Title != "new" || views != 1 {
		t.Fatalf("recent=%v err=%v views=%d", items, err, views)
	}
	if c.AccountID() != 10 {
		t.Fatal("upload UID leaked into sender identity")
	}
}

func TestMalformedAndUnsupportedPrivateContent(t *testing.T) {
	if got := (PrivateMessage{Type: 2, Content: `{"url":"https://i0.hdslb.com/test.png"}`}).Text(); !strings.HasPrefix(got, "[图片]") {
		t.Fatal(got)
	}
	if got := (PrivateMessage{Type: 7, Content: `{"title":"分享"}`}).Text(); !strings.Contains(got, "分享") {
		t.Fatal(got)
	}
	var msg PrivateMessage
	if err := json.Unmarshal([]byte(`{"content":"broken"}`), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Text() != "[无法解析的私信]" {
		t.Fatal(msg.Text())
	}
}

func sdkResponse(t *testing.T, data any) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]any{"code": 0, "data": data})
	if err != nil {
		t.Fatal(err)
	}
	return jsonResponse(string(body))
}

func TestClassifiedSessionsMergeLatestAndResetCursor(t *testing.T) {
	calls := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if q.Get("begin_ts") != "100" {
			t.Fatalf("lost incremental cursor: %v", q)
		}
		switch q.Get("session_type") {
		case "1":
			if q.Get("end_ts") == "" {
				return sdkResponse(t, message.Sessions{SessionList: []message.Session{{TalkerID: 20, SessionType: 1, SessionTS: 300, LastMessage: &message.PrivateMessage{MsgSeqno: 8}}}, HasMore: 1}), nil
			}
			if q.Get("end_ts") != "300" {
				t.Fatalf("wrong end cursor: %v", q)
			}
			return sdkResponse(t, message.Sessions{SessionList: []message.Session{{TalkerID: 21, SessionType: 1, SessionTS: 200}}}), nil
		case "2":
			if q.Get("end_ts") != "" {
				t.Fatal("cursor leaked across classifications")
			}
			return sdkResponse(t, message.Sessions{SessionList: []message.Session{{TalkerID: 20, SessionType: 1, SessionTS: 350, LastMessage: &message.PrivateMessage{MsgSeqno: 9}}, {TalkerID: 22, SessionType: 1, SessionTS: 325}}}), nil
		default:
			t.Fatal("All classification cannot paginate reliably")
			return nil, nil
		}
	})
	items, err := c.Sessions(context.Background(), 100)
	if err != nil || len(items) != 3 || calls != 3 {
		t.Fatalf("items=%v error=%v calls=%d", items, err, calls)
	}
	if items[0].TalkerID != 20 || items[0].LastMessage.Seq != 9 || items[1].TalkerID != 22 {
		t.Fatalf("newest session not retained: %v", items)
	}
}

func TestClassificationFailureReturnsNoPartialCheckpointInput(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("session_type") == "1" {
			return sdkResponse(t, message.Sessions{SessionList: []message.Session{{TalkerID: 20, SessionType: 1, SessionTS: 300}}}), nil
		}
		return jsonResponse(`{"code":-101,"message":"requires login"}`), nil
	})
	items, err := c.Sessions(context.Background(), 0)
	if err == nil || items != nil {
		t.Fatalf("partial result accepted: %v %v", items, err)
	}
}

func TestSDKMessageRejectsUnsignedOverflow(t *testing.T) {
	for _, item := range []message.PrivateMessage{{SenderUID: math.MaxUint64}, {ReceiverID: math.MaxUint64}, {MsgSeqno: math.MaxUint64}, {Timestamp: math.MaxUint64}} {
		if _, err := fromSDKMessage(item); err == nil {
			t.Fatal("unsigned value wrapped into checkpoint")
		}
	}
}

func TestUncertainSendIsNotRetried(t *testing.T) {
	for _, key := range []string{"0", "null"} {
		t.Run(key, func(t *testing.T) {
			posts := 0
			c := testClient(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/x/web-interface/nav" {
					return jsonResponse(navFixture), nil
				}
				posts++
				return jsonResponse(`{"code":0,"data":{"msg_key":` + key + `}}`), nil
			})
			if err := c.SendText(context.Background(), 20, "hello"); err == nil || posts != 1 {
				t.Fatalf("err=%v posts=%d", err, posts)
			}
		})
	}
}

func TestSDKPollForwardsOnceAfterEmptyBaselineAndRestart(t *testing.T) {
	fresh := false
	fetches, notifications := 0, 0
	incoming := message.PrivateMessage{SenderUID: 20, ReceiverID: 10, ReceiverType: 1, MsgType: 1, MsgSeqno: 1, MsgKey: 123, Timestamp: 1001, Content: `{"content":"新私信"}`}
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/x/web-interface/card":
			return jsonResponse(`{"code":0,"data":{"card":{"mid":"20","name":"测试联系人","fans":0,"attention":0},"following":false,"archive_count":0,"article_count":0,"follower":0,"like_num":0}}`), nil
		case "/session_svr/v1/session_svr/get_sessions":
			session := message.Session{TalkerID: 20, SessionType: 1, SessionTS: 1001000000}
			if fresh {
				session.LastMessage = &incoming
			}
			return sdkResponse(t, message.Sessions{SessionList: []message.Session{session}}), nil
		case "/svr_sync/v1/svr_sync/fetch_session_msgs":
			fetches++
			return sdkResponse(t, message.SessionMessages{Messages: []message.PrivateMessage{incoming}, MinSeqno: 1, MaxSeqno: 1}), nil
		default:
			t.Fatal("unexpected request; default off must not send")
			return nil, nil
		}
	})
	cfg := config.BilibiliConfig{ReplyMode: "off", ReplyCooldownSeconds: 60, ReplyPerMinute: 5}
	store := &memoryStore{}
	m := NewManager(c, store, cfg, []int64{99}, nil)
	m.now = func() time.Time { return time.Unix(1000, 0) }
	bootstrap(t, m)
	if fetches != 0 {
		t.Fatal("baseline fetched historical messages")
	}
	fresh = true
	m.now = func() time.Time { return time.Unix(1002, 0) }
	notify := func(qq int64, text string) error {
		notifications++
		if qq != 99 || !strings.Contains(text, "新私信") || !strings.Contains(text, "测试联系人") || !strings.Contains(text, "B站回复 1") || !strings.Contains(text, "B站回复 <内容>") {
			t.Fatalf("wrong QQ notification: %d %s", qq, text)
		}
		return nil
	}
	if count, err := m.Poll(context.Background(), notify); err != nil || count != 1 {
		t.Fatalf("new message count=%d err=%v", count, err)
	}
	restarted := NewManager(c, store, cfg, []int64{99}, nil)
	restarted.now = m.now
	if count, err := restarted.Poll(context.Background(), notify); err != nil || count != 0 {
		t.Fatalf("restart count=%d err=%v", count, err)
	}
	if notifications != 1 || fetches != 1 {
		t.Fatalf("notifications=%d fetches=%d", notifications, fetches)
	}
}
