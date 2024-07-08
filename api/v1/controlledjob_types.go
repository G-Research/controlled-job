/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"errors"
	"fmt"
	"regexp"

	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TimezoneSpec defines the timezone which governs scheduled times
type TimezoneSpec struct {
	// Name of the timezone in the tzData. See https://golang.org/pkg/time/#LoadLocation for possible values
	Name string `json:"name"`

	// Additional offset from UTC on top of the specified timezone. If the timezone is normally UTC-2, and
	// OffsetSeconds is +3600 (1h in seconds), then the overall effect will be UTC-1. If the timezone is
	// normally UTC+2 and OffsetSeconds is +3600, then the overall effect will be UTC+3.
	//
	// In practice - if you set this field to 60s on top of a normally UTC-1 timezone, then you end up with
	// a 'UTC-59m' timezone. In that timezone 10:00 UTC == 09:01 UTC-59m. So if you have a scheduled start time
	// of 09:00 in that UTC-59m timezone, your job will be started at 09:59 UTC
	//
	// +optional
	OffsetSeconds int32 `json:"offset"`
}

type EventType string

const (
	EventTypeStart EventType = "start"
	EventTypeStop  EventType = "stop"
	// Restart not yet supported
	EventTypeRestart EventType = "restart"
)

// FriendlyScheduleSpec is a more user friendly way to specify an event schedule
// It's more limited than the format supported by CronSchedule
type FriendlyScheduleSpec struct {
	// TimeOfDay this event happens on the specified days
	// Format: hh:mm
	// +kubebuilder:validation:Pattern:=`^(\d{2}):(\d{2})$`
	TimeOfDay string `json:"timeOfDay"`

	// DaysOfWeek this event occurs on.
	// Either a comma separated list (MON,TUE,THU)
	// Or a range (MON-FRI)
	// +kubebuilder:validation:Pattern:=`(?:^([a-zA-Z]{3})(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?)$|^(?P<startRange>[a-zA-Z]{3})-(?P<endRange>[a-zA-Z]{3}$)`
	DaysOfWeek string `json:"daysOfWeek"`
}

// A specific event in the schedule
type EventSpec struct {
	// Action to take at the specified time(s)
	Action EventType `json:"action"`

	// CronSchedule can contain an arbitrary Golang Cron Schedule
	// (see https://pkg.go.dev/github.com/robfig/cron#hdr-CRON_Expression_Format)
	// If set, takes precedence over Schedule
	// +optional
	CronSchedule string `json:"cronSchedule,omitempty"`

	// Schedule is a more user friendly way to specify an event schedule
	// It's more limited than the format supported by CronSchedule
	Schedule *FriendlyScheduleSpec `json:"schedule,omitempty"`
}

var (
	timeOfDayRegex  = regexp.MustCompile(`^(\d{2}):(\d{2})$`)
	daysOfWeekRegex = regexp.MustCompile(`(?:^([a-zA-Z]{3})(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?(?:,([a-zA-Z]{3}))?)$|^(?P<startRange>[a-zA-Z]{3})-(?P<endRange>[a-zA-Z]{3}$)`)
)

// AsCronSpec presents the given EventSpec in CronTab format (as that's how the scheduling code needs to process it in).
// If a CronSchedule is provided, it is returned unaltered and un-validated. Otherwise we check against some regexes for validation
// before converting to a cron format
func (e *EventSpec) AsCronSpec() (string, error) {
	if e.CronSchedule != "" {
		return e.CronSchedule, nil
	}
	if e.Schedule.TimeOfDay == "" || e.Schedule.DaysOfWeek == "" {
		return "", errors.New("must specify either cronSchedule or schedule")
	}
	timeOfDayMatches := timeOfDayRegex.FindStringSubmatch(e.Schedule.TimeOfDay)
	// timeOfDayMatches should contain the entire string, the hours part, and the minutes part (the two subexpressions), so should
	// have a length of 3
	if len(timeOfDayMatches) != 3 {
		return "", errors.New("timeOfDay must be in the format hh:mm")
	}

	if !daysOfWeekRegex.Match([]byte(e.Schedule.DaysOfWeek)) {
		return "", errors.New("daysOfWeek must be in the format MON-FRI or SAT,SUN,TUE,WED")
	}

	return fmt.Sprintf("%s %s * * %s", timeOfDayMatches[2], timeOfDayMatches[1], e.Schedule.DaysOfWeek), nil
}

type FailurePolicy string

const (
	NeverRestartFailurePolicy  FailurePolicy = "NeverRestart"
	AlwaysRestartFailurePolicy FailurePolicy = "AlwaysRestart"
)

type SpecChangePolicy string

type RestartStrategy struct {
	// SpecChangePolicy deals with policy to apply when the jobTemplate of the controlled job changes while it's running
	// Valid values are:
	//
	// - "Ignore": (default) Do nothing if spec of a job differs from spec of controlled job (next scheduled creation
	//     pick up the change)
	//
	// - "Recreate": If the job is currently running, stop it and wait for it to have completely stopped before starting
	//     a new job with the updated spec
	//
	// Note - the terminology here consciously mirrors that of a Deployment's Strategy:
	// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy
	// +optional
	SpecChangePolicy SpecChangePolicy `json:"specChangePolicy,omitempty"`
}

