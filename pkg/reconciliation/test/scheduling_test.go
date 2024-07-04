package reconciletests

import (
	"testing"
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/metadata"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Scheduling(t *testing.T) {
	var earlyMorning = time.Date(2022, time.December, 12, 7, 12, 0, 0, time.UTC)
	var startTime = time.Date(2022, time.December, 12, 9, 0, 0, 0, time.UTC)
	//var midday = time.Date(2022, time.December, 12, 12, 12, 0, 0, time.UTC)
	//var lateEvening = time.Date(2022, time.December, 12, 22, 12, 0, 0, time.UTC)
	var startDaily = "09:00"
	var stopDaily = "17:00"
	//var startTimeToday = time.Date(2022, time.December, 12, 9, 0, 0, 0, time.UTC)
	//var stopTimeToday = time.Date(2022, time.December, 12, 17, 0, 0, 0, time.UTC)
	//
	var beforeLastStopTime = time.Date(2022, time.December, 11, 12, 12, 0, 0, time.UTC)
	var afterLastStopTime = time.Date(2022, time.December, 11, 19, 12, 0, 0, time.UTC)

	var givenControlledJobWithSchedule = func(tc *testContext, opts ...ControlledJobOption) {
		opts = append([]ControlledJobOption{
			WithControlledJobName("schedule-test"),
			WithDefaultJobTemplate(),
			WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
			WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
		}, opts...)
		tc.GivenAControlledJob(
			opts...,
		)
	}

	var jobWithStartTime = func(startTime time.Time, jobIdx int, manuallyScheduled bool) JobOption {
		return metadata.WithControlledJobAnnotations(startTime, jobIdx, manuallyScheduled, DefaultJobTemplate())
	}

	Run(t, "before start time", func(tc *testContext) {
		tc.Run("when there are no jobs - takes no action", func(tc *testContext) {
			givenControlledJobWithSchedule(tc)

			tc.WhenReconcileIsRunAt(earlyMorning)

			tc.ShouldNotHaveCreatedAJob()
			tc.ShouldNotHaveDeletedAJob()
		})

		tc.Run("when there are any existing jobs - deletes the non-manual ones", func(tc *testContext) {
			givenControlledJobWithSchedule(tc)

			tc.GivenExistingJobs(
				NewJob("previous-run-0", jobWithStartTime(beforeLastStopTime, 0, false)),
				NewJob("between-run-1", jobWithStartTime(afterLastStopTime, 1, false)),
				NewJob("manual-2", jobWithStartTime(afterLastStopTime, 1, true)),
			)

			tc.WhenReconcileIsRunAt(earlyMorning)

			tc.ShouldNotHaveCreatedAJob()
			tc.ShouldHaveDeletedAJobMatching(WithExpectedJobName("previous-run-0"))
			tc.ShouldHaveDeletedAJobMatching(WithExpectedJobName("between-run-1"))
			tc.ShouldNotHaveDeletedAJobMatching(WithExpectedJobName("manual-2"))
		})
	})

	Run(t, "at start time", func(tc *testContext) {
		tc.Run("when there are no jobs - creates a job unsuspended with the correct settings", func(tc *testContext) {
			givenControlledJobWithSchedule(tc)

			tc.WhenReconcileIsRunAt(startTime)

			tc.ShouldHaveCreatedAJob(
				WithExpectedJobIndex(0),
				WithExpectedSuspendedFlag(nil),
				WithJobSpecMatching(DefaultJobTemplate()),
				WithExpectedScheduledTime(startTime),
				WithExpectedControlledJobOwner(tc.controlledJob.Name),
			)
		})
	})

	Run(t, "with start deadline", func(tc *testContext) {
		tc.Run("when within start deadline", func(tc *testContext) {
			givenControlledJobWithSchedule(tc,
				WithStartingDeadlineSeconds(30),
			)
			tc.WhenReconcileIsRunAt(startTime.Add(time.Second * 15))

			tc.ShouldHaveCreatedAJob()
			tc.ShouldHaveCondition(v1.ConditionTypeStartingDeadlineExceeded, metav1.ConditionFalse)
		})
	})
}
