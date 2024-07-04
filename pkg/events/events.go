package events

import (
	"strings"

	"github.com/pkg/errors"
)

type NormalEvent string
type WarningEvent string

const (
	EventJobStarted     NormalEvent = "JobStarted"
	EventJobStopped     NormalEvent = "JobStopped"
	EventJobRestarted   NormalEvent = "JobRestarted"
	EventJobSuspended   NormalEvent = "JobSuspended"
	EventJobUnsuspended NormalEvent = "JobUnsuspended"

	// All warning events must start with 'Failed'
	FailedToReconcile              WarningEvent = "FailedToReconcile"
	FailedToListJobs               WarningEvent = "FailedToListJobs"
	FailedToListJobsForPeriod      WarningEvent = "FailedToListJobsForPeriod"
	FailedToUpdateStatus           WarningEvent = "FailedToUpdateStatus"
	FailedToCalculateSchedule      WarningEvent = "FailedToCalculateSchedule"
	FailedToCalculateDesiredStatus WarningEvent = "FailedToCalculateDesiredStatus"
	FailedToTemplateJob            WarningEvent = "FailedToTemplateJob"
	FailedToCreateJob              WarningEvent = "FailedToCreateJob"
	FailedToDeleteJob              WarningEvent = "FailedToDeleteJob"
	FailedToSuspendJob             WarningEvent = "FailedToSuspendJob"
	FailedToUnsuspendJob           WarningEvent = "FailedToUnsuspendJob"
)

func IsWarningEvent(event string) bool {
	return strings.HasPrefix(event, "Failed")
}

type ErrorWithEvent struct {
	err   error
	event WarningEvent
}

func (e *ErrorWithEvent) Error() string {
	return e.err.Error()
}

func (e *ErrorWithEvent) Event() WarningEvent {
	return e.event
}

func WrapError(err error, event WarningEvent, message string) error {
	return &ErrorWithEvent{
		err:   errors.Wrap(err, message),
		event: event,
	}
}
