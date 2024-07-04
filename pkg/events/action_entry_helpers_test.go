package events

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/stretchr/testify/assert"
)

/*
These helpers aren't really worth unit testing,
but we do want to make sure that they serialize to
JSON correctly
*/
func Test_ActionEntriesSerializeCorrectly(t *testing.T) {
	now := time.Now().UTC()
	NowFunc = func() *time.Time { return &now }

	testCases := map[string]struct {
		ctor         func() *batch.ControlledJobActionHistoryEntry
		expectedJson string
	}{
		"NewJobStartedAction": {
			ctor: func() *batch.ControlledJobActionHistoryEntry {
				return NewJobStartedAction("my-job")
			},
			expectedJson: `{
	"type": "JobStarted",
	"timestamp": "` + now.Format(time.RFC3339) + `",
	"message": "Created job: my-job",
	"jobName": "my-job"
}`,
		},
		"NewJobStoppedAction": {
			ctor: func() *batch.ControlledJobActionHistoryEntry {
				return NewJobStoppedAction("my-job")
			},
			expectedJson: `{
	"type": "JobStopped",
	"timestamp": "` + now.Format(time.RFC3339) + `",
	"message": "Deleted job: my-job",
	"jobName": "my-job"
}`,
		},
		"NewJobFailedAction": {
			ctor: func() *batch.ControlledJobActionHistoryEntry {
				return NewJobFailedAction(FailedToCreateJob, errors.New("this is an error"), "my-job")
			},
			expectedJson: `{
	"type": "FailedToCreateJob",
	"timestamp": "` + now.Format(time.RFC3339) + `",
	"message": "Job my-job failed: this is an error",
	"jobName": "my-job"
}`,
		},
		"NewFailedAction": {
			ctor: func() *batch.ControlledJobActionHistoryEntry {
				return NewFailedAction(FailedToCalculateDesiredStatus, errors.New("this is an error"))
			},
			expectedJson: `{
	"type": "FailedToCalculateDesiredStatus",
	"timestamp": "` + now.Format(time.RFC3339) + `",
	"message": "this is an error"
}`,
		},
	}

	for key, testCase := range testCases {
		t.Run(key, func(t *testing.T) {
			action := testCase.ctor()
			actualJsonBytes, err := json.MarshalIndent(action, "", "\t")

			actualJson := string(actualJsonBytes)

			assert.Nil(t, err, "marshalling to JSON shouldn't result in error")
			assert.Equal(t, testCase.expectedJson, actualJson, "should serialize correctly")
		})
	}
}
