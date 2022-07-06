package otelsql

import "time"

func millisecondsSince(t time.Time) float64 {
	return float64(time.Since(t).Milliseconds())
}
