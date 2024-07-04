package clientadapter

import (
	"context"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/metadata"
	kbatch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerClientAdapter implements the ControlledJobClient interface
// by adapting the provided controller-runtime client
type ControllerClientAdapter struct {
	client.Client
}

// NewFromClient wraps the given controller-runtime client
// in our adapter
func NewFromClient(impl client.Client) ControlledJobClient {
	return &ControllerClientAdapter{
		impl,
	}
}

func (c *ControllerClientAdapter) GetControlledJob(ctx context.Context, namespacedName types.NamespacedName) (controlledJob *batch.ControlledJob, ok bool, err error) {
	controlledJob = &batch.ControlledJob{}
	err = c.Get(ctx, namespacedName, controlledJob)
	ok = err == nil
	err = client.IgnoreNotFound(err)
	return
}

func (c *ControllerClientAdapter) UpdateControlledJob(ctx context.Context, controlledJob *batch.ControlledJob) error {
	return c.Update(ctx, controlledJob)
}

func (c *ControllerClientAdapter) UpdateStatus(ctx context.Context, controlledJob *batch.ControlledJob) error {
	return c.Status().Update(ctx, controlledJob)
}

func (c *ControllerClientAdapter) ListJobsForControlledJob(ctx context.Context, namespacedName types.NamespacedName) (childJobs kbatch.JobList, err error) {
	err = c.List(ctx, &childJobs, client.InNamespace(namespacedName.Namespace), client.MatchingFields{metadata.JobOwnerKey: namespacedName.Name})
	return
}

func (c *ControllerClientAdapter) CreateJob(ctx context.Context, job *kbatch.Job) error {
	return c.Create(ctx, job)
}

func (c *ControllerClientAdapter) SuspendJob(ctx context.Context, job *kbatch.Job) error {
	t := true
	job.Spec.Suspend = &t
	return c.Update(ctx, job)
}

func (c *ControllerClientAdapter) UnsuspendJob(ctx context.Context, job *kbatch.Job) error {
	f := false
	job.Spec.Suspend = &f
	return c.Update(ctx, job)
}

func (c *ControllerClientAdapter) DeleteJob(ctx context.Context, job *kbatch.Job, propagation metav1.DeletionPropagation) error {
	err := c.Delete(ctx, job, client.PropagationPolicy(propagation))

	// we don't care if the job was already deleted
	err = client.IgnoreNotFound(err)
	return err
}
