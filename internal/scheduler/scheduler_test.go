package scheduler

import (
	"testing"
	"time"
)

// TestReminderDue covers the before/after cadence and disabling rules (FR-4.5).
func TestReminderDue(t *testing.T) {
	today := time.Date(2025, 6, 30, 12, 0, 0, 0, time.UTC)
	day := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

	tests := []struct {
		name          string
		status        string
		due           time.Time
		before, after int
		want          bool
	}{
		{"before: sent, 3 days ahead, before=3", "sent", day(2025, 7, 3), 3, 0, true},
		{"before: wrong day", "sent", day(2025, 7, 2), 3, 0, false},
		{"before: not sent", "draft", day(2025, 7, 3), 3, 0, false},
		{"after: overdue exactly N days", "overdue", day(2025, 6, 23), 0, 7, true}, // 7 days overdue
		{"after: overdue 2N days", "overdue", day(2025, 6, 16), 0, 7, true},        // 14 days
		{"after: overdue non-multiple", "overdue", day(2025, 6, 27), 0, 7, false},  // 3 days
		{"after: sent past due multiple", "sent", day(2025, 6, 23), 0, 7, true},
		{"disabled both", "overdue", day(2025, 6, 23), 0, 0, false},
		{"paid never", "paid", day(2025, 6, 23), 3, 7, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reminderDue(tt.status, tt.due, today, tt.before, tt.after); got != tt.want {
				t.Errorf("reminderDue(%s, due=%s, before=%d, after=%d) = %v; want %v",
					tt.status, tt.due.Format("2006-01-02"), tt.before, tt.after, got, tt.want)
			}
		})
	}
}
