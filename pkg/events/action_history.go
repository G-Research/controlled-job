package events

import (
	"context"
	"reflect"

	batch "github.com/G-Research/controlled-job/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const HistoryEntriesToKeep = 16

func addActionHistoryEntry(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
	addActionHistoryEntryImpl(ctx, controlledJob, action, false)
}

func addActionHistoryEntryIgnoringDuplicates(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry) {
	addActionHistoryEntryImpl(ctx, controlledJob, action, true)
}

func addActionHistoryEntryImpl(ctx context.Context, controlledJob *batch.ControlledJob, action *batch.ControlledJobActionHistoryEntry, ignoreDuplicates bool) {
	log := log.FromContext(ctx)

	if ignoreDuplicates && isSameAction(controlledJob.Status.MostRecentAction, action) {
		// Don't spam the status with multiple identical errors
		return
	}

	log.V(1).Info("recording action", "action", action)
	controlledJob.Status.MostRecentAction = action
	controlledJob.Status.ActionHistory = prependAndTruncateIfNeeded(controlledJob.Status.ActionHistory, *action, HistoryEntriesToKeep)
}

func isSameAction(existing, proposed *batch.ControlledJobActionHistoryEntry) bool {
	if existing == nil || proposed == nil {
		return false
	}

	// We need to clone the existing record so we can force the timestamps (that we don't care about) to match
	clone := existing.DeepCopy()
	clone.Timestamp = proposed.Timestamp

	return reflect.DeepEqual(clone, proposed)
}

func prependAndTruncateIfNeeded(existing []batch.ControlledJobActionHistoryEntry, newEntry batch.ControlledJobActionHistoryEntry, maxLength int) []batch.ControlledJobActionHistoryEntry {
	// Translation of the below:
	// create a new slice containing just the newEntry, and append the existing slice
	// existing[:min(len(existing), maxLength - 1)] gets a slice of the existing entries that is at most `maxLength - 1` entries long
	return append([]batch.ControlledJobActionHistoryEntry{newEntry}, existing[:min(len(existing), maxLength-1)]...)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
