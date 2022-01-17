package otelsql

import "time"

func microsecondsSince(t time.Time) float64 {
	return float64(time.Since(t).Nanoseconds()) / 1e6
}
