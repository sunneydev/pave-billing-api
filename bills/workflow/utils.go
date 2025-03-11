package workflow

import "time"

func calculateTimeUntilNextMonth() time.Duration {
	now := time.Now().UTC()
	currentYear, currentMonth, _ := now.Date()
	nextMonth := currentMonth + 1
	nextYear := currentYear

	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}

	nextMonthStart := time.Date(nextYear, nextMonth, 1, 0, 0, 0, 0, time.UTC)
	return nextMonthStart.Sub(now)
}
