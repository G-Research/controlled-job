package events

import (
	"context"

	batch "github.com/G-Research/controlled-job/api/v1"
	"k8s.io/client-go/tools/record"
)

// Handler abstracts the handling of controller events
//
//go:generate moq -out handler_mock.go . Handler
type Handler interface {
	RecordEvent(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry)
}

func NewHandler(recorder record.EventRecorder) Handler {
	return &defaultHandler{
		recorder,
	}
}

var _ Handler = &defaultHandler{}

// Make a shadow of the record.EventRecorder type so we can mock it
//
//go:generate moq -out event_recorder_mock.go . EventRecorder
type EventRecorder = record.EventRecorder

type defaultHandler struct {
	recorder EventRecorder
}

func (h *defaultHandler) RecordEvent(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
	if IsWarningEvent(action.Type) {
		recordWarningEvent(ctx, h.recorder, controlledJob, action.Type, action.Message)
	} else {
		recordNormalEvent(ctx, h.recorder, controlledJob, action.Type, action.Message)
	}
	addActionHistoryEntryIgnoringDuplicates(ctx, controlledJob, action)
}
