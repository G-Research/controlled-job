package schedule

import (
	"testing"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/testhelpers"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
)

func Test_FindNearestEvent(t *testing.T) {
	morningOnWednesdayUTC := time.Date(2022, 1, 19, 9, 0, 0, 0, time.UTC)
	lunchtimeOnWednesdayUTC := time.Date(2022, 1, 19, 12, 0, 0, 0, time.UTC)
	eveningOnWednesdayUTC := time.Date(2022, 1, 19, 17, 0, 0, 0, time.UTC)
	suppertimeOnWednesdayUTC := time.Date(2022, 1, 19, 20, 0, 0, 0, time.UTC)
	morningOnThursdayUTC := time.Date(2022, 1, 20, 9, 0, 0, 0, time.UTC)

	// 14:05 in UTC is 9:05am in EST. And so when we compare them, the most recent event should
	// be 9am in EST
	fiveMinutesAfter9amNYInUTC := time.Date(2022, 1, 19, 14, 5, 0, 0, time.UTC)

	estLoc, _ := time.LoadLocation("EST")

	morningOnWednesdayEST := time.Date(2022, 1, 19, 9, 0, 0, 0, estLoc)
	lunchtimeOnWednesdayEST := time.Date(2022, 1, 19, 12, 0, 0, 0, estLoc)
	eveningOnWednesdayEST := time.Date(2022, 1, 19, 17, 0, 0, 0, estLoc)
	suppertimeOnWednesdayEST := time.Date(2022, 1, 19, 20, 0, 0, 0, estLoc)
	morningOnThursdayEST := time.Date(2022, 1, 20, 9, 0, 0, 0, estLoc)

	testCases := map[string]testCase{

		//
		// Error conditions
		//
		"no events": newTest(
			utc,
			[]batch.EventSpec{},
			lunchtimeOnWednesdayUTC,
			// Should return no adjacent events
			nil,
			nil,
			// Should return no errors
			nil,
			nil,
		),
		"invalid cron format": newTest(
			utc,
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "I AM INVALID"),
			},
			lunchtimeOnWednesdayUTC,
			// Should return no adjacent events
			nil,
			nil,
			// Should return expected errors
			errors.New("Failed to parse cron schedule: expected exactly 5 fields, found 3: [I AM INVALID]"),
			errors.New("Failed to parse cron schedule: expected exactly 5 fields, found 3: [I AM INVALID]"),
		),
		"invalid schedule format": newTest(
			utc,
			[]batch.EventSpec{
				event(batch.EventTypeStart, "", "MON"),
			},
			lunchtimeOnWednesdayUTC,
			// Should return no adjacent events
			nil,
			nil,
			// Should return expected errors
			errors.New("must specify either cronSchedule or schedule"),
			errors.New("must specify either cronSchedule or schedule"),
		),

		//
		// Happy paths
		//
		"middle of cron period (UTC)": newTest(
			utc,
			// CRON schedule: start at 9am on weekdays, stop at 5pm on weekdays
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "0 9 * * MON-FRI"),
				cronEvent(batch.EventTypeStop, "0 17 * * MON-FRI"),
			},
			// Now is lunchtime on Wednesday
			lunchtimeOnWednesdayUTC,
			// Expect to be between start and stop
			schedEv(batch.EventTypeStart, morningOnWednesdayUTC),
			schedEv(batch.EventTypeStop, eveningOnWednesdayUTC),
			// No errors
			nil,
			nil,
		),

		"outside of cron period (UTC)": newTest(
			utc,
			// CRON schedule: start at 9am on weekdays, stop at 5pm on weekdays
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "0 9 * * MON-FRI"),
				cronEvent(batch.EventTypeStop, "0 17 * * MON-FRI"),
			},
			// Now is lunchtime on Wednesday
			suppertimeOnWednesdayUTC,
			// Expect to be between start and stop
			schedEv(batch.EventTypeStop, eveningOnWednesdayUTC),
			schedEv(batch.EventTypeStart, morningOnThursdayUTC),
			// No errors
			nil,
			nil,
		),
		"exactly match start time": newTest(
			utc,
			// CRON schedule: start at 9am on weekdays, stop at 5pm on weekdays
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "0 9 * * MON-FRI"),
				cronEvent(batch.EventTypeStop, "0 17 * * MON-FRI"),
			},
			// Now is morning on Wednesday
			morningOnWednesdayUTC,
			// Expect to be between start and stop
			schedEv(batch.EventTypeStart, morningOnWednesdayUTC),
			schedEv(batch.EventTypeStop, eveningOnWednesdayUTC),
			// No errors
			nil,
			nil,
		),

		"middle of cron period (EST)": newTest(
			est,
			// CRON schedule: start at 9am on weekdays, stop at 5pm on weekdays
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "0 9 * * MON-FRI"),
				cronEvent(batch.EventTypeStop, "0 17 * * MON-FRI"),
			},
			// Now is lunchtime on Wednesday
			lunchtimeOnWednesdayEST.In(time.UTC),
			// Expect to be between start and stop
			schedEv(batch.EventTypeStart, morningOnWednesdayEST.In(time.UTC)),
			schedEv(batch.EventTypeStop, eveningOnWednesdayEST.In(time.UTC)),
			// No errors
			nil,
			nil,
		),

		"outside of cron period (EST)": newTest(
			est,
			// CRON schedule: start at 9am on weekdays, stop at 5pm on weekdays
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "0 9 * * MON-FRI"),
				cronEvent(batch.EventTypeStop, "0 17 * * MON-FRI"),
			},
			// Now is lunchtime on Wednesday
			suppertimeOnWednesdayEST.In(time.UTC),
			// Expect to be between start and stop
			schedEv(batch.EventTypeStop, eveningOnWednesdayEST.In(time.UTC)),
			schedEv(batch.EventTypeStart, morningOnThursdayEST.In(time.UTC)),
			// No errors
			nil,
			nil,
		),

		// EST timezone in spec, but 'now' (in the controller) is UTC
		"EST vs UTC compares correctly": newTest(
			est,
			// CRON schedule: start at 9am on weekdays, stop at 5pm on weekdays
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "0 9 * * MON-FRI"),
				cronEvent(batch.EventTypeStop, "0 12 * * MON-FRI"),
			},
			// Now is lunchtime (14:05) in UTC, which is just after 9am in EST
			// 14:05 is after the stop time, but 09:05 is between the start and stop
			fiveMinutesAfter9amNYInUTC,
			// Expect to be between start and stop
			schedEv(batch.EventTypeStart, time.Date(2022, 1, 19, 14, 0, 0, 0, time.UTC)),
			schedEv(batch.EventTypeStop, time.Date(2022, 1, 19, 17, 0, 0, 0, time.UTC)),
			// No errors
			nil,
			nil,
		),
		"EST vs UTC compares correctly and obeys additional offset": newTest(
			// -360s = -6 minutes.
			// That means the scheduled start time of 09:00 in this timezone is
			// 14:06 UTC, not 14:00. So 14:05 UTC is no longer inside the start period
			batch.TimezoneSpec{Name: "EST", OffsetSeconds: -360},
			// CRON schedule: start at 9am on weekdays, stop at 5pm on weekdays
			[]batch.EventSpec{
				cronEvent(batch.EventTypeStart, "0 9 * * MON-FRI"),
				cronEvent(batch.EventTypeStop, "0 12 * * MON-FRI"),
			},
			fiveMinutesAfter9amNYInUTC,
			// Expect to be between stop and start
			schedEv(batch.EventTypeStop, time.Date(2022, 1, 18, 17, 6, 0, 0, time.UTC)),
			schedEv(batch.EventTypeStart, time.Date(2022, 1, 19, 14, 6, 0, 0, time.UTC)),
			// No errors
			nil,
			nil,
		),
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			location, _ := time.LoadLocation(tc.timezone.Name)
			loc := locationWithOffset{
				location,
				tc.timezone.OffsetSeconds,
			}

			previous, prErr := findNearestEvent(tc.events, tc.now, loc, directionPrevious, func(es batch.EventSpec) bool { return true })
			next, neErr := findNearestEvent(tc.events, tc.now, loc, directionNext, func(es batch.EventSpec) bool { return true })

			testhelpers.AssertDeepEqualJson(t, tc.expectedPreviousEvent, previous, "expected nearest previous events to match")
			testhelpers.AssertSameError(t, tc.expectedErrorForPrevious, prErr, "expected error for getting previous event to match")
			testhelpers.AssertDeepEqualJson(t, tc.expectedNextEvent, next, "expected nearest next events to match")
			testhelpers.AssertSameError(t, tc.expectedErrorForNext, neErr, "expected error for getting next event to match")

			if previous != nil {
				assert.Equal(t, time.UTC.String(), previous.ScheduledTimeUTC.Location().String())
			}
			if next != nil {
				assert.Equal(t, time.UTC.String(), next.ScheduledTimeUTC.Location().String())
			}
		})
	}

}

