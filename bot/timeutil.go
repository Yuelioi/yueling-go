package bot

import (
	"time"

	"github.com/Yuelioi/yueling-go/util"
)

// Now returns the current time in Asia/Shanghai.
func Now() time.Time { return util.Now() }

// Today returns the current date string (YYYY-MM-DD) in Asia/Shanghai.
func Today() string { return util.Today() }
