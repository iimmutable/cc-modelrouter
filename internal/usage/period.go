package usage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParsePeriod converts a period string to start and end times.
func ParsePeriod(period string, now time.Time) (start, end time.Time, err error) {
	switch period {
	case "all-time", "":
		return time.Time{}, now.AddDate(100, 0, 0), nil

	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "this-week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day()+(7-weekday), 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "last-week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = time.Date(now.Year(), now.Month(), now.Day()-weekday+1-7, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), now.Month(), now.Day()-weekday, 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "this-month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		// Get last day of current month
		nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
		end = nextMonth.Add(-time.Second)
		return start, end, nil

	case "last-month":
		start = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
		// Get last day of last month
		thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end = thisMonth.Add(-time.Second)
		return start, end, nil

	case "this-quarter":
		quarter := (int(now.Month()) - 1) / 3
		start = time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, now.Location())
		nextQuarter := time.Date(now.Year(), time.Month(quarter*3+4), 1, 0, 0, 0, 0, now.Location())
		end = nextQuarter.Add(-time.Second)
		return start, end, nil

	case "last-quarter":
		quarter := (int(now.Month()) - 1) / 3
		start = time.Date(now.Year(), time.Month(quarter*3-2), 1, 0, 0, 0, 0, now.Location())
		thisQuarter := time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, now.Location())
		end = thisQuarter.Add(-time.Second)
		return start, end, nil

	case "this-year":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year(), 12, 31, 23, 59, 59, 0, now.Location())
		return start, end, nil

	case "last-year":
		start = time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
		end = time.Date(now.Year()-1, 12, 31, 23, 59, 59, 0, now.Location())
		return start, end, nil

	default:
		// Try custom range YYYYMMDD-YYYYMMDD
		return parseCustomRange(period)
	}
}

func parseCustomRange(period string) (start, end time.Time, err error) {
	// Format: YYYYMMDD-YYYYMMDD
	idx := strings.Index(period, "-")
	if idx == -1 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period format: %s (expected YYYYMMDD-YYYYMMDD)", period)
	}

	startStr := period[:idx]
	endStr := period[idx+1:]

	start, err = parseDate(startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date: %w", err)
	}

	end, err = parseDate(endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date: %w", err)
	}

	// Validate that start <= end
	if start.After(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period: start date must be before or equal to end date")
	}

	// Set end to end of day
	end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, end.Location())

	return start, end, nil
}

func parseDate(s string) (time.Time, error) {
	if len(s) != 8 {
		return time.Time{}, fmt.Errorf("invalid date format")
	}

	year, err := strconv.Atoi(s[0:4])
	if err != nil {
		return time.Time{}, err
	}

	month, err := strconv.Atoi(s[4:6])
	if err != nil {
		return time.Time{}, err
	}

	day, err := strconv.Atoi(s[6:8])
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}