func Test_ParanoidTimezoneOffsetTests(t *testing.T) {
	// A set of more paranoid tests for timezone handling in this logic
	t.Run("Additional offsets", func(t *testing.T) {
		t.Run("Timezone plus extra positive offset works correctly", func(t *testing.T) {
			// From the documentation for OffsetSeconds in our CRD definition:
			//
			// Additional offset from UTC on top of the specified timezone. If the timezone is normally UTC-2, and
			// OffsetSeconds is +3600 (1h in seconds), then the overall effect will be UTC-1. If the timezone is
			// normally UTC+2 and OffsetSeconds is +3600, then the overall effect will be UTC+3.
			//
			// In practice - if you set this field to 60s on top of a normally UTC-1 timezone, then you end up with
			// a 'UTC-59m' timezone. In that timezone 10:00 UTC == 09:01 UTC-59m. So if you have a scheduled start time
			// of 09:00 in that UTC-59m timezone, your job will be started at 09:59 UTC

			// Note: Etc/GMT+1 is actually UTC-1: https://en.wikipedia.org/wiki/Tz_database#Area
			//  In order to conform with the POSIX style, those zone names beginning with "Etc/GMT" have their sign reversed from the standard ISO 8601 convention.
			//  In the "Etc" area, zones west of GMT have a positive sign and those east have a negative sign in their name (e.g "Etc/GMT-14" is 14 hours ahead of GMT).
			utcMinus1Loc, _ := time.LoadLocation("Etc/GMT+1")

			// Set up UTC-1 with 60s offset == 'UTC-59m'
			locationWithOffset := locationWithOffset{utcMinus1Loc, 60}

			events := []batch.EventSpec{
				event(batch.EventTypeStart, "09:00", "SUN-SAT"),
			}

			// 9am UTC-59m == 09:59 UTC
			// So at 09:58 UTC the next event should be today's start event, but a minute later at 09:59 the next event will be the start event tomorrow

			startEvent_yesterday := time.Date(2021, time.December, 31, 9, 59, 0, 0, time.UTC)
			startEvent_today := time.Date(2022, time.January, 1, 9, 59, 0, 0, time.UTC)
			startEvent_tomorrow := time.Date(2022, time.January, 2, 9, 59, 0, 0, time.UTC)

			nowIs_9_58_UTC := time.Date(2022, time.January, 1, 9, 58, 0, 0, time.UTC)
			nowIs_9_59_UTC := time.Date(2022, time.January, 1, 9, 59, 0, 0, time.UTC)

			previous_9_58, _ := findNearestEvent(events, nowIs_9_58_UTC, locationWithOffset, directionPrevious, func(es batch.EventSpec) bool { return true })
			next_9_58, _ := findNearestEvent(events, nowIs_9_58_UTC, locationWithOffset, directionNext, func(es batch.EventSpec) bool { return true })

			previous_9_59, _ := findNearestEvent(events, nowIs_9_59_UTC, locationWithOffset, directionPrevious, func(es batch.EventSpec) bool { return true })
			next_9_59, _ := findNearestEvent(events, nowIs_9_59_UTC, locationWithOffset, directionNext, func(es batch.EventSpec) bool { return true })

			datesShouldMatch(t, startEvent_yesterday, previous_9_58.ScheduledTimeUTC)
			datesShouldMatch(t, startEvent_today, next_9_58.ScheduledTimeUTC)
			datesShouldMatch(t, startEvent_today, previous_9_59.ScheduledTimeUTC)
			datesShouldMatch(t, startEvent_tomorrow, next_9_59.ScheduledTimeUTC)

		})
		t.Run("Timezone plus extra negative offset works correctly", func(t *testing.T) {
			// From the documentation for OffsetSeconds in our CRD definition:
			//
			// Additional offset from UTC on top of the specified timezone. If the timezone is normally UTC-2, and
			// OffsetSeconds is +3600 (1h in seconds), then the overall effect will be UTC-1. If the timezone is
			// normally UTC+2 and OffsetSeconds is +3600, then the overall effect will be UTC+3.
			//
			// In practice - if you set this field to 60s on top of a normally UTC-1 timezone, then you end up with
			// a 'UTC-59m' timezone. In that timezone 10:00 UTC == 09:01 UTC-59m. So if you have a scheduled start time
			// of 09:00 in that UTC-59m timezone, your job will be started at 09:59 UTC

			// Note: Etc/GMT+1 is actually UTC-1: https://en.wikipedia.org/wiki/Tz_database#Area
			//  In order to conform with the POSIX style, those zone names beginning with "Etc/GMT" have their sign reversed from the standard ISO 8601 convention.
			//  In the "Etc" area, zones west of GMT have a positive sign and those east have a negative sign in their name (e.g "Etc/GMT-14" is 14 hours ahead of GMT).
			utcMinus1Loc, _ := time.LoadLocation("Etc/GMT+1")

			// Set up UTC-1 with 60s offset == 'UTC-61m'
			locationWithOffset := locationWithOffset{utcMinus1Loc, -60}

			events := []batch.EventSpec{
				event(batch.EventTypeStart, "09:00", "SUN-SAT"),
			}

			// 9am UTC-61m == 10:01 UTC
			// So at 10:00 UTC the next event should be today's start event, but a minute later at 10:01 the next event will be the start event tomorrow

			startEvent_yesterday := time.Date(2021, time.December, 31, 10, 01, 0, 0, time.UTC)
			startEvent_today := time.Date(2022, time.January, 1, 10, 01, 0, 0, time.UTC)
			startEvent_tomorrow := time.Date(2022, time.January, 2, 10, 01, 0, 0, time.UTC)

			nowIs_10_00_UTC := time.Date(2022, time.January, 1, 10, 0, 0, 0, time.UTC)
			nowIs_10_01_UTC := time.Date(2022, time.January, 1, 10, 1, 0, 0, time.UTC)

			previous_10_00, _ := findNearestEvent(events, nowIs_10_00_UTC, locationWithOffset, directionPrevious, func(es batch.EventSpec) bool { return true })
			next_10_00, _ := findNearestEvent(events, nowIs_10_00_UTC, locationWithOffset, directionNext, func(es batch.EventSpec) bool { return true })

			previous_10_01, _ := findNearestEvent(events, nowIs_10_01_UTC, locationWithOffset, directionPrevious, func(es batch.EventSpec) bool { return true })
			next_10_01, _ := findNearestEvent(events, nowIs_10_01_UTC, locationWithOffset, directionNext, func(es batch.EventSpec) bool { return true })

			datesShouldMatch(t, startEvent_yesterday, previous_10_00.ScheduledTimeUTC)
			datesShouldMatch(t, startEvent_today, next_10_00.ScheduledTimeUTC)
			datesShouldMatch(t, startEvent_today, previous_10_01.ScheduledTimeUTC)
			datesShouldMatch(t, startEvent_tomorrow, next_10_01.ScheduledTimeUTC)

		})
	})
	t.Run("DST transitions", func(t *testing.T) {
		t.Run("Skips scheduled times when clocks go forward over them", func(t *testing.T) {
			/* From https://www.timeanddate.com/time/change/usa/new-york?year=2022:

					When local standard time was about to reach
					Sunday, 13 March 2022, 02:00:00 clocks were turned forward 1 hour to
					Sunday, 13 March 2022, 03:00:00 local daylight time instead.

			That means that, say, 2:30am on 13th March never existed. Because the cron calculation
			works by incrementally adding/subtracting time until a matching time is hit, it will
			skip over cases like this, and so the scheduled time will never get hit.

			I think that's correct behaviour. The user's said 'start at 2:30am New York time' but that
			time didn't exist that day
			*/

			nyLocation, _ := time.LoadLocation("America/New_York")
			nyDstDate := time.Date(2022, time.March, 13, 0, 0, 0, 0, nyLocation)

			events := []batch.EventSpec{
				event(batch.EventTypeStart, "02:30", "SUN-SAT"),
				event(batch.EventTypeStop, "09:00", "SUN-SAT"),
			}

			now := nyDstDate.Add(4 * time.Hour).In(time.UTC)
			t.Log(now)

			// At 4 am the most recent event should be the stop event from yesterday
			previousEvent, err := findNearestEvent(events, now, locationWithOffset{nyLocation, 0}, directionPrevious, func(es batch.EventSpec) bool { return true })

			assert.Nil(t, err)
			assert.Equal(t, batch.EventTypeStop, previousEvent.Type)
			assert.Equal(t, time.Date(2022, time.March, 12, 9, 0, 0, 0, nyLocation).In(time.UTC), previousEvent.ScheduledTimeUTC)
		})
		t.Run("Chooses nearest occurance of duplicate time when clock go back", func(t *testing.T) {
			/* From https://www.timeanddate.com/time/change/usa/new-york?year=2022:

					When local daylight time is about to reach
					Sunday, 6 November 2022, 02:00:00 clocks are turned backward 1 hour to
					Sunday, 6 November 2022, 01:00:00 local standard time instead.

			That means that, say, 1:30am on 6 November happened twice. So depending on where
			we approach that from, we'd get a different nearest event (i.e. different UTC timestamps)
			*/

			nyLocation, _ := time.LoadLocation("America/New_York")
			nyDstDate := time.Date(2022, time.November, 6, 0, 0, 0, 0, nyLocation)

			events := []batch.EventSpec{
				event(batch.EventTypeStart, "01:30", "SUN-SAT"),
				event(batch.EventTypeStop, "09:00", "SUN-SAT"),
			}

			nowAfterChange := nyDstDate.Add(4 * time.Hour).In(time.UTC)
			t.Log(nowAfterChange)

			// At 4 am the most recent event should be 1:30am in the new (non DST) timezone, which is UTC-5
			previousEvent, err := findNearestEvent(events, nowAfterChange, locationWithOffset{nyLocation, 0}, directionPrevious, func(es batch.EventSpec) bool { return true })

			assert.Nil(t, err)
			assert.Equal(t, batch.EventTypeStart, previousEvent.Type)
			assert.Equal(t, time.Date(2022, time.November, 6, 6, 30, 0, 0, time.UTC), previousEvent.ScheduledTimeUTC, "%s != %s", time.Date(2022, time.November, 6, 5, 30, 0, 0, time.UTC), previousEvent.ScheduledTimeUTC)

			nowBeforeChange := nyDstDate.In(time.UTC)
			t.Log(nowBeforeChange)

			// At 1 am the next event should be 1:30am in the old (DST) timezone, which is UTC-4
			nextEvent, err := findNearestEvent(events, nowBeforeChange, locationWithOffset{nyLocation, 0}, directionNext, func(es batch.EventSpec) bool { return true })

			assert.Nil(t, err)
			assert.Equal(t, batch.EventTypeStart, nextEvent.Type)
			assert.Equal(t, time.Date(2022, time.November, 6, 5, 30, 0, 0, time.UTC), nextEvent.ScheduledTimeUTC)
		})
	})
}

