package schedule

import (
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
)

// mapEventToSpecSchedule takes a user-supplied EventSpec and normalizes it
// to a Cron schedule.
// The EventSpec contains either a cron spec as string, or a human friendly format
// In order to work out the next OR previous event in a cron schedule we need to
//  1. Get a canonical cron spec string out of the event
//  2. Get the cron library to parse it
//  3. Cast it to a cron.SpecSchedule object (will always succeed in practice)
func mapEventToSpecSchedule(event batch.EventSpec, location *time.Location) (*cron.SpecSchedule, error) {
	//  1. Get a canonical cron spec string out of the event
	cronSpec, err := event.AsCronSpec()
	if err != nil {
		return nil, err
	}
	//  2. Get the cron library to parse it
	schedule, err := cron.ParseStandard(cronSpec)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse cron schedule")
	}
	//  3. Cast it to a cron.SpecSchedule object (will always succeed in practice)
	// We need to do this so we can examine the parsed fields exposed in SpecSchedule
	// in cronPrev()
	specSchedule, ok := schedule.(*cron.SpecSchedule)
	if !ok {
		return nil, errors.Wrap(err, "Expected instance of SpecSchedule")
	}
	// Ignore any timezone the user has specified in the cron schedule itself (using TZ=foo syntax)
	// and override it using the location specified in the wider ControllerJob spec
	specSchedule.Location = location
	return specSchedule, nil
}

// eventIsNearer determines if testTime is time closer (in the desired direction)
// to the reference time?
func eventIsNearer(testTime, referenceTime time.Time, direction eventDirection) bool {
	if referenceTime.IsZero() {
		return true
	}
	if direction == directionNext {
		return testTime.Before(referenceTime)
	}
	return testTime.After(referenceTime)
}
