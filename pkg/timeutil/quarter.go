package timeutil

import "time"

// MonthsPerQuarter is the number of months in a calendar quarter.
const MonthsPerQuarter = 3

// QuarterStart returns the first day of the quarter containing t.
func QuarterStart(t time.Time) time.Time {
	month := (int(t.Month())-1)/MonthsPerQuarter*MonthsPerQuarter + 1
	return time.Date(t.Year(), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}
