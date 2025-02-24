package timeutil

import "time"

const (
	DateTimeFormat = "2006-01-02 15:04:05 MST"
)

func FormatInUTC(t time.Time) string {
	return t.UTC().Format(DateTimeFormat)
}
