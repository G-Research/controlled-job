package testhelpers

import (
	"math/rand"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func RandomControlledJobName(baseName string) string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(append([]byte(baseName), b...))
}

func NewControlledJob(name string, opts ...ControlledJobOption) *batch.ControlledJob {
	return NewControlledJobInNamepsace(name, DefaultNamespace, opts...)
}

func NewControlledJobInNamepsace(name string, namespace string, opts ...ControlledJobOption) *batch.ControlledJob {
	obj := &batch.ControlledJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: batch.GroupVersion.Identifier(),
			Kind:       "ControlledJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

type ControlledJobOption func(controlledJob *batch.ControlledJob)

func WithControlledJobName(name string) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		controlledJob.Name = name
	}
}

func WithUID(uid types.UID) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		controlledJob.ObjectMeta.UID = uid
	}
}

func WithTimezone(name string, offsetSeconds int32) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		controlledJob.Spec.Timezone = batch.TimezoneSpec{
			Name:          name,
			OffsetSeconds: offsetSeconds,
		}
	}
}

func WithCronEvent(eventType batch.EventType, cronSpec string) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		controlledJob.Spec.Events = append(controlledJob.Spec.Events, batch.EventSpec{
			Action:       eventType,
			CronSchedule: cronSpec,
		})
	}
}

func WithScheduledEvent(eventType batch.EventType, daysOfWeek, timeOfDay string) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		controlledJob.Spec.Events = append(controlledJob.Spec.Events, batch.EventSpec{
			Action: eventType,
			Schedule: &batch.FriendlyScheduleSpec{
				DaysOfWeek: daysOfWeek,
				TimeOfDay:  timeOfDay,
			},
		})
	}
}

func WithScheduledEventAtTime(eventType batch.EventType, time time.Time) ControlledJobOption {
	// GoLang has this really weird way of doing date time formatting where
	// instead of something sane like yyyy-MM-dd you supply the values
	// of some arbitrary timestamp in 2006..
	// https://stackoverflow.com/questions/20234104/how-to-format-current-time-using-a-yyyymmddhhmmss-format
	day := time.Format("Mon")
	timeOfDay := time.Format("15:04")
	return WithScheduledEvent(eventType, day, timeOfDay)
}

func WithScheduledEventAtTimeEveryDay(eventType batch.EventType, timeOfDay string) ControlledJobOption {
	return WithScheduledEvent(eventType, "SUN-SAT", timeOfDay)
}

func WithEventInThePast(eventType batch.EventType) ControlledJobOption {
	return WithScheduledEventAtTime(eventType, time.Now().Add(-time.Hour))
}
func WithEventInTheFuture(eventType batch.EventType) ControlledJobOption {
	return WithScheduledEventAtTime(eventType, time.Now().Add(time.Hour))
}

func WithJobTemplate(jobTemplate batchv1beta1.JobTemplateSpec) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		controlledJob.Spec.JobTemplate = jobTemplate
	}
}

func WithDefaultJobTemplate() ControlledJobOption {
	return WithJobTemplate(DefaultJobTemplate())
}

func DefaultJobTemplate() batchv1beta1.JobTemplateSpec {
	return *NewJobTemplate(WithPodTemplate(
		*NewPodTemplate(WithPodSpec(DefaultPodSpec())),
	))
}

func WithStartingDeadlineSeconds(deadline int64) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		controlledJob.Spec.StartingDeadlineSeconds = &deadline
	}
}

func WithSpecChangePolicy(policyType batch.SpecChangePolicy) ControlledJobOption {

	return func(controlledJob *batch.ControlledJob) {
		controlledJob.Spec.RestartStrategy = batch.RestartStrategy{
			SpecChangePolicy: policyType,
		}
	}
}

func WithAnnotation(key, value string) ControlledJobOption {
	return func(controlledJob *batch.ControlledJob) {
		if controlledJob.Annotations == nil {
			controlledJob.Annotations = make(map[string]string)
		}
		controlledJob.Annotations[key] = value
	}
}
