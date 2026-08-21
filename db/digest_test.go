package db

import "testing"

func TestDailyDigestUpsertAndDelete(t *testing.T) {
	initPostgresForTest(t)

	if _, err := UpsertDailyDigest(100, 1, "21:30", "30 21 * * *", 80); err != nil {
		t.Fatal(err)
	}
	row, err := UpsertDailyDigest(100, 2, "22:00", "0 22 * * *", 60)
	if err != nil {
		t.Fatal(err)
	}
	if row.CreatedBy != 2 || row.SendTime != "22:00" || row.MessageCount != 60 {
		t.Fatalf("digest = %+v", row)
	}
	rows, err := GetActiveDailyDigests()
	if err != nil || len(rows) != 1 {
		t.Fatalf("active digests = %+v, err=%v", rows, err)
	}
	if err := DeleteDailyDigest(100); err != nil {
		t.Fatal(err)
	}
	if rows, _ := GetActiveDailyDigests(); len(rows) != 0 {
		t.Fatalf("active after delete = %+v", rows)
	}
}
