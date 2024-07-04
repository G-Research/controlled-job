package schedule

import (
	"testing"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/stretchr/testify/assert"
)

var (
	testDate time.Time = time.Date(2022, 02, 04, 0, 0, 0, 0, time.UTC)

	nyLoc, _             = time.LoadLocation("America/New_York")
	testDateNy time.Time = time.Date(2022, 02, 04, 0, 0, 0, 0, nyLoc)
	// Half a day's worth of hours
	hours []time.Time = []time.Time{
		testDate,
		testDate.Add(1 * time.Hour),
		testDate.Add(2 * time.Hour),
		testDate.Add(3 * time.Hour),
		testDate.Add(4 * time.Hour),
		testDate.Add(5 * time.Hour),
		testDate.Add(6 * time.Hour),
		testDate.Add(7 * time.Hour),
		testDate.Add(8 * time.Hour),
		testDate.Add(9 * time.Hour),
		testDate.Add(10 * time.Hour),
		testDate.Add(11 * time.Hour),
		testDate.Add(12 * time.Hour),
	}
	hoursNy []time.Time = []time.Time{
		testDateNy,
		testDateNy.Add(1 * time.Hour),
		testDateNy.Add(2 * time.Hour),
		testDateNy.Add(3 * time.Hour),
		testDateNy.Add(4 * time.Hour),
		testDateNy.Add(5 * time.Hour),
		testDateNy.Add(6 * time.Hour),
		testDateNy.Add(7 * time.Hour),
		testDateNy.Add(8 * time.Hour),
		testDateNy.Add(9 * time.Hour),
		testDateNy.Add(10 * time.Hour),
		testDateNy.Add(11 * time.Hour),
		testDateNy.Add(12 * time.Hour),
	}
)

func Test_findStartOfCurrentRunPeriod(t *testing.T) {

	// TODO: Test things like:
	// No stop events
	// No start events
	// now == time of stop event
	// now == time of start event
	// now == between stop and start
	// now == between start and stop (with multiple start times)

	utcTimezone := batch.TimezoneSpec{
		Name:          "UTC",
		OffsetSeconds: 0,
	}
	nyTimezone := batch.TimezoneSpec{
		Name:          "America/New_York",
		OffsetSeconds: 0,
	}

	testCases := map[string]struct {
		events   []batch.EventSpec
		timezone batch.TimezoneSpec
		now      time.Time
		expected *time.Time
	}{
		"no stop events": {
			events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 0 * * * ",
				},
			},
			timezone: utcTimezone,
			now:      hours[1],
			expected: nil,
		},
		"no events": {
			events:   []batch.EventSpec{},
			timezone: utcTimezone,
			now:      hours[1],
			expected: nil,
		},
		"now == stop time": {
			events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 1 * * * ",
				},
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 2 * * * ",
				},
			},
			timezone: utcTimezone,
			now:      hours[1],
			expected: &hours[2],
		},
		"now == start time": {
			events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 1 * * * ",
				},
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 2 * * * ",
				},
			},
			timezone: utcTimezone,
			now:      hours[2],
			expected: &hours[2],
		},
		"now is between stop and start time": {
			// stop at 1am, now is 2am, start at 3am
			events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 1 * * * ",
				},
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 3 * * * ",
				},
			},
			timezone: utcTimezone,
			now:      hours[2],
			expected: &hours[3],
		},
		"now is between start and stop time": {
			// start at 3am, now is 4am, stop at 5am
			events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 3 * * * ",
				},
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 5 * * * ",
				},
			},
			timezone: utcTimezone,
			now:      hours[4],
			expected: &hours[3],
		},
		"now is between start and stop time (with multiple overlapping start and stop times": {
			events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 1 * * * ",
				},
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 2 * * * ",
				},
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 3 * * * ",
				},
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 4 * * * ",
				},
			},
			timezone: utcTimezone,
			now:      hours[5],
			expected: &hours[3],
		},
		"now is after stop time in UTC, but before it in NY": {
			// stop at 1am, now is 2am, start at 3am
			events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 1 * * * ",
				},
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 5 * * * ",
				},
			},
			timezone: nyTimezone,
			now:      hours[7],  // 7am UTC == 2am NY
			expected: &hours[6], // 1am NY (=6am UTC) today, not 1am tomorrow
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			location, _ := time.LoadLocation(tc.timezone.Name)
			actual, _ := findStartOfCurrentRunPeriod(tc.events, locationWithOffset{location, tc.timezone.OffsetSeconds}, tc.now)

			if actual == nil {
				assert.Nil(t, tc.expected)
			} else {
				assert.Equal(t, *tc.expected, *actual, "%v (expected) != %v (actual)", tc.expected, *actual)
			}
		})
	}
}

func Test_StateFor(t *testing.T) {
	controlledJob := &batch.ControlledJob{
		Spec: batch.ControlledJobSpec{
			Timezone: batch.TimezoneSpec{
				Name: "America/New_York",
			},
			Events: []batch.EventSpec{
				{
					Action:       batch.EventTypeStart,
					CronSchedule: "0 3 * * * ",
				},
				{
					Action:       batch.EventTypeStop,
					CronSchedule: "0 5 * * * ",
				},
			},
		},
	}

	// 9am UTC = 4am New_York
	now := hours[9]

	sut, err := StateFor(controlledJob, now)

	assert.Nil(t, err, "Should not return an error")

	nextEventTime := sut.NextEventTime()
	actualStartOfRunPeriod := sut.StartOfCurrentRunPeriod()

	assert.Equal(t, hours[10], *nextEventTime, "%v (expected) != %v (actual)", hours[10], sut.NextEventTime())
	assert.True(t, sut.ShouldBeRunning(), "Expect ShouldBeRunning to be true")
	assert.Equal(t, hours[8], *actualStartOfRunPeriod, "%v (expected) != %v (actual)", hours[8], sut.StartOfCurrentRunPeriod())
}
