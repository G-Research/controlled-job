package schedule

import (
	"time"

	"github.com/robfig/cron/v3"
)

// cronPrev returns the most recent previous time this schedule was activated, less than OR EQUAL TO the given
// time.  If no time can be found to satisfy the schedule, return the zero time.
// This is the inverse of SpecSchedule.Next
func cronPrev(s *cron.SpecSchedule, t time.Time) time.Time {
	// General approach:
	// t is always the current timestamp being considered, and we keep going backwards in time
	// until we find the first second that matches all restrictions

	// For Month, Day, Hour, Minute, Second:
	// Check if the time value matches.  If yes, continue to the next field.
	// If the field doesn't match the schedule, then truncate the other fields and go back 1s and try again

	// For example, if
	//   t = 2022/04/10 02:34:56
	// but the schedule says to run on the 8th of the month then the 'day' check will fail. So we truncate
	// the fields after day:
	//   t = 2022/04/10 00:00:00
	// and then take exactly one second off:
	//   t = 2022/04/09 23:59:59
	// The day check still fails so we truncate the time again:
	//   t = 2022/04/09 00:00:00
	// and take another second off:
	//   t = 2022/04/08 23:59:59
	// Now the day check succeeds, and we can proceed to compare hours, and so on

	// While decrementing the field, if we end up wrapping back to the end of the previous field
	// (e.g. we go back 1s from 2022/04/01 00:00:00 to 2022/03/30 23:59:59) then we loop back to
	// the start (comparing month again) to make sure all the checks still match.
	// Case in point: imagine a schedule runs on the 5th of October, and t is the 4th October 2022
	// The month check initially succeeds, but the day check fails until we wrap back (eventually) to the
	// 5th September. But at that point the month is no longer correct. The easiest thing to do is start again
	// from the start and keep going until the month matches again (in this case when we reach October 2021)

	// Convert the given time into the schedule's timezone, if one is specified.
	// Save the original timezone so we can convert back after we find a time.
	// Note that schedules without a time zone specified (time.Local) are treated
	// as local to the time provided.
	origLocation := t.Location()
	loc := s.Location
	if loc == time.Local {
		loc = t.Location()
	}
	if s.Location != time.Local {
		t = t.In(s.Location)
	}

	// If no time is found within five years, return zero.
	yearLimit := t.Year() - 5

WRAP:
	if t.Year() < yearLimit {
		return time.Time{}
	}

	// Find the last applicable month.
	// If it's this month, then do nothing.
	for 1<<uint(t.Month())&s.Month == 0 {
		t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		t = t.Add(-1 * time.Second)

		// Wrapped around.
		if t.Month() == time.December {
			goto WRAP
		}
	}

	for !dayMatches(s, t) {

		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

		// If the current day is the 1st, then by subtracting 1
		// we might be going back a year (January->December) so
		// WRAP back to recheck the year limit
		// We may also now be in a non matching month
		needToWrapBack := t.Day() == 1
		t = t.Add(-1 * time.Second)

		if needToWrapBack {
			goto WRAP
		}

	}

	for 1<<uint(t.Hour())&s.Hour == 0 {

		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
		t = t.Add(-1 * time.Second)

		if t.Hour() == 23 {
			goto WRAP
		}
	}

	for 1<<uint(t.Minute())&s.Minute == 0 {
		t = t.Truncate(time.Minute)
		t = t.Add(-1 * time.Second)

		if t.Minute() == 59 {
			goto WRAP
		}
	}

	for 1<<uint(t.Second())&s.Second == 0 {
		t = t.Truncate(time.Second)
		t = t.Add(-1 * time.Second)

		if t.Second() == 59 {
			goto WRAP
		}
	}

	return t.In(origLocation)
}

const (
	// Set the top bit if a star was included in the expression.
	// Copied from cron package
	starBit = 1 << 63
)

// dayMatches returns true if the schedule's day-of-week and day-of-month
// restrictions are satisfied by the given time.
func dayMatches(s *cron.SpecSchedule, t time.Time) bool {
	var (
		domMatch bool = 1<<uint(t.Day())&s.Dom > 0
		dowMatch bool = 1<<uint(t.Weekday())&s.Dow > 0
	)
	if s.Dom&starBit > 0 || s.Dow&starBit > 0 {
		return domMatch && dowMatch
	}
	return domMatch || dowMatch
}
