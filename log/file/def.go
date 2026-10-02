package file

import "github.com/dobyte/due/v2/log/internal"

// Rotate is a log rotation rule.
type Rotate string

const (
	RotateNone  Rotate = "none"  // No rotation
	RotateYear  Rotate = "year"  // Rotate by year
	RotateMonth Rotate = "month" // Rotate by month
	RotateWeek  Rotate = "week"  // Rotate by week
	RotateDay   Rotate = "day"   // Rotate by day
	RotateHour  Rotate = "hour"  // Rotate by hour
)

// Format is the log output format.
type Format = internal.Format

const (
	FormatText = internal.FormatText // Text format
	FormatJson = internal.FormatJson // JSON format
)
