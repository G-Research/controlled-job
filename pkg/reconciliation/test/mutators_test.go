package reconciletests

import (
	"errors"
	"testing"
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/metadata"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
)

func Test_Mutators(t *testing.T) {
	var startTime = time.Date(2022, time.December, 12, 9, 0, 0, 0, time.UTC)

	var startDaily = "09:00"
	var stopDaily = "17:00"

	var givenControlledJobWithSchedule = func(tc *testContext) {
		tc.GivenAControlledJob(
			WithControlledJobName("mutators-test"),
			WithAnnotation(metadata.ApplyMutationsAnnotation, "true"),
			WithJobTemplate(DefaultJobTemplate()),
			WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
			WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
		)
	}

	Run(t, "at start time", func(tc *testContext) {
		tc.Run("when mutator succeeds - creates a job with mutation", func(tc *testContext) {
			givenControlledJobWithSchedule(tc)
			tc.WithTestMutator("mutated-image", nil)

			tc.WhenReconcileIsRunAt(startTime)

			tc.ShouldHaveCreatedAJob(
				WithExpectedJobIndex(0),
				WithExpectedSuspendedFlag(nil),
				WithJobSpecMatching(WithImageMutation(DefaultJobTemplate(), "mutated-image")),
				WithExpectedScheduledTime(startTime),
				WithExpectedControlledJobOwner(tc.controlledJob.Name),
			)
			tc.ShouldHaveCalledMutator()
		})

		tc.Run("when mutator fails - fails to create job", func(tc *testContext) {
			givenControlledJobWithSchedule(tc)
			tc.WithTestMutator("", errors.New("super fatal error"))

			tc.WhenReconcileIsRunAt(startTime)

			tc.ShouldNotHaveCreatedAJob()
			tc.ShouldHaveCalledMutator()
		})
	})
}
