package reconciletests

import (
	"testing"
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
)

var torontoLoc, _ = time.LoadLocation("America/Toronto")

func Test_TimezoneSupport(t *testing.T) {
	Run(t, "Toronto timezone with no additional offset", func(tc *testContext) {

		tc.GivenAControlledJob(
			WithTimezone("America/Toronto", 0),
			WithScheduledEvent(v1.EventTypeStart, "Mon-Fri", "09:00"),
			WithScheduledEvent(v1.EventTypeStop, "Mon-Fri", "17:00"),
			WithDefaultJobTemplate(),
		)
		// 2022-04-22 13:01 (UTC) = 2022-04-22 09:01 (America/Toronto)
		tc.WhenReconcileIsRunAt(time.Date(2022, time.April, 22, 13, 1, 0, 0, time.UTC))

		// Wake again at 5pm Toronto time (stop time
		tc.ShouldHaveBeenRequeuedAt(time.Date(2022, time.April, 22, 17, 0, 0, 0, torontoLoc))

		// We're inside the run period in Toronto, so a job should have been created, with the correct time stamp (in UTC)
		tc.ShouldHaveCreatedAJob(
			WithExpectedScheduledTime(time.Date(2022, time.April, 22, 13, 0, 0, 0, time.UTC)))
	})

	Run(t, "Toronto timezone with additional offset", func(tc *testContext) {

		// start time of 09:00 in Toronto-2m means start at 13:02 in UTC
		tc.GivenAControlledJob(
			WithTimezone("America/Toronto", -120),
			WithScheduledEvent(v1.EventTypeStart, "Mon-Fri", "09:00"),
			WithScheduledEvent(v1.EventTypeStop, "Mon-Fri", "17:00"),
			WithDefaultJobTemplate(),
		)
		// 2022-04-22 13:01 (UTC)
		//   = 2022-04-22 09:01 (America/Toronto)
		//   = 2022-04-22 08:59 (America/Toronto - 120s)
		// which is outside of the run period of the controller job
		tc.WhenReconcileIsRunAt(time.Date(2022, time.April, 22, 13, 1, 0, 0, time.UTC))
		// Wake again at 9am Toronto(-120s) time == 13:02 UTC
		tc.ShouldHaveBeenRequeuedAt(time.Date(2022, time.April, 22, 13, 2, 0, 0, time.UTC))

		tc.ShouldNotHaveCreatedAJob()
	})
}
