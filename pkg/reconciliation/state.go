package reconciliation

import (
	"context"
	"fmt"
	"strings"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	v1 "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/events"
	"github.com/G-Research/controlled-job/pkg/metadata"
	"github.com/G-Research/controlled-job/pkg/schedule"
	"github.com/go-logr/logr"
	kbatch "k8s.io/api/batch/v1"
)

// State collects the current state of a ControlledJob
// and its owned Jobs. This means we can pre-compute these
// values in one go, and use them to decide what to do
type state struct {
	IsSuspended             bool
	ShouldBeRunning         *bool
	StartOfCurrentRunPeriod *schedule.RunPeriodStartTime
	LastStopTime            *time.Time
	NextEventTime           *time.Time
	AllJobs                 []*kbatch.Job
	DesiredHash             string
	AutoRestartIsEnabled    bool
}

// GetStateForReconcile loads information from the cluster for the given target ControlledJob we've
// been asked to reconcile. This involves:
// - Resolving the ControlledJob itself
//   - If this is not found, we will return a nil state and no error (it's assumed the job has been deleted by the user)
//
// - Resolving any Jobs owned by the ControlledJob
// - Calculating the schedule state - should the job currently be running? When's the next event time etc.
func buildState(ctx context.Context, controlledJob *batch.ControlledJob, childJobs *kbatch.JobList, now time.Time) (*state, error) {

	scheduleState, err := schedule.StateFor(controlledJob, now)
	if err != nil {
		return nil, events.WrapError(err, events.FailedToCalculateSchedule, fmt.Sprintf("Failed to calculate schedule for controlled job %s in namespace %s", controlledJob.Name, controlledJob.Namespace))
	}

	allJobs := make([]*kbatch.Job, len(childJobs.Items))
	for i := range childJobs.Items {
		allJobs[i] = &childJobs.Items[i]
	}

	startOfCurrentRunPeriod := scheduleState.StartOfCurrentRunPeriod()

	var shouldBeRunning *bool = nil
	if startOfCurrentRunPeriod != nil {
		val := scheduleState.ShouldBeRunning()
		shouldBeRunning = &val
	}

	return &state{
		IsSuspended:             controlledJob.Spec.Suspend != nil && *controlledJob.Spec.Suspend,
		ShouldBeRunning:         shouldBeRunning,
		StartOfCurrentRunPeriod: startOfCurrentRunPeriod,
		LastStopTime:            scheduleState.LastStopTime(),
		NextEventTime:           scheduleState.NextEventTime(),
		AllJobs:                 allJobs,
		DesiredHash:             metadata.CalculateHashFor(controlledJob.Spec.JobTemplate),
		AutoRestartIsEnabled:    strings.EqualFold(string(controlledJob.Spec.RestartStrategy.SpecChangePolicy), string(v1.RecreateSpecChangePolicy)),
	}, nil
}

func (s *state) AddToLog(log logr.Logger) logr.Logger {
	return log.
		WithValues("shouldBeRunning", s.ShouldBeRunning).
		WithValues("startOfCurrentRunPeriod", s.StartOfCurrentRunPeriod).
		WithValues("nextEvent", s.NextEventTime).
		WithValues("jobsCount", len(s.AllJobs))
}

func jobsForLog(jobsByPeriod map[schedule.RunPeriodStartTime][]*kbatch.Job) (result map[schedule.RunPeriodStartTime][]string) {
	result = make(map[time.Time][]string)
	for key, jobs := range jobsByPeriod {
		result[key] = make([]string, len(jobs))
		for i, job := range jobs {
			result[key][i] = job.Name
		}
	}
	return
}
