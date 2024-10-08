// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package events

import (
	"context"
	batch "github.com/G-Research/controlled-job/api/v1"
	"sync"
)

// Ensure, that HandlerMock does implement Handler.
// If this is not the case, regenerate this file with moq.
var _ Handler = &HandlerMock{}

// HandlerMock is a mock implementation of Handler.
//
//	func TestSomethingThatUsesHandler(t *testing.T) {
//
//		// make and configure a mocked Handler
//		mockedHandler := &HandlerMock{
//			RecordEventFunc: func(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry)  {
//				panic("mock out the RecordEvent method")
//			},
//		}
//
//		// use mockedHandler in code that requires Handler
//		// and then make assertions.
//
//	}
type HandlerMock struct {
	// RecordEventFunc mocks the RecordEvent method.
	RecordEventFunc func(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry)

	// calls tracks calls to the methods.
	calls struct {
		// RecordEvent holds details about calls to the RecordEvent method.
		RecordEvent []struct {
			// Ctx is the ctx argument value.
			Ctx context.Context
			// ControlledJob is the controlledJob argument value.
			ControlledJob *batch.ControlledJob
			// Action is the action argument value.
			Action *batch.ControlledJobActionHistoryEntry
		}
	}
	lockRecordEvent sync.RWMutex
}

// RecordEvent calls RecordEventFunc.
func (mock *HandlerMock) RecordEvent(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
	if mock.RecordEventFunc == nil {
		panic("HandlerMock.RecordEventFunc: method is nil but Handler.RecordEvent was just called")
	}
	callInfo := struct {
		Ctx           context.Context
		ControlledJob *batch.ControlledJob
		Action        *batch.ControlledJobActionHistoryEntry
	}{
		Ctx:           ctx,
		ControlledJob: controlledJob,
		Action:        action,
	}
	mock.lockRecordEvent.Lock()
	mock.calls.RecordEvent = append(mock.calls.RecordEvent, callInfo)
	mock.lockRecordEvent.Unlock()
	mock.RecordEventFunc(ctx, controlledJob, action)
}

// RecordEventCalls gets all the calls that were made to RecordEvent.
// Check the length with:
//
//	len(mockedHandler.RecordEventCalls())
func (mock *HandlerMock) RecordEventCalls() []struct {
	Ctx           context.Context
	ControlledJob *batch.ControlledJob
	Action        *batch.ControlledJobActionHistoryEntry
} {
	var calls []struct {
		Ctx           context.Context
		ControlledJob *batch.ControlledJob
		Action        *batch.ControlledJobActionHistoryEntry
	}
	mock.lockRecordEvent.RLock()
	calls = mock.calls.RecordEvent
	mock.lockRecordEvent.RUnlock()
	return calls
}