const (
	// IgnoreSpecChangePolicy ignores changes to the job template while the job is running. The change
	// will be picked up the next time a Job is created
	IgnoreSpecChangePolicy SpecChangePolicy = "Ignore"
	// RecreateSpecChangePolicy will, when a change to the job template is detected, immediately kill any running Jobs,
	// wait for them to fully stop, and then create a new Job running the new specification
	RecreateSpecChangePolicy SpecChangePolicy = "Recreate"
)

// ControlledJobSpec defines the desired state of ControlledJob
type ControlledJobSpec struct {

	// Timezone which governs the timing of all Events
	Timezone TimezoneSpec `json:"timezone"`

	// Events are a list of timings and operations to perform at those times. For example, 'start at 09:00', 'stop every hour on the half hour'
	Events []EventSpec `json:"events"`

	// Specifies the job that will be created when executing a CronJob. Uses the native Kubernetes JobTemplateSpec, and so supports all features
	// Kubernetes Jobs natively support
	JobTemplate batchv1beta1.JobTemplateSpec `json:"jobTemplate"`

	//+kubebuilder:validation:Minimum=0

	// Optional deadline in seconds for starting the job if it misses scheduled
	// time for any reason. In other words, if a new job is expected to be running, but it's more than
	// startingDeadlineSeconds after the scheduled start time, no job will be created.
	// If not set or set to < 1 this has no effect, and jobs will always be started however long after the start
	// time it is.
	// WARNING!!! Be aware that enabling this setting makes it impossible to restart a controlled job this number of seconds after
	// a scheduled start time. For example if the scheduled start time is 9am and this is set to 3600 (1h), then if you try to restart
	// the controlled job by deleting the current Job any time after 10am, it will have no effect.
	// +optional
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	// Specifies options on how to deal with job restart behaviour for various triggers
	// +optional
	RestartStrategy RestartStrategy `json:"restartStrategy,omitempty"`

	// This flag tells the controller to suspend subsequent executions, it does
	// not apply to already started executions.  Defaults to false.
	// Is also set by the controller when a job fails and should not be restarted.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`
}

