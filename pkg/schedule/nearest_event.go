package schedule

import (
	"time"

	"github.com/robfig/cron/v3"

	batch "github.com/G-Research/controlled-job/api/v1"
)

type eventDirection int

const (
	directionNext eventDirection = iota
	directionPrevious
)

type eventFilter func(batch.EventSpec) bool

// A controlledJob contains a list of events with different cron schedules
// We need to be able to find, amongst all those events, the most recent and the next
// scheduled event.
// This func searches either forward or backward for the nearest event to 'now' among the event specs
// The provided filter fucntion should return true if we should consider the given event spec. This allows
// us to e.g. find the nearest stop event specifically, not just the nearest event of any kind
//
// If the schedule is invalid, then err will be non-nil and will explain how it's invalid
// If there is no nearest matching event in the given direction, then both return values will be nil
func findNearestEvent(schedule []batch.EventSpec, now time.Time, locationWithOffset locationWithOffset, direction eventDirection, filter eventFilter) (*ScheduledEvent, error) {

	cronSchedulesToSearch := make([]*cron.SpecSchedule, 0)
	// The function to search the schedules will return an index to the schedule
	// that matched. We will use that to index into _this_ slice as well to work
	// out what the action was for the matching schedule
	correspondingActions := make([]batch.EventType, 0)

	// We support multiple event specs on one ControlledJob
	// so we need to loop over each and for each event:
	//  1. Extract it's cron SpecSchedule
	//  2. Work out what the nearest adjacent event in that spec in the desired direction is
	//  3. Compare that to our current 'nearest' event
	for _, event := range schedule {

		if !filter(event) {
			// We shouldn't consider this event
			continue
		}

		// Extract its cron SpecSchedule
		specSchedule, err := mapEventToSpecSchedule(event, locationWithOffset.Location)
		if err != nil {
			return nil, err
		}

		cronSchedulesToSearch = append(cronSchedulesToSearch, specSchedule)
		correspondingActions = append(correspondingActions, event.Action)
	}

	nearestEventTime, nearestEventIdx := findNearestScheduleTime(cronSchedulesToSearch, now, direction, locationWithOffset.OffsetSeconds)

	if nearestEventTime.IsZero() {
		return nil, nil
	}
	return &ScheduledEvent{
		Type:             correspondingActions[nearestEventIdx],
		ScheduledTimeUTC: nearestEventTime,
	}, nil
}

// findNearestScheduleTime is the core logic used to determine the nearest scheduled time from a list of schedules, either forward
// or backward in time.
// The return values are the time of the nearest event in the given direction, and the index of the schedule from the provided schedules slice
// which is responsible for that timestamp (e.g. if schedules[1] has an event closer to now than schedules[0], nearestScheduleIdx will be returned as 1)
// If no matching scheduled time is found in either direction, (time.Time{}, -1) will be returned
func findNearestScheduleTime(schedules []*cron.SpecSchedule, now time.Time, direction eventDirection, additionalOffsetSeconds int32) (nearestEventTime time.Time, nearestScheduleIdx int) {

	nearestEventTime = time.Time{}
	nearestScheduleIdx = -1

	// This handles any additional offset seconds requested by the user.
	// The cron schedules have the regular, named, timezone embedded in them (e.g. America/New_York)
	// and so if you pass in 9:00 UTC to specSchedule.Next(now) or cronPrev(specSchedule, now)
	// it will first translate that UTC time into local time in the named timezone so that
	// the schedules work in those local times
	// BUT if the user has an extra offset specified that logic won't work by default (9am in the user's desired local time
	// might be 10:01am in UTC, not just 10am)
	// So what we do is add the offset to the passed in 'now' time here so that that extra offset is taken into account
	// in the cron calculations
	now = now.Add(time.Second * time.Duration(additionalOffsetSeconds))

	for idx, specSchedule := range schedules {
		var adjacentEventTime time.Time
		if direction == directionNext {
			adjacentEventTime = specSchedule.Next(now)
		} else {
			adjacentEventTime = cronPrev(specSchedule, now)
		}

		if adjacentEventTime.IsZero() {
			// Could not find a time to satisfy the schedule in that direction
			continue
		}

		// The cron calculation will have returned us a time in UTC, but without taking the additional
		// offset into account. e.g. if the schedule says 9am in UTC-1 with an extra offset of +60s
		// then cron will return 10am (i.e. 9am UTC-1 in UTC), but we want to return 09:59 UTC to the user
		// so we have to subtract the additional offset seconds
		adjacentEventTime = adjacentEventTime.Add(time.Second * time.Duration(-additionalOffsetSeconds))

		//  3. Compare that to our current 'nearest' event
		if eventIsNearer(adjacentEventTime, nearestEventTime, direction) {
			nearestEventTime = adjacentEventTime
			nearestScheduleIdx = idx
		}
	}
	return
}
