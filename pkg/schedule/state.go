package schedule

import (
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/pkg/errors"
)

// Type alias to represent the start time of a run period for a ControlledJob
// This start time is what we use as the identifier for Jobs that we spin up (it becomes
// the batch.gresearch.co.uk/scheduled-at annotation), it's how we group Jobs in the
// job.ChildrenOfControlledJob struct, and it's how we determine which Jobs to start
// and stop
type RunPeriodStartTime = time.Time

// State records the surrounding events in a ControlledJob's schedule
type State interface {
	// NextEventTime is the next time in the schedule that we'll hit. This is the time
	// we should ask the controller to wake us up to reprocess this resource
	NextEventTime() *time.Time

	// ShouldBeRunning returns true if we currently expect the controlled job to be running
	ShouldBeRunning() bool

	// The last scheduled stop time in the schedule
	LastStopTime() *time.Time

	// StartOfCurrentRunPeriod is the time at which the controlled job last transitioned from
	// stopped to started.
	//
	// Note this is _not_ strictly speaking the last start event. Imagine a schedule which ended up
	// having:
	// - stop event at 12:00
	// - start event at 13:00
	// - start event at 14:00
	// - stop event at 15:00
	// We would expect to start a new job at 13:00, and then effectively ignore the duplicate start
	// event at 14:00. In the above example, calling StartOfCurrentRunPeriod() at 14:30 would return
	// 13:00.
	//
	// If we're not currently running (ShouldBeRunning() == false) then this will return the start of the most
	// recent completed period. That is, a 'run period' goes from one stop-start transition up until the next one
	// e.g. for a standard 'start at 9am, stop at 5pm' schedule, StartOfCurrentPeriod will be 9am yesterday right up
	// until 9am today
	//
	// This could return nil if there are no previous start events (for example, there are only stop events defined)
	StartOfCurrentRunPeriod() *RunPeriodStartTime
}

type state struct {
	lastStopTime            *time.Time
	startOfCurrentRunPeriod *RunPeriodStartTime
	previousEvent           *ScheduledEvent
	nextEvent               *ScheduledEvent
}

// ScheduledEvent represents an instance of an event
type ScheduledEvent struct {
	Type             batch.EventType
	ScheduledTimeUTC time.Time
}

type locationWithOffset struct {
	*time.Location
	// Additional offset from the specified timezone
	OffsetSeconds int32
}

// StateFor works out the closest previous and next events to the given time in the given ControlledJob's schedule
func StateFor(controlledJob *batch.ControlledJob, now time.Time) (State, error) {
	location, err := time.LoadLocation(controlledJob.Spec.Timezone.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve timezone named %s", controlledJob.Spec.Timezone.Name)
	}
	locationWithOffset := locationWithOffset{
		location,
		controlledJob.Spec.Timezone.OffsetSeconds,
	}
	previousEvent, err := findNearestEvent(controlledJob.Spec.Events, now, locationWithOffset, directionPrevious, func(es batch.EventSpec) bool { return true })
	if err != nil {
		return nil, errors.Wrap(err, "failed to find previous event in the schedule")
	}
	nextEvent, err := findNearestEvent(controlledJob.Spec.Events, now, locationWithOffset, directionNext, func(es batch.EventSpec) bool { return true })
	if err != nil {
		return nil, errors.Wrap(err, "failed to find next event in the schedule")
	}

	lastStopTime, err := findMostRecentStopTime(controlledJob.Spec.Events, locationWithOffset, now)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find most recent stop time")
	}

	startOfCurrentRunPeriod, err := findStartOfCurrentRunPeriod(controlledJob.Spec.Events, locationWithOffset, now)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find start of current run period")
	}

	return &state{
		lastStopTime:            lastStopTime,
		startOfCurrentRunPeriod: startOfCurrentRunPeriod,
		previousEvent:           previousEvent,
		nextEvent:               nextEvent,
	}, nil
}

func (s *state) NextEventTime() *time.Time {
	if s.nextEvent == nil {
		return nil
	}
	return &s.nextEvent.ScheduledTimeUTC
}

func (s *state) ShouldBeRunning() bool {
	return s.previousEvent != nil && s.previousEvent.Type == batch.EventTypeStart
}

func (s *state) LastStopTime() *time.Time {
	return s.lastStopTime
}

func (s *state) StartOfCurrentRunPeriod() *RunPeriodStartTime {
	return s.startOfCurrentRunPeriod
}

func findMostRecentStopTime(events []batch.EventSpec, locationWithOffset locationWithOffset, now time.Time) (*RunPeriodStartTime, error) {
	lastStopEvent, err := findNearestEvent(events, now, locationWithOffset, directionPrevious,
		func(es batch.EventSpec) bool { return es.Action == batch.EventTypeStop },
	)
	if err != nil {
		return nil, err
	}
	if lastStopEvent == nil {
		return nil, nil
	}
	return &lastStopEvent.ScheduledTimeUTC, nil
}

func findStartOfCurrentRunPeriod(events []batch.EventSpec, locationWithOffset locationWithOffset, now time.Time) (*RunPeriodStartTime, error) {
	// To find the start of the current run period we go back until we find a stop event (which
	// defines the end of the previous period) and then go forward from there until we find a start event

	lastStopTime, err := findMostRecentStopTime(events, locationWithOffset, now)
	if err != nil {
		return nil, err
	}
	if lastStopTime == nil {
		// No recent stop event.
		hasAnyStartEvent := false
		for _, event := range events {
			if event.Action == batch.EventTypeStart {
				hasAnyStartEvent = true
				break
			}
		}

		if !hasAnyStartEvent {
			// No start events, so just return nil
			return nil, nil
		}

		// If there _are_ some start events though, return an error because we can't be sure what to do here for the best.
		// Why?
		// Because a schedule of just start events could be the user trying to:
		// - start a new job at each start event, even if previous runs are still going (like CronJob semantics)
		// - make sure the job is still running
		// - explicitly restart the job each start event
		// - a mistake - they forgot to add stop events
		// Because we can't tell the difference, for now let's fail-fast and fail-safe by not running anything
		// until we get a real user requirement for start-only schedules
		return nil, errors.New("No previous stop events found, only start events. We don't currently support start-only schedules (as it's not clear what the semantics should be)")
	}

	nextStartEvent, err := findNearestEvent(events, *lastStopTime, locationWithOffset, directionNext,
		func(es batch.EventSpec) bool { return es.Action == batch.EventTypeStart },
	)
	if err != nil {
		return nil, err
	}

	if nextStartEvent == nil {
		return nil, nil
	}

	return &nextStartEvent.ScheduledTimeUTC, nil
}
