package controllers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	batch "github.com/G-Research/controlled-job/api/v1"
)

func (r *ControlledJobReconciler) handleError(ctx context.Context, req ctrl.Request, controlledJob *batch.ControlledJob, failureAction *batch.ControlledJobActionHistoryEntry, retryable bool) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Failed to reconcile", "req", req, "failure", failureAction)
	r.EventHandler.RecordEvent(ctx, controlledJob, failureAction)

	// If the error is retryable (e.g. couldn't update status which is likely temporary), request a requeue so we can try to correct the error
	// The controller-runtime will handle backoff automatically so we don't spam ourselves with the same repeated error
	return reconcile.Result{
		Requeue: retryable,
	}, nil

}
