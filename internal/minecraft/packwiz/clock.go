package packwiz

import "time"

var nowMs = func() int64 { return time.Now().UnixMilli() }
