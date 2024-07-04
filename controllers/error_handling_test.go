package controllers

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/events"
)

func Test_handleError_RecordsAction(t *testing.T) {
	req := buildMockReq()
	controlledJob := buildMockControlledJob(req)
	mockHandler := buildMockHandler()
	sut := ControlledJobReconciler{
		EventHandler: mockHandler,
	}

	action := events.NewFailedAction(events.FailedToCalculateSchedule, errors.New("my error"))
	_, err := sut.handleError(context.Background(), req, controlledJob, action, false)
	assert.Nil(t, err, "should return no error")
	assert.Equal(t, 1, len(mockHandler.RecordEventCalls()), "should have recorded the event with the handler")
	assert.Equal(t, action, mockHandler.RecordEventCalls()[0].Action, "should have passed the correct event to the handler")
}

func Test_handleError_RequeuesIfRetryable(t *testing.T) {
	req := buildMockReq()
	controlledJob := buildMockControlledJob(req)
	sut := ControlledJobReconciler{
		EventHandler: buildMockHandler(),
	}

	testCases := []bool{true, false}

	for _, testCase := range testCases {

		result, err := sut.handleError(context.Background(), req, controlledJob, events.NewFailedAction(events.FailedToCalculateSchedule, errors.New("my error")), testCase)

		assert.Nil(t, err, "should return no error")

		assert.Equal(t, testCase, result.Requeue, "should requeue if error is retryable")
	}
}

func buildMockControlledJob(req ctrl.Request) *batch.ControlledJob {
	return &batch.ControlledJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}
}

func buildMockReq() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "ns",
			Name:      "name",
		},
	}
}

func buildMockHandler() *events.HandlerMock {
	return &events.HandlerMock{
		RecordEventFunc: func(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
		},
	}
}
