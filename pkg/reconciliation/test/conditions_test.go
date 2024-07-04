package reconciletests

import (
	"testing"
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/G-Research/controlled-job/pkg/metadata"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
)

func Test_Conditions(t *testing.T) {
	var beforeStart = time.Date(2022, time.December, 12, 8, 30, 0, 0, time.UTC)
	var startDaily = "09:00"
	var betweenStartAndStop = time.Date(2022, time.December, 12, 12, 0, 0, 0, time.UTC)
	var stopDaily = "17:00"

	// Test all combinations of job state (no job, job starting up, job running, job completed, job failed)
	// and running schedule (inside/outside of schedule) and make sure the correct conditions are set

	Run(t, "With no job", func(tc *testContext) {
		tc.Run("When outside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.WhenReconcileIsRunAt(beforeStart)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
		tc.Run("When inside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.WhenReconcileIsRunAt(betweenStartAndStop)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionTrue)
		})
	})
	Run(t, "With job that's starting up", func(tc *testContext) {
		tc.Run("When outside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(0),
			)

			tc.WhenReconcileIsRunAt(beforeStart)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
		tc.Run("When inside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(0),
			)

			tc.WhenReconcileIsRunAt(betweenStartAndStop)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionTrue)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
	})
	Run(t, "With job that's running", func(tc *testContext) {
		tc.Run("When outside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(1),
			)

			tc.WhenReconcileIsRunAt(beforeStart)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
		tc.Run("When inside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(1),
			)

			tc.WhenReconcileIsRunAt(betweenStartAndStop)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionTrue)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
	})
	Run(t, "With job that's completed successfully", func(tc *testContext) {
		tc.Run("When outside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(1),
				WithCondition(kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				}),
			)

			tc.WhenReconcileIsRunAt(beforeStart)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
		tc.Run("When inside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(1),
				WithCondition(kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				}),
			)

			tc.WhenReconcileIsRunAt(betweenStartAndStop)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionTrue)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
	})
	Run(t, "With job that's failed", func(tc *testContext) {
		tc.Run("When outside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(1),
				WithCondition(kbatch.JobCondition{
					Type:   kbatch.JobFailed,
					Status: corev1.ConditionTrue,
				}),
			)

			tc.WhenReconcileIsRunAt(beforeStart)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown)
		})
		tc.Run("When inside of schedule", func(tc *testContext) {
			tc.GivenAControlledJob(
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
				WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
			)

			tc.GivenAnExistingJob(
				metadata.WithJobRunIdx(0),
				WithActiveCount(1),
				WithReadyCount(1),
				WithCondition(kbatch.JobCondition{
					Type:   kbatch.JobFailed,
					Status: corev1.ConditionTrue,
				}),
			)

			tc.WhenReconcileIsRunAt(betweenStartAndStop)

			tc.ShouldHaveCondition(v1.ConditionTypeShouldBeRunning, metav1.ConditionTrue)

			tc.ShouldHaveCondition(v1.ConditionTypeJobExists, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobRunning, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobComplete, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeJobFailed, metav1.ConditionTrue)
			tc.ShouldHaveCondition(v1.ConditionTypeJobSuspended, metav1.ConditionFalse)
			tc.ShouldHaveCondition(v1.ConditionTypeJobBeingDeleted, metav1.ConditionFalse)

			tc.ShouldHaveCondition(v1.ConditionTypeRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown)
			tc.ShouldHaveCondition(v1.ConditionTypeNotRunningUnexpectedly, metav1.ConditionTrue)
		})
	})

	Run(t, "should not update condition transition time if no change", func(tc *testContext) {
		tc.GivenAControlledJob(
			WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily),
			WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily),
		)

		tc.WhenReconcileIsRunAt(betweenStartAndStop)

		initialStatus := tc.currentReconcileRun.status.DeepCopy()

		// Ensure that we would definitiely see a new timestamp in the status if the code is wrong
		time.Sleep(time.Second * 2)

		tc.WhenReconcileIsRunAt(betweenStartAndStop.Add(time.Second * 2))

		finalStatus := tc.currentReconcileRun.status.DeepCopy()

		AssertDeepEqualJson(tc, initialStatus, finalStatus, "Status should not have changed if conditions have not changed")
	})
}
