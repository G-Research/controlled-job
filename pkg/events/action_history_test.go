package events

import (
	"context"
	"testing"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_AddActionHistoryEntry(t *testing.T) {
	name := "my-controlled-job"
	namespace := "my-ns"
	metadata := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}

	existingAction := &batch.ControlledJobActionHistoryEntry{
		Type:      string(EventJobStarted),
		Message:   "First event",
		Timestamp: &metav1.Time{Time: time.Now()},
	}

	testCases := map[string]struct {
		name      string
		namespace string

		controlledJob *batch.ControlledJob
		action        *batch.ControlledJobActionHistoryEntry
		updateError   error
		test          func(t *testing.T, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry)
	}{
		"FirstEntry_AddsSuccessfully": {
			name:      name,
			namespace: namespace,
			controlledJob: &batch.ControlledJob{
				ObjectMeta: metadata,
			},
			action: &batch.ControlledJobActionHistoryEntry{
				Type:      string(EventJobStarted),
				Message:   "This is a test",
				Timestamp: &metav1.Time{Time: time.Now()},
			},
			test: func(t *testing.T, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
				assert.Equal(t, 1, len(controlledJob.Status.ActionHistory))
				assert.Equal(t, *action, controlledJob.Status.ActionHistory[0])
				assert.Equal(t, action, controlledJob.Status.MostRecentAction)
			},
		},
		"SubsequentEntry_PrependsSuccessfully": {
			name:      name,
			namespace: namespace,
			controlledJob: &batch.ControlledJob{
				ObjectMeta: metadata,
				Status: batch.ControlledJobStatus{
					ActionHistory: []batch.ControlledJobActionHistoryEntry{*existingAction},
				},
			},
			action: &batch.ControlledJobActionHistoryEntry{
				Type:      string(EventJobStopped),
				Message:   "This is a test",
				Timestamp: &metav1.Time{Time: time.Now()},
			},
			test: func(t *testing.T, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
				assert.Equal(t, 2, len(controlledJob.Status.ActionHistory))
				assert.Equal(t, *action, controlledJob.Status.ActionHistory[0])
				assert.Equal(t, *existingAction, controlledJob.Status.ActionHistory[1])
				assert.Equal(t, action, controlledJob.Status.MostRecentAction)
			},
		},
		"SubsequentEntry_LimitsTo16": {
			name:      name,
			namespace: namespace,
			controlledJob: &batch.ControlledJob{
				ObjectMeta: metadata,
				Status: batch.ControlledJobStatus{
					ActionHistory: repeatedAction(*existingAction, 20),
				},
			},
			action: &batch.ControlledJobActionHistoryEntry{},
			test: func(t *testing.T, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
				assert.Equal(t, 16, len(controlledJob.Status.ActionHistory))
			},
		},
	}

	for key, testCase := range testCases {
		t.Run(key, func(t *testing.T) {

			addActionHistoryEntry(context.Background(), testCase.controlledJob, testCase.action)

			testCase.test(t, testCase.controlledJob, testCase.action)
		})
	}
}

func Test_AddActionHistoryEntryAddActionHistoryEntryIgnoringDuplicates_DoesNotRecordDuplicateAction(t *testing.T) {
	timeInThePast := &metav1.Time{Time: time.Now().Add(-time.Hour)}
	timeNow := &metav1.Time{Time: time.Now()}
	existingAction := &batch.ControlledJobActionHistoryEntry{
		Type:      "Foo",
		Message:   "Bar",
		Timestamp: timeInThePast,
	}
	newAction := &batch.ControlledJobActionHistoryEntry{
		Type:      "Foo",
		Message:   "Bar",
		Timestamp: timeNow,
	}
	controlledJob := &batch.ControlledJob{
		Status: batch.ControlledJobStatus{
			MostRecentAction: existingAction,
		}}

	addActionHistoryEntryIgnoringDuplicates(context.Background(), controlledJob, newAction)
	assert.Equal(t, existingAction, controlledJob.Status.MostRecentAction, "should not have updated the status")
}

