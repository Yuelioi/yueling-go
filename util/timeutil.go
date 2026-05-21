package util

import "time"

var shanghaiLoc = func() *time.Location {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return loc
}()

// Now returns the current time in Asia/Shanghai.
func Now() time.Time {
	return time.Now().In(shanghaiLoc)
}

// Today returns the current date string (YYYY-MM-DD) in Asia/Shanghai.
func Today() string {
	return Now().Format("2006-01-02")
}
