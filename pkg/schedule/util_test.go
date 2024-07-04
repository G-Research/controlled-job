package schedule

import (
	"errors"
	"testing"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/testhelpers"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
)

func Test_mapEventToSpecSchedule(t *testing.T) {
	londonTimezone, _ := time.LoadLocation("Europe/London")

	s, _ := cron.ParseStandard("5 * * * ?")
	fiveMinsPastEveryHour := s.(*cron.SpecSchedule)

	testCases := map[string]struct {
		event            batch.EventSpec
		location         *time.Location
		expectedSchedule *cron.SpecSchedule
		expectedError    error
	}{
		"Invalid spec": {
			event: batch.EventSpec{
				Schedule: &batch.FriendlyScheduleSpec{
					TimeOfDay:  "INVALID",
					DaysOfWeek: "FOO",
				},
			},
			expectedError: errors.New("timeOfDay must be in the format hh:mm"),
		},
		"Unparseable cron spec": {
			event: batch.EventSpec{
				CronSchedule: "A B C",
			},
			expectedError: errors.New("Failed to parse cron schedule: expected exactly 5 fields, found 3: [A B C]"),
		},
		"Overrides any timezone specified in the cron spec itself": {
			event: batch.EventSpec{
				CronSchedule: "TZ=America/New_York 5 * * * ?",
			},
			location: londonTimezone,
			expectedSchedule: &cron.SpecSchedule{
				Location: londonTimezone,
				Second:   fiveMinsPastEveryHour.Second,
				Minute:   fiveMinsPastEveryHour.Minute,
				Hour:     fiveMinsPastEveryHour.Hour,
				Dom:      fiveMinsPastEveryHour.Dom,
				Month:    fiveMinsPastEveryHour.Month,
				Dow:      fiveMinsPastEveryHour.Dow,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actualSched, actualErr := mapEventToSpecSchedule(tc.event, tc.location)

			testhelpers.AssertDeepEqualJson(t, tc.expectedSchedule, actualSched)
			testhelpers.AssertSameError(t, tc.expectedError, actualErr)
		})
	}
}

func Test_eventIsNearer(t *testing.T) {

	testCases := map[string]struct {
		testTime      time.Time
		referenceTime time.Time
		direction     eventDirection
		expected      bool
	}{
		"[Next] Every time is nearer then the zero time": {
			testTime:      time.Now(),
			referenceTime: time.Time{},
			direction:     directionNext,
			expected:      true,
		},
		"[Previous] Every time is nearer then the zero time": {
			testTime:      time.Now(),
			referenceTime: time.Time{},
			direction:     directionPrevious,
			expected:      true,
		},
		"[Next] time to test is before reference time": {
			testTime:      time.Now().Add(-1 * time.Hour),
			referenceTime: time.Now(),
			direction:     directionNext,
			expected:      true,
		},
		"[Next] time to test is after reference time": {
			testTime:      time.Now().Add(1 * time.Hour),
			referenceTime: time.Now(),
			direction:     directionNext,
			expected:      false,
		},
		"[Previous] time to test is before reference time": {
			testTime:      time.Now().Add(-1 * time.Hour),
			referenceTime: time.Now(),
			direction:     directionPrevious,
			expected:      false,
		},
		"[Previous] time to test is after reference time": {
			testTime:      time.Now().Add(1 * time.Hour),
			referenceTime: time.Now(),
			direction:     directionPrevious,
			expected:      true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := eventIsNearer(tc.testTime, tc.referenceTime, tc.direction)

			assert.Equal(t, tc.expected, actual)
		})
	}
}
