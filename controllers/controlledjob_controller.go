/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/clientadapter"
	"github.com/G-Research/controlled-job/pkg/events"
	"github.com/G-Research/controlled-job/pkg/metadata"
	"github.com/G-Research/controlled-job/pkg/metrics"
	"github.com/G-Research/controlled-job/pkg/reconciliation"
	kbatch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ControlledJobReconciler reconciles a ControlledJob object
type ControlledJobReconciler struct {
	clientadapter.ControlledJobClient
	Scheme *runtime.Scheme
	Clock
	EventHandler events.Handler
}

/*
We'll mock out the clock to make it easier to jump around in time while testing,
the "real" clock just calls `time.Now`.
*/
type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() }

// clock knows how to get the current time.
// It can be used to fake out timing for testing.
type Clock interface {
	Now() time.Time
}

//+kubebuilder:rbac:groups=batch.gresearch.co.uk,resources=controlledjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.gresearch.co.uk,resources=controlledjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.gresearch.co.uk,resources=controlledjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ControlledJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ControlledJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	result := reconciliation.Reconcile(ctx, req.NamespacedName, r.Now(), r, r.EventHandler)

	log.FromContext(ctx).Info("reconcile complete", "req", req, "result", result)

	return result.AsControllerResultAndError()
}

func (r *ControlledJobReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	// set up a real clock, since we're not in a test
	if r.Clock == nil {
		r.Clock = realClock{}
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kbatch.Job{}, metadata.JobOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*kbatch.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a ControlledJob...
		if owner.APIVersion != metadata.ApiGVStr || owner.Kind != "ControlledJob" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&batch.ControlledJob{}).
		Owns(&kbatch.Job{}).
		Watches(&batch.ControlledJob{}, &metrics.Watcher{}).
		WithOptions(options).
		Complete(r)
}

func missedStartingDeadline(startOfCurrentPeriod time.Time, startingDeadlineSeconds *int64) bool {
	if startingDeadlineSeconds == nil {
		return false
	}
	return time.Since(startOfCurrentPeriod) > (time.Second * time.Duration(*startingDeadlineSeconds))
}
