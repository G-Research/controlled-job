package job

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/G-Research/controlled-job/pkg/mutators"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/k8s"
	"github.com/G-Research/controlled-job/pkg/metadata"
	kbatch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

/*
We need to construct a job based on our ControlledJob's template.  We'll copy over the spec
from the template and copy some basic object meta.
Then, we'll set the "scheduled time" annotation so that we can reconstitute our
`LastScheduleTime` field each reconcile.
Finally, we'll need to set an owner reference.  This allows the Kubernetes garbage collector
to clean up jobs when we delete the ControlledJob, and allows controller-runtime to figure out
which controlledjob needs to be reconciled when a given job changes (is added, deleted, completes, etc).
*/
func BuildForControlledJob(ctx context.Context, controlledJob *batch.ControlledJob, scheduledTime time.Time, jobRunIdx int, isManuallyScheduled, startSuspended bool) (*kbatch.Job, error) {
	return buildJob(ctx, controlledJob, scheduledTime, jobRunIdx, isManuallyScheduled, startSuspended)
}

func RecreateJobWithNewSpec(ctx context.Context, existingJob *kbatch.Job, controlledJob *batch.ControlledJob, jobRunIdx int, startSuspended bool) (*kbatch.Job, error) {
	wasManuallyScheduled := metadata.IsManuallyScheduledJob(existingJob)

	oldScheduledTime, err := metadata.GetScheduledTime(existingJob)
	if err != nil {
		return nil, err
	}
	return buildJob(ctx, controlledJob, oldScheduledTime, jobRunIdx, wasManuallyScheduled, startSuspended)
}

func buildJob(ctx context.Context, controlledJob *batch.ControlledJob, scheduledTime time.Time, jobRunId int, isManuallyScheduled, startSuspended bool) (*kbatch.Job, error) {
	// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
	name := metadata.JobName(controlledJob.Name, scheduledTime, jobRunId)

	typeMeta := metav1.TypeMeta{}
	typeMeta.SetGroupVersionKind(kbatch.SchemeGroupVersion.WithKind("Job"))

	job := &kbatch.Job{
		TypeMeta: typeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   controlledJob.Namespace,
		},
		Spec: *controlledJob.Spec.JobTemplate.Spec.DeepCopy(),
	}
	for k, v := range controlledJob.Spec.JobTemplate.Annotations {
		job.Annotations[k] = v
	}
	job.Annotations[metadata.ScheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
	job.Annotations[metadata.JobRunIdAnnotation] = fmt.Sprintf("%d", jobRunId)
	job.Annotations[metadata.TemplateHashAnnotation] = metadata.CalculateHashFor(controlledJob.Spec.JobTemplate)
	if len(controlledJob.Spec.Timezone.Name) > 0 {
		job.Annotations[metadata.TimeZoneAnnotation] = controlledJob.Spec.Timezone.Name
	}
	if controlledJob.Spec.Timezone.OffsetSeconds != 0 {
		job.Annotations[metadata.TimeZoneOffsetSecondsAnnotation] = fmt.Sprintf("%d", controlledJob.Spec.Timezone.OffsetSeconds)
	}
	if isManuallyScheduled {
		job.Annotations[metadata.ManualJobAnnotation] = "true"
	}
	if startSuspended {
		job.Spec.Suspend = &startSuspended
	}

	for k, v := range controlledJob.Spec.JobTemplate.Labels {
		job.Labels[k] = v
	}
	job.Labels[metadata.ControlledJobLabel] = controlledJob.Name
	if err := ctrl.SetControllerReference(controlledJob, job, k8s.GetScheme()); err != nil {
		return nil, err
	}

	if v, ok := controlledJob.Annotations[metadata.ApplyMutationsAnnotation]; ok && strings.ToLower(v) == "true" {
		if mutatedJob, err := mutators.Apply(ctx, job); err == nil {
			job = mutatedJob
		} else {
			return nil, err
		}
	}

	return job, nil
}
