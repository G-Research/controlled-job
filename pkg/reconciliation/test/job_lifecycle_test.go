package reconciletests

import (
	"testing"
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/metadata"
	"github.com/G-Research/controlled-job/pkg/reconciliation"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_JobLifecycle(t *testing.T) {
	// Make sure the feature is enabled (should be enabled by default everywhere now)
	reconciliation.Options.EnableAutoRecreateJobsOnSpecChange = true

	var now = time.Date(2022, time.December, 12, 12, 12, 0, 0, time.UTC)
	var startDaily = "09:00"
	var stopDaily = "17:00"
	var startTimeToday = time.Date(2022, time.December, 12, 9, 0, 0, 0, time.UTC)
	//var inTheFuture = now.Add(time.Hour)

	var beforeLastStopTime = time.Date(2022, time.December, 11, 12, 12, 0, 0, time.UTC)

	var oldJobTemplate = v1beta1.JobTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"version": "old"},
		},
		Spec: kbatch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container", Image: "alpine:1.1"}},
				},
			},
		},
	}
	var newJobTemplate = v1beta1.JobTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"version": "new"},
		},
		Spec: kbatch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "container", Image: "alpine:2.2"}},
				},
			},
		},
	}

	// Common helper to setup the controlled job in the common way for all of these tests
	var withNewSpec ControlledJobOption = func(cj *v1.ControlledJob) {
		WithControlledJobName("basic-job")(cj)
		// The controlled job has the new job template
		WithJobTemplate(newJobTemplate)(cj)
		WithScheduledEventAtTimeEveryDay(v1.EventTypeStart, startDaily)(cj)
		WithScheduledEventAtTimeEveryDay(v1.EventTypeStop, stopDaily)(cj)
	}
	var withRecreateEnabled = WithSpecChangePolicy(v1.RecreateSpecChangePolicy)

	var upToDateJob = func(jobIdx int) JobOption {
		return func(job *kbatch.Job) {
			metadata.WithControlledJobMetadata("basic-job", "1234", startTimeToday, jobIdx, newJobTemplate)(job)
		}
	}
	var outOfDateJob = func(jobIdx int) JobOption {
		return func(job *kbatch.Job) {
			metadata.WithControlledJobMetadata("basic-job", "1234", startTimeToday, jobIdx, oldJobTemplate)(job)
		}
	}

	Run(t, "one existing suspended job", func(tc *testContext) {

		tc.Run("has up to date spec - gets unsuspended immediately", func(tc *testContext) {
			tc.GivenAControlledJob(withNewSpec)

			tc.GivenExistingJobs(
				NewJob("basic-job-0",
					IsSuspended(true),
					metadata.WithControlledJobMetadata("basic-job", "1234", startTimeToday, 0, newJobTemplate),
				),
			)

			tc.WhenReconcileIsRunAt(now)

			tc.ShouldHaveUnsuspendedAJob(WithExpectedJobName("basic-job-0"))
		})

		tc.Run("has out of date spec, recreate is not enabled - job is unsuspended", func(tc *testContext) {
			tc.GivenAControlledJob(withNewSpec)

			tc.GivenExistingJobs(
				NewJob("basic-job-0",
					IsSuspended(true),
					metadata.WithControlledJobMetadata("basic-job", "1234", startTimeToday, 0, oldJobTemplate),
				),
			)

			tc.WhenReconcileIsRunAt(now)

			tc.ShouldHaveUnsuspendedAJob(WithExpectedJobName("basic-job-0"))
			tc.ShouldNotHaveCreatedAJob()
		})

		tc.Run("has out of date spec, recreate is enabled - new job is created, but is not unsuspended", func(tc *testContext) {
			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateEnabled)

			tc.GivenExistingJobs(
				NewJob("basic-job-0",
					IsSuspended(true),
					metadata.WithControlledJobMetadata("basic-job", "1234", startTimeToday, 0, oldJobTemplate),
				),
			)

			tc.WhenReconcileIsRunAt(now)

			tc.ShouldNotHaveUnsuspendedAJob()
			tc.ShouldHaveCreatedAJob(WithExpectedJobIndex(1), ThatShouldBeSuspended())
			tc.ShouldHaveDeletedAJob(WithExpectedJobIndex(0))
		})
	})

	Run(t, "one existing job being deleted", func(tc *testContext) {
		tc.Run("is not deleted again", func(tc *testContext) {
			// We'll get noisy errors in the logs if we try to delete a Job that is already being deleted
			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateEnabled)

			tc.GivenExistingJobs(
				NewJob("basic-job-0",
					IsBeingDeleted(),
					metadata.WithControlledJobMetadata("basic-job", "1234", startTimeToday, 0, newJobTemplate),
				),
			)

			tc.WhenReconcileIsRunAt(now)

			tc.ShouldNotHaveDeletedAJob()
		})
	})

	Run(t, "one existing job that the user suspended", func(tc *testContext) {
		tc.Run("is not unsuspended", func(tc *testContext) {
			// We'll get noisy errors in the logs if we try to delete a Job that is already being deleted
			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateEnabled)

			tc.GivenExistingJobs(
				NewJob("basic-job-0",
					WithJobAnnotation(metadata.SuspendReason, "user-stop"),
					metadata.WithControlledJobMetadata("basic-job", "1234", startTimeToday, 0, newJobTemplate),
				),
			)

			tc.WhenReconcileIsRunAt(now)

			tc.ShouldNotHaveUnsuspendedAJob()
			tc.ShouldNotHaveDeletedAJob()
		})
	})

	Run(t, "multiple existing jobs", func(tc *testContext) {

		tc.Run("expired jobs are always deleted", func(tc *testContext) {
			tc.GivenAControlledJob(withNewSpec)
			tc.GivenExistingJobs(
				NewJob("basic-job-0",
					IsSuspended(true),
					metadata.WithControlledJobMetadata("basic-job", "1234", beforeLastStopTime, 0, oldJobTemplate),
				),
				NewJob("basic-job-1",
					IsSuspended(false),
					metadata.WithControlledJobMetadata("basic-job", "1234", beforeLastStopTime, 1, oldJobTemplate),
				),
				NewJob("basic-job-2",
					IsSuspended(false),
					HasSucceeded(),
					metadata.WithControlledJobMetadata("basic-job", "1234", beforeLastStopTime, 2, oldJobTemplate),
				),
			)

			tc.WhenReconcileIsRunAt(now)

			tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(0))
			tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(1))
			tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(2))
		})
		tc.Run("chosen job selection", func(tc *testContext) {
			// When there are multiple jobs, we delete all but one of them. So we need to make sure we choose the
			// one to keep carefully, so that:
			// - we don't delete the main running job
			// - we don't delete the job with the most up to date spec
			// - we don't flip between spawning and deleting jobs

			tc.Run("prefer jobs not being deleted to one's being deleted", func(tc *testContext) {
				tc.GivenAControlledJob(withNewSpec)

				tc.GivenExistingJobs(
					NewJob("job-0", upToDateJob(0)),
					NewJob("job-1", upToDateJob(1), IsBeingDeleted()),
				)

				tc.WhenReconcileIsRunAt(now)

				tc.ShouldNotHaveDeletedAJobMatching(WithExpectedJobIndex(0))
				// No need to delete a job that's already being deleted
				//tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(1))
			})

			tc.Run("prefer jobs with up to date spec over out of date spec", func(tc *testContext) {
				tc.GivenAControlledJob(withNewSpec)

				tc.GivenExistingJobs(
					NewJob("job-0", upToDateJob(0)),
					NewJob("job-1", outOfDateJob(1)),
				)

				tc.WhenReconcileIsRunAt(now)

				tc.ShouldNotHaveDeletedAJobMatching(WithExpectedJobIndex(0))
				tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(1))
			})

			tc.Run("an out of date job that's not being deleted is better than an up to date job that's being deleted", func(tc *testContext) {
				tc.GivenAControlledJob(withNewSpec)

				tc.GivenExistingJobs(
					NewJob("job-0", outOfDateJob(0)),
					NewJob("job-1", upToDateJob(1), IsBeingDeleted()),
				)

				tc.WhenReconcileIsRunAt(now)

				tc.ShouldNotHaveDeletedAJobMatching(WithExpectedJobIndex(0))
				// No need to delete a job that's already being deleted
				//tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(1))
			})

			tc.Run("when lots of jobs are equally good, the highest named one wins", func(tc *testContext) {
				tc.GivenAControlledJob(withNewSpec)

				tc.GivenExistingJobs(
					NewJob("job-0", outOfDateJob(0)),
					NewJob("job-1", upToDateJob(1), IsBeingDeleted()),
					// These next ones are all just as good as each other
					NewJob("job-2", upToDateJob(2)),
					NewJob("job-3", upToDateJob(3)),
					NewJob("job-4", upToDateJob(4)), // job 4 should win
					// This last one is no good
					NewJob("job-5", outOfDateJob(5)),
				)

				tc.WhenReconcileIsRunAt(now)

				tc.ShouldNotHaveDeletedAJobMatching(WithExpectedJobIndex(4))
				// everything else should be deleted
				tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(0))
				// No need to delete a job that's already being deleted
				//tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(1))
				tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(2))
				tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(3))
				tc.ShouldHaveDeletedAJobMatching(WithExpectedJobIndex(5))
			})

		})

	})

}
