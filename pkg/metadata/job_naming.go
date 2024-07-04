package metadata

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var jobNameRegex *regexp.Regexp

func init() {
	jobNameRegex = regexp.MustCompile(`^(?P<controlledJobName>.+)-(?P<scheduledTimeUnix>\d+)-(?P<JobRunId>\d+)$`)
}

func JobName(controlledJobName string, scheduledTime time.Time, jobRunId int) string {
	return fmt.Sprintf("%s-%d-%d", controlledJobName, scheduledTime.Unix(), jobRunId)
}

func ParseJobName(jobName string) (controlledJobName string, scheduledTime *time.Time, jobRunId *int, err error) {
	matches := jobNameRegex.FindStringSubmatch(jobName)
	if matches == nil {
		err = errors.New(fmt.Sprintf("Failed to parse %s as a valid job name", jobName))
		return
	}

	// matches[0] is the whole matched string
	controlledJobName = matches[1]
	unixSeconds, err := strconv.Atoi(matches[2])
	if err != nil {
		return
	}
	parsedTime := time.Unix(int64(unixSeconds), 0)
	scheduledTime = &parsedTime

	parsedJobId, err := strconv.Atoi(matches[3])
	if err != nil {
		return
	}
	jobRunId = &parsedJobId

	return
}