func datesShouldMatch(t *testing.T, a, b time.Time) {
	assert.Equal(t, a, b, "%s != %s", a, b)
}

// testCase (and the following functions) is a helper to build a test case for testing the FindNearestEvent function
type testCase struct {
	timezone                 batch.TimezoneSpec
	events                   []batch.EventSpec
	now                      time.Time
	expectedPreviousEvent    *ScheduledEvent
	expectedNextEvent        *ScheduledEvent
	expectedErrorForPrevious error
	expectedErrorForNext     error
}

func newTest(timezone batch.TimezoneSpec, events []batch.EventSpec, now time.Time, expectedPreviousEvent *ScheduledEvent, expectedNextEvent *ScheduledEvent, expectedErrorForPrevious error, expectedErrorForNext error) testCase {
	return testCase{
		timezone,
		events,
		now,
		expectedPreviousEvent,
		expectedNextEvent,
		expectedErrorForPrevious,
		expectedErrorForNext,
	}
}

func tz(name string, offsetS int32) batch.TimezoneSpec {
	return batch.TimezoneSpec{
		Name:          name,
		OffsetSeconds: offsetS,
	}
}

var (
	utc batch.TimezoneSpec = tz("UTC", 0)
	est batch.TimezoneSpec = tz("EST", 0)
)