// ControlledJobStatus defines the observed state of ControlledJob
type ControlledJobStatus struct {

	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// The most recent scheduled start time that was actioned
	// +optional
	LastScheduledStartTime *metav1.Time `json:"lastScheduledStartTime,omitempty"`

	// ShouldBeRunning is true if we're between a start/stop event
	// +optional
	ShouldBeRunning *bool `json:"shouldBeRunning,omitempty"`

	// IsRunning is true if there are any active events
	// +optional
	IsRunning *bool `json:"isRunning,omitempty"`

	// Set by the controller when a job fails and we shouldn't
	// restart it. Can be cleared by the user resetting the
	// spec.suspend flag
	// +optional
	IsSuspended *bool `json:"isSuspended,omitempty"`

	// MostRecentAction is the most recent action taken by this ControlledJob
	// +optional
	MostRecentAction *ControlledJobActionHistoryEntry `json:"mostRecentAction,omitempty"`

	// ActionHistory gives the recent history of actions taken by this ControlledJob
	// e.g. job started, job killed etc. in reverse chronological order
	// The number of recent actions is limited to 16
	// +optional
	ActionHistory []ControlledJobActionHistoryEntry `json:"actionHistory,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// ControlledJobActionHistoryEntry
type ControlledJobActionHistoryEntry struct {
	// Type is the action the ControlledJob took
	Type string `json:"type"`
	// Timestamp is the time the condition was last observed
	Timestamp *metav1.Time `json:"timestamp"`
	// Message contains human-readable message indicating details about the action
	// +optional
	Message string `json:"message,omitempty"`
	// The most recent scheduled start time prior to this action. This allows grouping of
	// actions by start time to see a 'history for today'
	// NOW DEPRECATED AND NOT SET ANYMORE
	// +optional
	ScheduledStartTime *metav1.Time `json:"scheduledStartTime,omitempty"`
	// JobIndex is an incrementing number of jobs for the current run period.
	// At a start time in the schedule, a job with index 0 will be created. If that
	// fails and auto-restart is enabled a new job with index 1 will be created in its
	// place. This field records the JobIndex of the affected job
	// +optional
	JobIndex *int `json:"jobIndex,omitempty"`
	// JobName is the name of the job affected by this action (if any). e.g. the job
	// that was started, or stopped
	// +optional
	JobName string `json:"jobName,omitempty"`
}

// ControlledJobConditionType is a enum type defining the conditions that ControlledJobs support
type ControlledJobConditionType string

const (
	// ConditionTypeShouldBeRunning is set to True when the ControlledJob is between a start and stop time, and False if
	// between a stop and start time. If there are no start times, it will be set to Unknown
	ConditionTypeShouldBeRunning ControlledJobConditionType = "ShouldBeRunning"

	// ConditionTypeSuspended is set to True if the user has marked this ControlledJob as suspended
	ConditionTypeSuspended ControlledJobConditionType = "Suspended"

	// ConditionTypeOutOfDate is True if the spec of the running job does not match the desired JobSpec, and we
	// are not able to recreate the job with the new spec
	ConditionTypeOutOfDate ControlledJobConditionType = "OutOfDate"

	// ConditionTypeJobManuallyScheduled is True if the current job (if any) was manually scheduled by a user, not
	// created based on a start event
	ConditionTypeJobManuallyScheduled ControlledJobConditionType = "JobManuallyScheduled"

	// ConditionTypeJobExists is True if there is a current job that isn't being deleted
	ConditionTypeJobExists ControlledJobConditionType = "JobExists"

	// ConditionTypeJobRunning is True when the job exists, and has reached its expected number of ready pods. That is, all pods have been
	// created and are reporting as ready. This will be False if the Job exists, but hasn't reached this state yet. Staying in a False state
	// for a while is an indication that the Pod is failing to successfully start (image pull failure etc)
	// Note that the 'ready' status of a pod is a beta feature, so if it is not present on the Job, this will always be Unknown
	ConditionTypeJobRunning ControlledJobConditionType = "JobRunning"

	// ConditionTypeJobComplete is True if the current job (if any) reports itself as complete
	ConditionTypeJobComplete ControlledJobConditionType = "JobComplete"

	// ConditionTypeJobFailed is True if the current job (if any) reports itself as failed
	ConditionTypeJobFailed ControlledJobConditionType = "JobFailed"

	// ConditionTypeJobBeingDeleted is True if the current job (if any) is currently being deleted
	ConditionTypeJobBeingDeleted ControlledJobConditionType = "JobBeingDeleted"

	// ConditionTypeJobSuspended is True if the current job (if any) is currently suspended. This often happens at startup
	// so that we can ensure only one Job is running at any time
	ConditionTypeJobSuspended ControlledJobConditionType = "JobSuspended"

	// ConditionTypeJobStoppedByUser is True if the current job (if any) was stopped by the user (using the API)
	ConditionTypeJobStoppedByUser ControlledJobConditionType = "JobStoppedByUser"

	// ConditionTypeFailedToCreateJob occurs if we encountered an error the last time we tried to create a job
	ConditionTypeFailedToCreateJob ControlledJobConditionType = "FailedToCreateJob"

	// ConditionTypeFailedToSuspendJob occurs if we encountered an error the last time we tried to suspend a job
	ConditionTypeFailedToSuspendJob ControlledJobConditionType = "FailedToSuspendJob"

	// ConditionTypeFailedToUnsuspendJob occurs if we encountered an error the last time we tried to unsuspend a job
	ConditionTypeFailedToUnsuspendJob ControlledJobConditionType = "FailedToUnsuspendJob"

	// ConditionTypeFailedToDeleteJob occurs if we encountered an error the last time we tried to delete a job
	ConditionTypeFailedToDeleteJob ControlledJobConditionType = "FailedToDeleteJob"

	// ConditionTypeFailedToDeleteJob occurs if we expect to be starting a job, but the configured StartingDeadline has been exceeded
	ConditionTypeStartingDeadlineExceeded ControlledJobConditionType = "StartingDeadlineExceeded"

	// ConditionTypeRunningExpectedly is true if JobPotentiallyRunning, and either ShouldBeRunning or JobManuallyScheduled
	ConditionTypeRunningExpectedly ControlledJobConditionType = "RunningExpectedly"

	// ConditionTypeRunningUnexpectedly is true if JobPotentiallyRunning, and both NOT ShouldBeRunning and NOT JobManuallyScheduled
	ConditionTypeRunningUnexpectedly ControlledJobConditionType = "RunningUnexpectedly"

	// ConditionTypeNotRunningExpectedly is true if NOT JobPotentiallyRunning, and both NOT ShouldBeRunning and NOT JobManuallyScheduled
	ConditionTypeNotRunningExpectedly ControlledJobConditionType = "NotRunningExpectedly"

	// ConditionTypeNotRunningUnexpectedly is true if NOT JobPotentiallyRunning, and either ShouldBeRunning or JobManuallyScheduled
	ConditionTypeNotRunningUnexpectedly ControlledJobConditionType = "NotRunningUnexpectedly"

	// ConditionTypeError records if the last attempt to reconcile generated an error, and if so what error
	ConditionTypeError ControlledJobConditionType = "Error"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Is running",type=boolean,JSONPath=`.status.isRunning`
//+kubebuilder:printcolumn:name="Should be running",type=boolean,JSONPath=`.status.shouldBeRunning`
//+kubebuilder:printcolumn:name="Suspended",type=boolean,JSONPath=`.status.isSuspended`
//+kubebuilder:printcolumn:name="Last scheduled start time",type=date,JSONPath=`.status.lastScheduledStartTime`
//+kubebuilder:resource:shortName="ctj"

// ControlledJob is the Schema for the controlledjobs API
type ControlledJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlledJobSpec   `json:"spec,omitempty"`
	Status ControlledJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ControlledJobList contains a list of ControlledJob
type ControlledJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlledJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ControlledJob{}, &ControlledJobList{})
}
