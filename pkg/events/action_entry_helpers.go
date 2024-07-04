package events

import (
	"fmt"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Func to get now. Extracted as a variable so we can override it in tests
var NowFunc func() *time.Time

func init() {
	NowFunc = func() *time.Time {
		now := time.Now()
		return &now
	}
}

func NewJobStartedAction(jobName string) *batch.ControlledJobActionHistoryEntry {
	return newActionForJob(string(EventJobStarted), fmt.Sprintf("Created job: %s", jobName), jobName)
}

func NewJobSuspendedAction(jobName string) *batch.ControlledJobActionHistoryEntry {
	return newActionForJob(string(EventJobSuspended), fmt.Sprintf("Suspended job: %s", jobName), jobName)
}
func NewJobUnsuspendedAction(jobName string) *batch.ControlledJobActionHistoryEntry {
	return newActionForJob(string(EventJobUnsuspended), fmt.Sprintf("Unsuspended job: %s", jobName), jobName)
}

func NewJobStoppedAction(jobName string) *batch.ControlledJobActionHistoryEntry {
	return newActionForJob(string(EventJobStopped), fmt.Sprintf("Deleted job: %s", jobName), jobName)
}

func NewJobFailedAction(event WarningEvent, err error, jobName string) *batch.ControlledJobActionHistoryEntry {
	return newActionForJob(string(event), fmt.Sprintf("Job %s failed: %v", jobName, err), jobName)
}

func NewFailedAction(event WarningEvent, err error) *batch.ControlledJobActionHistoryEntry {
	return &batch.ControlledJobActionHistoryEntry{
		Type:      string(event),
		Timestamp: timeOrNilIfZero(NowFunc()),
		Message:   err.Error(),
	}
}

func newActionForJob(eventType, message string, jobName string) *batch.ControlledJobActionHistoryEntry {
	return &batch.ControlledJobActionHistoryEntry{
		Type:      eventType,
		Timestamp: timeOrNilIfZero(NowFunc()),
		Message:   message,
		JobName:   jobName,
	}
}

func timeOrNilIfZero(time *time.Time) *metav1.Time {
	if time == nil || time.IsZero() {
		return nil
	}
	return &metav1.Time{Time: *time}
}
