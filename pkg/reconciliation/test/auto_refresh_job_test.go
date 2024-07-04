package reconciletests

import (
	"testing"
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/reconciliation"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/G-Research/controlled-job/pkg/metadata"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
)

func Test_AutoRestart(t *testing.T) {

	// Make sure the feature is enabled (should be enabled by default everywhere now)
	reconciliation.Options.EnableAutoRecreateJobsOnSpecChange = true

	var now = time.Date(2022, time.December, 12, 12, 12, 0, 0, time.UTC)
	var inThePast = now.Add(-time.Hour)
	var inTheFuture = now.Add(time.Hour)

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
		WithScheduledEventAtTime(v1.EventTypeStart, inThePast)(cj)
		WithScheduledEventAtTime(v1.EventTypeStop, inTheFuture)(cj)
	}

	// Some helpers to setup a job with either out of date or up to date specs
	var withOutOfDateSpec = metadata.WithControlledJobMetadata("basic-job", "1234", inThePast, 1, oldJobTemplate)
	var withUpToDateSpec = metadata.WithControlledJobMetadata("basic-job", "1234", inThePast, 1, newJobTemplate)

	Run(t, "With policy set to RecreateSpecChangePolicy", func(tc *testContext) {

		// Every test here uses the same controlled job setup
		var withRecreateEnabled = WithSpecChangePolicy(v1.RecreateSpecChangePolicy)

		tc.Run("Active job with up to date spec is not replaced", func(tc *testContext) {

			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateEnabled)
			tc.GivenAnExistingJob(
				WithActiveCount(1),
				withUpToDateSpec)
			tc.WhenReconcileIsRunAt(now)

			// Should not have deleted the old job or create a new one
			tc.ShouldNotHaveDeletedAJob()
			tc.ShouldNotHaveCreatedAJob()
		})

		tc.Run("Active job with out of date spec is replaced", func(tc *testContext) {

			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateEnabled)
			tc.GivenAnExistingJob(
				WithJobName("basic-job-1"),
				WithActiveCount(1),
				withOutOfDateSpec)
			tc.WhenReconcileIsRunAt(now)

			// Should create a job with index 2 and with the suspended flag set
			tc.ShouldHaveCreatedAJob(
				WithExpectedJobIndex(2),
				ThatShouldBeSuspended(),
				WithJobSpecMatching(newJobTemplate))

			// Should have deleted the old job
			tc.ShouldHaveDeletedAJob(
				WithExpectedJobName("basic-job-1"),
			)

		})

		tc.Run("Completed job with out of date spec is not replaced", func(tc *testContext) {

			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateEnabled)
			tc.GivenAnExistingJob(
				HasSucceeded(),
				withOutOfDateSpec)
			tc.WhenReconcileIsRunAt(now)

			// Should not have deleted the old job or create a new one
			tc.ShouldNotHaveDeletedAJob()
			tc.ShouldNotHaveCreatedAJob()
		})

		tc.Run("Job being deleted with out of date spec is not replaced", func(tc *testContext) {
			// This is a bit of a subtle edge case. If the job is being deleted, but has an out of date spec, then we
			// should _not_ recreate it, because the most likely situation is that the user has issued a stop request
			// not a restart request, and so would be surprised if the job then started back up underneath them.

			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateEnabled)
			tc.GivenAnExistingJob(
				IsBeingDeleted(),
				withOutOfDateSpec)
			tc.WhenReconcileIsRunAt(now)

			// Should not have deleted the old job or create a new one
			tc.ShouldNotHaveDeletedAJob()
			tc.ShouldNotHaveCreatedAJob()
		})
	})

	Run(t, "With policy set to IgnoreSpecChangePolicy", func(tc *testContext) {

		// Every test here uses the same controlled job setup
		var withRecreateDisabled = WithSpecChangePolicy(v1.IgnoreSpecChangePolicy)

		tc.Run("Active job with up to date spec is not replaced", func(tc *testContext) {

			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateDisabled)
			tc.GivenAnExistingJob(
				WithActiveCount(1),
				withUpToDateSpec)
			tc.WhenReconcileIsRunAt(now)

			// Should not have deleted the old job or create a new one
			tc.ShouldNotHaveDeletedAJob()
			tc.ShouldNotHaveCreatedAJob()
		})

		tc.Run("Active job with out of date spec is not replaced", func(tc *testContext) {

			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateDisabled)
			tc.GivenAnExistingJob(
				WithJobName("basic-job-1"),
				WithActiveCount(1),
				withOutOfDateSpec)
			tc.WhenReconcileIsRunAt(now)

			// Should not have deleted the old job or create a new one
			tc.ShouldNotHaveDeletedAJob()
			tc.ShouldNotHaveCreatedAJob()
		})

		tc.Run("Inactive job with out of date spec is not replaced", func(tc *testContext) {

			tc.GivenAControlledJob(
				withNewSpec,
				withRecreateDisabled)
			tc.GivenAnExistingJob(
				WithActiveCount(0),
				withOutOfDateSpec)
			tc.WhenReconcileIsRunAt(now)

			// Should not have deleted the old job or create a new one
			tc.ShouldNotHaveDeletedAJob()
			tc.ShouldNotHaveCreatedAJob()
		})
	})
}
