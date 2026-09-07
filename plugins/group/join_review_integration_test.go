package group

import (
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/internal/testdb"
)

type joinReviewRecorder struct {
	called                bool
	approve               bool
	flag, subtype, reason string
}

func (r *joinReviewRecorder) SetGroupAddRequest(flag, subtype string, approve bool, reason string) error {
	r.called, r.approve, r.flag, r.subtype, r.reason = true, approve, flag, subtype, reason
	return nil
}

func TestJoinReviewRequestUsesLiveGlobalAndGroupConfig(t *testing.T) {
	testdb.Init(t)
	set := func(id int64, mode string, allow, deny []string) {
		t.Helper()
		if err := db.SetJoinReview(id, db.JoinReviewConfig{Mode: mode, Allow: allow, Deny: deny}); err != nil {
			t.Fatal(err)
		}
	}
	request := func(comment string, called, approve bool) {
		t.Helper()
		recorder := &joinReviewRecorder{}
		err := reviewJoinRequest(recorder, &bot.RequestEvent{GroupID: 123, SubType: "add", Flag: "request-flag", Comment: comment})
		if err != nil || recorder.called != called || (called && recorder.approve != approve) {
			t.Fatalf("%q: %+v err=%v", comment, recorder, err)
		}
		if called && (recorder.flag != "request-flag" || recorder.subtype != "add" || (!approve && recorder.reason != joinDenyReason)) {
			t.Fatalf("approval payload: %+v", recorder)
		}
	}
	set(0, db.JoinModeOverride, []string{"交流"}, []string{"广告"})
	request("交流", true, true)
	request("交流广告", true, false)
	set(0, db.JoinModeOverride, []string{"学习"}, nil)
	request("交流", false, false)
	request("学习", true, true)
	set(123, db.JoinModeOverride, []string{"本群"}, nil)
	request("学习", false, false)
	request("本群", true, true)
	set(123, db.JoinModeDisabled, []string{"*"}, nil)
	request("本群", false, false)
	set(123, db.JoinModeInherit, nil, nil)
	request("学习", true, true)
	set(0, db.JoinModeOverride, []string{"*"}, nil)
	request("", false, false)
	request("任意理由", true, true)
	recorder := &joinReviewRecorder{}
	if err := reviewJoinRequest(recorder, &bot.RequestEvent{GroupID: 123, SubType: "invite", Comment: "任意理由"}); err != nil || recorder.called {
		t.Fatalf("invitation processed: %+v %v", recorder, err)
	}
}
