package clientadapter

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kbatch "k8s.io/api/batch/v1"

	batch "github.com/G-Research/controlled-job/api/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ControlledJobClient is a facade interface to hide the complexity of the sigs.k8s.io/controller-runtime/pkg/client interface
// for our usages. To make it more injectable and mockable
//
//go:generate moq -out controlled_job_client_mock.go -rm . ControlledJobClient
type ControlledJobClient interface {
	// GetControlledJob gets the controlled job represented by the given
	// request (namespace and name).
	//
	// ok will be true if the controlled job was successfully found.
	//
	// If the given controlled job is not found that error will be
	// swallowed - ok false and a nil error will be returned.
	//
	// In all other error cases, ok false and the error will be returned.
	//
	// In other words, the ok result will be true if the controlledJob was
	// successfully found and will be false otherwise, whether err is nil or not
	GetControlledJob(ctx context.Context, namespacedName types.NamespacedName) (controlledJob *batch.ControlledJob, ok bool, err error)

	// UpdateControlledJob updates the specification of the given controlledjob in the cluster.
	//
	// It will return any error returned by the underlying implementation.
	UpdateControlledJob(ctx context.Context, controlledJob *batch.ControlledJob) error

	// UpdateStatus updates just the status of the given controlledjob in the cluster.
	//
	// It will return any error returned by the underlying implementation.
	UpdateStatus(ctx context.Context, controlledJob *batch.ControlledJob) error

	// ListJobsForControlledJob finds all jobs in the same namespace as namespacedName.Namespace
	// which are owned by the controlled job named namespacedName.Name.
	//
	// The implmentation takes care of the logic to determine which jobs are owned by the
	// ControlledJob.
	//
	// It will return any error returned by the underlying implementation.
	ListJobsForControlledJob(ctx context.Context, namespacedName types.NamespacedName) (kbatch.JobList, error)

	// CreateJob creates the given job on the cluster, which will then take care
	// of spinning up a Pod to run it and manage its runtime.
	//
	// It will return any error returned by the underlying implementation
	CreateJob(ctx context.Context, job *kbatch.Job) error

	// SuspendJob will mark a Job as suspended. This will cause the associated Pods to be killed with SIGTERM
	//
	// It will return any error returned by the underlying implementation
	SuspendJob(ctx context.Context, job *kbatch.Job) error

	// UnsuspendJob will resume a suspended job. This will cause a new Pod to be started up to run the Job
	//
	// It will return any error returned by the underlying implementation
	UnsuspendJob(ctx context.Context, job *kbatch.Job) error

	// DeleteJob removes the given job from the cluster, which will cause the
	// underlying pod (if any) to be killed.
	//
	// If the given job is not found that error will be
	// swallowed - a nil error will be returned.
	//
	// In all other error cases, the underlying error will be returned.
	//
	// The caller can choose the deletion propagation. Foreground will block until the underlying Pods have been
	// terminated and deleted before returning
	DeleteJob(ctx context.Context, job *kbatch.Job, propagation metav1.DeletionPropagation) error
}