func Test_AddActionHistoryEntryAddActionHistoryEntryIgnoringDuplicates_DoesRecordDuplicateAction(t *testing.T) {
	timeInThePast := &metav1.Time{Time: time.Now().Add(-time.Hour)}
	timeNow := &metav1.Time{Time: time.Now()}
	existingAction := &batch.ControlledJobActionHistoryEntry{
		Type:      "Foo",
		Message:   "Bar",
		Timestamp: timeInThePast,
	}
	newAction := &batch.ControlledJobActionHistoryEntry{
		Type:      "Foo",
		Message:   "Bar",
		Timestamp: timeNow,
	}
	controlledJob := &batch.ControlledJob{
		Status: batch.ControlledJobStatus{
			MostRecentAction: existingAction,
		}}

	addActionHistoryEntry(context.Background(), controlledJob, newAction)
	assert.Equal(t, newAction, controlledJob.Status.MostRecentAction, "should have updated the status")
}

// Make sure that if two actions are structurally identical (except for their timestamp) that
// we consider them the same action
func Test_isSameAction(t *testing.T) {
	type1 := "TypeOne"
	type2 := "Type2"
	message1 := "Message one"
	message2 := "Message two"
	startTime1 := &metav1.Time{Time: time.Date(2021, 12, 30, 12, 0, 0, 0, time.UTC)}
	startTime1Duplicate := &metav1.Time{Time: time.Date(2021, 12, 30, 12, 0, 0, 0, time.UTC)}
	startTime2 := &metav1.Time{Time: time.Date(2022, 1, 1, 1, 0, 0, 0, time.UTC)}
	timeInThePast := &metav1.Time{Time: time.Now().Add(-time.Hour)}
	timeNow := &metav1.Time{Time: time.Now()}
	index1 := 1
	index1Duplicate := 1
	index2 := 2
	jobName1 := "job-1-1"
	jobName2 := "job-1-2"

	testCases := map[string]struct {
		existing       *batch.ControlledJobActionHistoryEntry
		proposed       *batch.ControlledJobActionHistoryEntry
		expectedResult bool
	}{
		"existing is nil - always false": {
			existing:       nil,
			proposed:       &batch.ControlledJobActionHistoryEntry{},
			expectedResult: false,
		},
		"field values differ - not the same": {
			existing: &batch.ControlledJobActionHistoryEntry{
				Type:               type1,
				Message:            message1,
				ScheduledStartTime: startTime1,
				JobIndex:           &index1,
				JobName:            jobName1,
				Timestamp:          timeInThePast,
			},
			proposed: &batch.ControlledJobActionHistoryEntry{
				Type:               type2,
				Message:            message2,
				ScheduledStartTime: startTime2,
				JobIndex:           &index2,
				JobName:            jobName2,
				Timestamp:          timeNow,
			},
			expectedResult: false,
		},
		"field pointers differ but values are the same - should return true": {
			existing: &batch.ControlledJobActionHistoryEntry{
				Type:               type1,
				Message:            message1,
				ScheduledStartTime: startTime1,
				JobIndex:           &index1,
				JobName:            jobName1,
				Timestamp:          timeInThePast,
			},
			proposed: &batch.ControlledJobActionHistoryEntry{
				Type:               type1,
				Message:            message1,
				ScheduledStartTime: startTime1Duplicate,
				JobIndex:           &index1Duplicate,
				JobName:            jobName1,
				Timestamp:          timeNow,
			},
			expectedResult: true,
		},
		"identical (except for timestamp) - is same": {
			existing: &batch.ControlledJobActionHistoryEntry{
				Type:               type1,
				Message:            message1,
				ScheduledStartTime: startTime1,
				JobIndex:           &index1,
				JobName:            jobName1,
				Timestamp:          timeInThePast,
			},
			proposed: &batch.ControlledJobActionHistoryEntry{
				Type:               type1,
				Message:            message1,
				ScheduledStartTime: startTime1,
				JobIndex:           &index1,
				JobName:            jobName1,
				Timestamp:          timeNow,
			},
			expectedResult: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			result := isSameAction(testCase.existing, testCase.proposed)
			assert.Equal(t, testCase.expectedResult, result)
		})
	}

}

func repeatedAction(action batch.ControlledJobActionHistoryEntry, n int) []batch.ControlledJobActionHistoryEntry {
	arr := make([]batch.ControlledJobActionHistoryEntry, n)
	for i := 0; i < n; i++ {
		arr[i] = action
	}
	return arr
}
