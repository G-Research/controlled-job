package testhelpers

import (
	"time"

	kbatch "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultNamespace = "ns"
)

func NewJob(name string, opts ...JobOption) *kbatch.Job {
	return NewJobInNamespace(name, DefaultNamespace, opts...)
}
func NewJobInNamespace(name, namespace string, opts ...JobOption) *kbatch.Job {
	typeMeta := metav1.TypeMeta{}
	typeMeta.SetGroupVersionKind(kbatch.SchemeGroupVersion.WithKind("Job"))
	obj := &kbatch.Job{
		TypeMeta: typeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   namespace,
		},
		Spec: kbatch.JobSpec{},
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

type JobOption func(job *kbatch.Job)

func WithJobName(name string) JobOption {
	return func(job *kbatch.Job) {
		job.Name = name
	}
}

func WithJobAnnotations(a map[string]string) JobOption {
	return func(job *kbatch.Job) {
		job.Annotations = make(map[string]string)
		for key, value := range a {
			job.Annotations[key] = value
		}
	}
}

func WithJobLabels(l map[string]string) JobOption {
	return func(job *kbatch.Job) {
		job.Labels = make(map[string]string)
		for key, value := range l {
			job.Labels[key] = value
		}
	}
}

func WithTemplate(jobTemplate *batchv1beta1.JobTemplateSpec) JobOption {
	return func(job *kbatch.Job) {
		job.Spec = jobTemplate.Spec
	}
}

func IsSuspended(suspend bool) JobOption {
	return func(job *kbatch.Job) {
		job.Spec.Suspend = &suspend
	}
}

func HasSucceeded() JobOption {
	return WithCondition(kbatch.JobCondition{
		Type:   kbatch.JobComplete,
		Status: v1.ConditionTrue,
	})
}

func IsBeingDeleted() JobOption {
	return func(job *kbatch.Job) {
		now := metav1.NewTime(time.Now())
		job.ObjectMeta.DeletionTimestamp = &now
	}
}

func WithJobAnnotation(name, value string) JobOption {
	return func(job *kbatch.Job) {
		job.Annotations[name] = value
	}
}

func WithCondition(condition kbatch.JobCondition) JobOption {
	return func(job *kbatch.Job) {
		job.Status.Conditions = append(job.Status.Conditions, condition)
	}
}

func WithActiveCount(activeCount int32) JobOption {
	return func(job *kbatch.Job) {
		job.Status.Active = activeCount
	}
}
func WithReadyCount(readyCount int32) JobOption {
	return func(job *kbatch.Job) {
		job.Status.Ready = &readyCount
	}
}

func WithJobDeletionTimestamp(timestamp *metav1.Time) JobOption {
	return func(job *kbatch.Job) {
		job.ObjectMeta.DeletionTimestamp = timestamp
	}
}

func WithSuspendFlag(flag *bool) JobOption {
	return func(job *kbatch.Job) {
		job.Spec.Suspend = flag
	}
}

func NewJobTemplate(opts ...JobTemplateOption) *batchv1beta1.JobTemplateSpec {
	obj := &batchv1beta1.JobTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       kbatch.JobSpec{},
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

type JobTemplateOption func(jobTemplate *batchv1beta1.JobTemplateSpec)

func WithJobTemplateAnnotations(a map[string]string) JobTemplateOption {
	return func(jobTemplate *batchv1beta1.JobTemplateSpec) {
		jobTemplate.Annotations = a
	}
}

func WithJobTemplateLabels(l map[string]string) JobTemplateOption {
	return func(jobTemplate *batchv1beta1.JobTemplateSpec) {
		jobTemplate.Labels = l
	}
}

func WithPodTemplate(podTemplate v1.PodTemplateSpec) JobTemplateOption {
	return func(jobTemplate *batchv1beta1.JobTemplateSpec) {
		jobTemplate.Spec.Template = podTemplate
	}
}

func NewPodTemplate(opts ...PodTemplateOption) *v1.PodTemplateSpec {
	obj := &v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       v1.PodSpec{},
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

type PodTemplateOption func(podTemplate *v1.PodTemplateSpec)

func WithPodSpec(podSpec v1.PodSpec) PodTemplateOption {
	return func(podTemplate *v1.PodTemplateSpec) {
		podTemplate.Spec = podSpec
	}
}

func DefaultPodSpec() v1.PodSpec {
	return v1.PodSpec{
		// For simplicity, we only fill out the required fields.
		Containers: []v1.Container{
			{
				Name:  "test-container",
				Image: "test-image",
			},
		},
		RestartPolicy: v1.RestartPolicyOnFailure,
	}
}