func cronEvent(action batch.EventType, cron string) batch.EventSpec {
	return batch.EventSpec{
		Action:       action,
		CronSchedule: cron,
	}
}

func event(action batch.EventType, timeOfDay string, daysOfWeek string) batch.EventSpec {
	return batch.EventSpec{
		Action: action,
		Schedule: &batch.FriendlyScheduleSpec{
			TimeOfDay:  timeOfDay,
			DaysOfWeek: daysOfWeek,
		},
	}
}

func schedEv(action batch.EventType, time time.Time) *ScheduledEvent {
	return &ScheduledEvent{
		Type:             action,
		ScheduledTimeUTC: time,
	}
}

func Test_findNearestScheduleTime(t *testing.T) {
	// In this method we don't need to do test any complex cron schedules
	// We trust the cron library (and our tests of cronPrev) are sufficiently
	// reliable
	// So just create some simple schedules on midnight on different days of the week

	everyOtherDay := newCronSchedule("0 0 * * */2") // Sunday, Tuesday, Thursday, Saturday
	monday := newCronSchedule("0 0 * * Mon")
	tuesday := newCronSchedule("0 0 * * Tue")
	wednesday := newCronSchedule("0 0 * * Wed")
	thursday := newCronSchedule("0 0 * * Thu")
	friday := newCronSchedule("0 0 * * Fri")

	// Now is midday on Wednesday
	now := time.Date(2022, 02, 02, 12, 0, 0, 0, time.UTC)

	midnightOnWednesday := time.Date(2022, 02, 02, 0, 0, 0, 0, time.UTC)
	midnightOnThursday := time.Date(2022, 02, 03, 0, 0, 0, 0, time.UTC)

	testCases := map[string]struct {
		schedules                []*cron.SpecSchedule
		now                      time.Time
		direction                eventDirection
		additionalOffsetSeconds  int32
		expectedNearestEventTime time.Time
		expectedIdx              int
	}{
		"No schedules": {
			expectedNearestEventTime: time.Time{},
			expectedIdx:              -1,
		},
		"[Previous] Between two events": {
			schedules: []*cron.SpecSchedule{
				wednesday,
				thursday,
			},
			now:                      now,
			direction:                directionPrevious,
			expectedNearestEventTime: midnightOnWednesday,
			expectedIdx:              0,
		},
		"[Next] Between two events": {
			schedules: []*cron.SpecSchedule{
				wednesday,
				thursday,
			},
			now:                      now,
			direction:                directionNext,
			expectedNearestEventTime: midnightOnThursday,
			expectedIdx:              1,
		},
		"[Previous] Multiple events": {
			schedules: []*cron.SpecSchedule{
				everyOtherDay,
				monday,
				tuesday,
				wednesday,
				friday,
			},
			now:                      now,
			direction:                directionPrevious,
			expectedNearestEventTime: midnightOnWednesday,
			expectedIdx:              3,
		},
		"[Next] Multiple events": {
			schedules: []*cron.SpecSchedule{
				everyOtherDay,
				monday,
				tuesday,
				wednesday,
				friday,
			},
			now:                      now,
			direction:                directionNext,
			expectedNearestEventTime: midnightOnThursday,
			expectedIdx:              0,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actualTime, actualIdx := findNearestScheduleTime(tc.schedules, tc.now, tc.direction, tc.additionalOffsetSeconds)

			assert.Equal(t, tc.expectedNearestEventTime, actualTime, "%v (expected) != %v (actual)", tc.expectedNearestEventTime, actualTime)
			assert.Equal(t, tc.expectedIdx, actualIdx)
		})
	}
}

