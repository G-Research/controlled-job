package events

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	batch "github.com/G-Research/controlled-job/api/v1"
)

func Test_recordWarningEvent(t *testing.T) {
	recorder := record.NewFakeRecorder(16)

	obj := &batch.ControlledJob{}

	recordWarningEvent(context.Background(), recorder, obj, string(FailedToListJobs), "this is my message")

	expected := fmt.Sprintf("%s %s %s", v1.EventTypeWarning, "FailedToListJobs", "this is my message")
	got := <-recorder.Events
	assert.Equalf(t, expected, got, "expected event %q, got %q", expected, got)
}

func Test_recordNormalEvent(t *testing.T) {
	recorder := record.NewFakeRecorder(16)

	obj := &batch.ControlledJob{}

	recordNormalEvent(context.Background(), recorder, obj, string(EventJobStarted), "this is my message")

	expected := fmt.Sprintf("%s %s %s", v1.EventTypeNormal, "JobStarted", "this is my message")
	got := <-recorder.Events
	assert.Equalf(t, expected, got, "expected event %q, got %q", expected, got)
}
