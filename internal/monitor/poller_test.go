package monitor

import (
	"testing"
	"time"
)

func TestCalculateDateRange(t *testing.T) {
	sp := &StatsPoller{DBPath: "/tmp/test.db"}

	t.Run("DateRangeToday", func(t *testing.T) {
		start, end := sp.calculateDateRange(DateRangeToday)
		now := time.Now()
		expectedStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		expectedEnd := expectedStart.Add(24 * time.Hour)

		if !start.Equal(expectedStart) {
			t.Errorf("start: expected %v, got %v", expectedStart, start)
		}
		if !end.Equal(expectedEnd) {
			t.Errorf("end: expected %v, got %v", expectedEnd, end)
		}
	})

	t.Run("DateRangeWeek", func(t *testing.T) {
		start, end := sp.calculateDateRange(DateRangeWeek)
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		expectedStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		expectedEnd := expectedStart.Add(7 * 24 * time.Hour)

		if !start.Equal(expectedStart) {
			t.Errorf("start: expected %v, got %v", expectedStart, start)
		}
		if !end.Equal(expectedEnd) {
			t.Errorf("end: expected %v, got %v", expectedEnd, end)
		}
	})

	t.Run("DateRangeMonth", func(t *testing.T) {
		start, end := sp.calculateDateRange(DateRangeMonth)
		now := time.Now()
		expectedStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		expectedEnd := expectedStart.AddDate(0, 1, 0)

		if !start.Equal(expectedStart) {
			t.Errorf("start: expected %v, got %v", expectedStart, start)
		}
		if !end.Equal(expectedEnd) {
			t.Errorf("end: expected %v, got %v", expectedEnd, end)
		}
	})

	t.Run("DateRangeYTD", func(t *testing.T) {
		before := time.Now()
		start, end := sp.calculateDateRange(DateRangeYTD)
		after := time.Now()
		expectedStart := time.Date(before.Year(), 1, 1, 0, 0, 0, 0, before.Location())

		if !start.Equal(expectedStart) {
			t.Errorf("start: expected %v, got %v", expectedStart, start)
		}
		// end should be approximately now (between before and after)
		if end.Before(before) || end.After(after) {
			t.Errorf("end: expected between %v and %v, got %v", before, after, end)
		}
	})

	t.Run("DateRangeTTM", func(t *testing.T) {
		before := time.Now()
		start, end := sp.calculateDateRange(DateRangeTTM)
		after := time.Now()
		expectedStart := before.AddDate(-1, 0, 0)

		if !start.Equal(expectedStart) {
			t.Errorf("start: expected %v, got %v", expectedStart, start)
		}
		// end should be approximately now
		if end.Before(before) || end.After(after) {
			t.Errorf("end: expected between %v and %v, got %v", before, after, end)
		}
	})

	t.Run("default (unknown)", func(t *testing.T) {
		start, end := sp.calculateDateRange(DateRange(99))
		// default returns now, now — both should be equal
		if !start.Equal(end) {
			t.Errorf("default should return start == end, got start=%v end=%v", start, end)
		}
	})
}

func TestCalculateDateRangeForInstances(t *testing.T) {
	t.Run("DateRangeWeek", func(t *testing.T) {
		start, end := calculateDateRangeForInstances(DateRangeWeek)
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		expectedStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		expectedEnd := expectedStart.AddDate(0, 3, 0)

		if !start.Equal(expectedStart) {
			t.Errorf("start: expected %v, got %v", expectedStart, start)
		}
		if !end.Equal(expectedEnd) {
			t.Errorf("end: expected %v (3 months ahead), got %v", expectedEnd, end)
		}
	})
}
