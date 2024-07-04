package metadata

import (
	"fmt"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/testhelpers"
	kbatch "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func WithControlledJobAnnotations(scheduledTime time.Time, jobIdx int, manuallyScheduled bool, jobTemplate batchv1beta1.JobTemplateSpec) testhelpers.JobOption {
	return func(job *kbatch.Job) {
		job.Annotations[ScheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
		job.Annotations[JobRunIdAnnotation] = fmt.Sprintf("%d", jobIdx)
		if manuallyScheduled {
			job.Annotations[ManualJobAnnotation] = "true"
		}
		job.Annotations[TemplateHashAnnotation] = CalculateHashFor(jobTemplate)
	}
}

func WithControlledJobMetadata(name string, uid types.UID, scheduledTime time.Time, jobIdx int, jobTemplate batchv1beta1.JobTemplateSpec) testhelpers.JobOption {
	return func(job *kbatch.Job) {
		WithControlledJobAnnotations(scheduledTime, jobIdx, false, jobTemplate)(job)
		job.Labels[ControlledJobLabel] = name
		job.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion:         batch.GroupVersion.Identifier(),
				Kind:               "ControlledJob",
				Name:               name,
				UID:                uid,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			},
		}
	}
}

func WithScheduledTimeAnnotation(scheduledTime time.Time) testhelpers.JobOption {
	return func(job *kbatch.Job) {
		job.Annotations[ScheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
	}
}

func WithJobRunIdx(idx int) testhelpers.JobOption {
	return func(job *kbatch.Job) {
		job.Annotations[JobRunIdAnnotation] = fmt.Sprintf("%d", idx)
	}
}
