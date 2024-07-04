package reconciletests

import (
	"testing"
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/metadata"

	. "github.com/G-Research/controlled-job/pkg/testhelpers"
)

func Test_ManuallyStartedJobs(t *testing.T) {
	startedOnMonday := time.Date(2022, time.July, 25, 0, 0, 0, 0, time.UTC)
	monday9am := time.Date(2022, time.July, 25, 9, 0, 0, 0, time.UTC)
	tuesdayMidnight := time.Date(2022, time.July, 26, 0, 0, 0, 0, time.UTC)

	Run(t, "No schedule", func(tc *testContext) {
		tc.Run("job without manual annotation gets deleted", func(tc *testContext) {
			tc.GivenAControlledJob(WithDefaultJobTemplate())
			tc.GivenAnExistingJob(
				WithJobName("my-job-1"),
				metadata.WithControlledJobAnnotations(startedOnMonday, 1, false, DefaultJobTemplate()),
			)

			tc.WhenReconcileIsRunAt(monday9am)

			tc.ShouldHaveDeletedAJob(WithExpectedJobName("my-job-1"))
		})

		tc.Run("job with manual annotation does not get deleted", func(tc *testContext) {
			tc.GivenAControlledJob(WithDefaultJobTemplate())
			tc.GivenAnExistingJob(
				WithJobName("my-job-1"),
				metadata.WithControlledJobAnnotations(startedOnMonday, 1, true, DefaultJobTemplate()),
			)

			tc.WhenReconcileIsRunAt(monday9am)

			tc.ShouldNotHaveDeletedAJob()
		})
	})

	Run(t, "No start schedule", func(tc *testContext) {
		tc.Run("job without manual annotation gets deleted", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithDefaultJobTemplate(),
				WithScheduledEvent(v1.EventTypeStop, "Mon-Fri", "17:00"),
			)
			tc.GivenAnExistingJob(
				WithJobName("my-job-1"),
				metadata.WithControlledJobAnnotations(startedOnMonday, 1, false, DefaultJobTemplate()),
			)

			tc.WhenReconcileIsRunAt(monday9am)

			tc.ShouldHaveDeletedAJob(WithExpectedJobName("my-job-1"))
		})

		tc.Run("job with manual annotation does not get deleted", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithDefaultJobTemplate(),
				WithScheduledEvent(v1.EventTypeStop, "Mon-Fri", "17:00"),
			)
			tc.GivenAnExistingJob(
				WithJobName("my-job-1"),
				metadata.WithControlledJobAnnotations(startedOnMonday, 1, true, DefaultJobTemplate()),
			)

			tc.WhenReconcileIsRunAt(monday9am)

			tc.ShouldNotHaveDeletedAJob()
		})

		tc.Run("job with manual annotation, started before last stop time _does_ get deleted", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithDefaultJobTemplate(),
				WithScheduledEvent(v1.EventTypeStop, "Mon-Fri", "17:00"),
			)
			tc.GivenAnExistingJob(
				WithJobName("my-job-1"),
				metadata.WithControlledJobAnnotations(startedOnMonday, 1, true, DefaultJobTemplate()),
			)

			tc.WhenReconcileIsRunAt(tuesdayMidnight)

			tc.ShouldHaveDeletedAJob(WithExpectedJobName("my-job-1"))
		})
	})
}
