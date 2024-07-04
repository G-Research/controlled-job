package metrics

import (
	"context"
	"reflect"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	batch "github.com/G-Research/controlled-job/api/v1"
)

// Watcher implements handler.EventHandler so
// we can record metrics when ControlledJob resources
// are modified in the cluster
type Watcher struct{}

var _ handler.EventHandler = &Watcher{}

// Create implements handler.EventHandler
func (*Watcher) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	log.FromContext(ctx).Info("Got create event", "object", evt.Object)
	cj := evt.Object.(*batch.ControlledJob)
	ControlledJobInfo.WithLabelValues(ControlledJobInfoLabelValuesFor(cj)...).Set(1)
}

// Delete implements handler.EventHandler
func (*Watcher) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	log.FromContext(ctx).Info("Got delete event", "object", evt.Object)
	cj := evt.Object.(*batch.ControlledJob)
	ControlledJobInfo.DeleteLabelValues(ControlledJobInfoLabelValuesFor(cj)...)
}

// Generic implements handler.EventHandler
// We need to implement it to satisfy the interface, but we don't expect it to get called
// in practice, as it's used for external events like GitHub webhook events
func (*Watcher) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	log.FromContext(ctx).Info("Got generic event", "object", evt.Object)
}

// Update implements handler.EventHandler
func (*Watcher) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	log.FromContext(ctx).Info("Got update event", "objectOld", evt.ObjectOld, "objectNew", evt.ObjectNew)
	// Record the old ControlledJob being deleted and the new one being created
	cjOld := evt.ObjectOld.(*batch.ControlledJob)
	cjNew := evt.ObjectNew.(*batch.ControlledJob)
	oldLabelValues := ControlledJobInfoLabelValuesFor(cjOld)
	newLabelValues := ControlledJobInfoLabelValuesFor(cjNew)
	if !reflect.DeepEqual(oldLabelValues, newLabelValues) {
		// The labels have changed, we need to delete and recreate the metric
		ControlledJobInfo.DeleteLabelValues(oldLabelValues...)
		ControlledJobInfo.WithLabelValues(newLabelValues...).Set(1)
	}
}
