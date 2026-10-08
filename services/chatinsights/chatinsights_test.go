package chatinsights

import (
	"testing"
	"time"
)

func TestResolvePeriodUsesCallerTimezoneAndRollingWindows(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 14, 0, 30, 0, 0, location)

	today, ok := ResolvePeriod("today", now)
	if !ok || today.Start.Format(time.RFC3339) != "2026-08-14T00:00:00+08:00" {
		t.Fatalf("today=%+v ok=%v", today, ok)
	}
	yesterday, ok := ResolvePeriod("yesterday", now)
	if !ok || yesterday.Start.Format(time.RFC3339) != "2026-08-13T00:00:00+08:00" || yesterday.End != today.Start {
		t.Fatalf("yesterday=%+v ok=%v", yesterday, ok)
	}
	sevenDays, ok := ResolvePeriod("7days", now)
	if !ok || sevenDays.Start != now.AddDate(0, 0, -7) || sevenDays.End.Sub(now) != time.Second {
		t.Fatalf("seven days=%+v ok=%v", sevenDays, ok)
	}
	thirtyDays, ok := ResolvePeriod("30days", now)
	if !ok || thirtyDays.Start != now.AddDate(0, 0, -30) || thirtyDays.End.Sub(now) != time.Second {
		t.Fatalf("thirty days=%+v ok=%v", thirtyDays, ok)
	}
	if _, ok := ResolvePeriod("year", now); ok {
		t.Fatal("unsupported period accepted")
	}
}