func newCronSchedule(spec string) *cron.SpecSchedule {
	s, _ := cron.ParseStandard(spec)
	return s.(*cron.SpecSchedule)
}

func Test_findNearestEvent_filtering(t *testing.T) {
	schedules := []batch.EventSpec{
		{
			Action:       batch.EventTypeStart,
			CronSchedule: "0 0 * * Mon",
		},
		{
			Action:       batch.EventTypeStop,
			CronSchedule: "0 0 * * Tue",
		},
	}

	// Now is midday on Wednesday
	now := time.Date(2022, 02, 02, 12, 0, 0, 0, time.UTC)

	actualPrevious, _ := findNearestEvent(schedules, now, locationWithOffset{time.UTC, 0}, directionPrevious,
		func(es batch.EventSpec) bool { return es.Action == batch.EventTypeStart },
	)
	actualNext, _ := findNearestEvent(schedules, now, locationWithOffset{time.UTC, 0}, directionNext,
		func(es batch.EventSpec) bool { return es.Action == batch.EventTypeStart },
	)

	// expect the nearest previous event to be midnight on Monday 31st January
	// i.e. we skip over the stop event on Tuesday
	expectedPrevious := &ScheduledEvent{
		ScheduledTimeUTC: time.Date(2022, 01, 31, 0, 0, 0, 0, time.UTC),
		Type:             batch.EventTypeStart,
	}

	// expect the nearest previous event to be midnight on Monday 7th February
	// i.e. we don't skip over any events, because the start event is the next one
	expectedNext := &ScheduledEvent{
		ScheduledTimeUTC: time.Date(2022, 02, 7, 0, 0, 0, 0, time.UTC),
		Type:             batch.EventTypeStart,
	}

	testhelpers.AssertDeepEqualJson(t, expectedPrevious, actualPrevious, "Previous event should match")
	testhelpers.AssertDeepEqualJson(t, expectedNext, actualNext, "Next event should match")

}
