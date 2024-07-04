package events

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

func recordWarningEvent(ctx context.Context, recorder record.EventRecorder, obj runtime.Object, reason string, message string) {
	recordEvent(ctx, recorder, obj, corev1.EventTypeWarning, reason, message)
}

func recordNormalEvent(ctx context.Context, recorder record.EventRecorder, obj runtime.Object, reason string, message string) {
	recordEvent(ctx, recorder, obj, corev1.EventTypeNormal, reason, message)
}

func recordEvent(ctx context.Context, recorder record.EventRecorder, runtimeObj runtime.Object, eventType, reason, message string) {
	recorder.Event(runtimeObj, eventType, reason, message)
}
