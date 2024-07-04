package reconciliation

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "github.com/G-Research/controlled-job/api/v1"
	v1 "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/clientadapter"
	"github.com/G-Research/controlled-job/pkg/events"
	"github.com/pkg/errors"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PossiblyRetryableError struct {
	Error       error
	IsRetryable bool
}

// ReconcileResult encapsulates the possible exit conditions of the Reconcile method:
// - We processed successfully and want to be requeued after a certain amount of time
// - There was an error, and it's retryable, so we want to be requeued immediately
// - There was an error, but it's not something we can retry (e.g. user error) so we don't want to be requeued
type ReconcileResult struct {
	RequeueAfter time.Duration
	Error        *PossiblyRetryableError
}

type ReconcileOptions struct {
	EnableAutoRecreateJobsOnSpecChange bool
}

var (
	Options = &ReconcileOptions{
		EnableAutoRecreateJobsOnSpecChange: false,
	}
)

// AsControllerResultAndError maps a ReconcileResult to a form that we can
// return to the controller-runtime
func (r ReconcileResult) AsControllerResultAndError() (ctrl.Result, error) {
	if r.Error != nil {
		if r.Error.IsRetryable {
			return ctrl.Result{}, r.Error.Error
		}
		// Non-retryable error - don't return an error (or we'll be retried)
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: r.RequeueAfter}, nil
}

func TransientErrorResult(err error) ReconcileResult {
	return ReconcileResult{
		Error: &PossiblyRetryableError{
			Error:       err,
			IsRetryable: true,
		},
	}
}

func NonRetryableErrorResult(err error) ReconcileResult {
	return ReconcileResult{
		Error: &PossiblyRetryableError{
			Error:       err,
			IsRetryable: false,
		},
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func Reconcile(ctx context.Context, target types.NamespacedName, now time.Time, client clientadapter.ControlledJobClient, eventHandler events.Handler) ReconcileResult {
	var controlledJob *batch.ControlledJob
	var childJobs *kbatch.JobList
	var err error

	// If we get an error during the run, then try to record it
	defer func() {
		recordFailedReconcile(ctx, controlledJob, err, eventHandler)
	}()

	controlledJob, childJobs, err = loadFromCluster(ctx, target, client)
	if err != nil {
		return TransientErrorResult(err)
	}
	if controlledJob == nil {
		// No controlled job found, nothing to do
		return ReconcileResult{}
	}
	defer func() {
		// Now we've processed the reconciliation, update the status of the controlledJob
		calculateOverallConditions(controlledJob, err)
		if updateErr := client.UpdateStatus(ctx, controlledJob); updateErr != nil {
			log.FromContext(ctx).Error(updateErr, "failed to update status", "name", controlledJob.Name, "namespace", controlledJob.Namespace)
		}
	}()

	decision, err := makeDecision(ctx, controlledJob, childJobs, now, Options.EnableAutoRecreateJobsOnSpecChange)
	if err != nil {
		// Don't requeue, as a failure to build state is (likely) a user error and we need to
		// wait for them to fix it.
		// In any case, it is a pure function in respect of its inputs and
		// so there's no point retrying it until some external change occurs
		// at which point we'll be requeued by runtime.
		return NonRetryableErrorResult(err)
	}

	for i := range decision.JobsToDelete {
		job := decision.JobsToDelete[i]
		err = client.DeleteJob(ctx, job, metav1.DeletePropagationForeground)
		if err != nil {
			err = events.WrapError(err, events.FailedToDeleteJob, fmt.Sprintf("failed to delete job %s in namespace %s", job.Name, job.Namespace))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToDeleteJob, metav1.ConditionTrue, "FailedToDeleteJob", err.Error())
			return TransientErrorResult(err)
		} else {
			eventHandler.RecordEvent(ctx, controlledJob, events.NewJobStoppedAction(job.Name))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToDeleteJob, metav1.ConditionFalse, "DeletedJob", "Successfully deleted job")
		}
	}

	for i := range decision.JobsToCreate {
		job := decision.JobsToCreate[i]
		err = client.CreateJob(ctx, job)
		if err != nil {
			err = events.WrapError(err, events.FailedToCreateJob, fmt.Sprintf("failed to create job %s in namespace %s", job.Name, job.Namespace))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToCreateJob, metav1.ConditionTrue, "FailedToCreateJob", err.Error())
			return TransientErrorResult(err)
		} else {
			eventHandler.RecordEvent(ctx, controlledJob, events.NewJobStartedAction(job.Name))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToCreateJob, metav1.ConditionFalse, "CreatedJob", "Successfully created job")
		}
	}

	for i := range decision.JobsToSuspend {
		job := decision.JobsToSuspend[i]
		err = client.SuspendJob(ctx, job)
		if err != nil {
			err = events.WrapError(err, events.FailedToSuspendJob, fmt.Sprintf("failed to suspend job %s in namespace %s", job.Name, job.Namespace))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToSuspendJob, metav1.ConditionTrue, "FailedToSuspendJob", err.Error())
			return TransientErrorResult(err)
		} else {
			eventHandler.RecordEvent(ctx, controlledJob, events.NewJobSuspendedAction(job.Name))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToSuspendJob, metav1.ConditionTrue, "SuspendedJob", "Successfully suspended job")
		}
	}
	for i := range decision.JobsToUnsuspend {
		job := decision.JobsToUnsuspend[i]
		err = client.UnsuspendJob(ctx, job)
		if err != nil {
			err = events.WrapError(err, events.FailedToUnsuspendJob, fmt.Sprintf("failed to unsuspend job %s in namespace %s", job.Name, job.Namespace))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToUnsuspendJob, metav1.ConditionTrue, "FailedToUnsuspendJob", err.Error())
			return TransientErrorResult(err)
		} else {
			eventHandler.RecordEvent(ctx, controlledJob, events.NewJobUnsuspendedAction(job.Name))
			v1.SetCondition(controlledJob, v1.ConditionTypeFailedToUnsuspendJob, metav1.ConditionTrue, "UnsuspendedJob", "Successfully unsuspended job")
		}
	}

	return ReconcileResult{RequeueAfter: decision.RequeueAt.Sub(now)}
}

// calculateOverallConditions calculates some useful second-order conditions, based on other conditions on the ControlledJob. For example
// NotRunningUnexpectedly can be used by users to alert if the job should be running but isn't
func calculateOverallConditions(controlledJob *v1.ControlledJob, err error) {

	if err != nil {
		batch.SetCondition(controlledJob, batch.ConditionTypeError, metav1.ConditionTrue, "Error", err.Error())
	} else {
		batch.SetCondition(controlledJob, batch.ConditionTypeError, metav1.ConditionFalse, "NoError", "No error encountered")
	}

	jobExists := batch.CoerceConditionToBoolen(batch.FindCondition(controlledJob.Status, v1.ConditionTypeJobExists))
	jobFailed := batch.CoerceConditionToBoolen(batch.FindCondition(controlledJob.Status, v1.ConditionTypeJobFailed))
	jobExistsAndNotFailed := jobExists && !jobFailed

	shouldBeRunning := batch.CoerceConditionToBoolen(batch.FindCondition(controlledJob.Status, batch.ConditionTypeShouldBeRunning))
	jobManuallyScheduled := batch.CoerceConditionToBoolen(batch.FindCondition(controlledJob.Status, batch.ConditionTypeJobManuallyScheduled))

	// Each of the four conditions are either True if their conditions match, or False with CannotDetermine otherwise (because none
	// of them are strict binary conditions)

	// We need to make sure we set each of the 4 conditions exactly once. Otherwise, we will flip one of them between unknown and true, and
	// that will cause a new transition timestamp, which will cause a change to the status object, which will requeue this resource onto the queue
	// forever.

	// To reduce line noise in the logic below, we define some helper functions to set each of the four conditions to either Unknown of True
	runningExpectedlyIsUnknown := func() {
		batch.SetCondition(controlledJob, batch.ConditionTypeRunningExpectedly, metav1.ConditionUnknown, "CannotDetermine",
			"Job either not running, or it is running and we don't expect it to be (see other conditions for details)")
	}
	runningExpectedlyBecause := func(reason, message string) {
		batch.SetCondition(controlledJob, batch.ConditionTypeRunningExpectedly, metav1.ConditionTrue, reason, message)
	}

	runningUnexpectedlyIsUnknown := func() {
		batch.SetCondition(controlledJob, batch.ConditionTypeRunningUnexpectedly, metav1.ConditionUnknown, "CannotDetermine",
			"Job either not running, or it is running and we expect it to be (see other conditions for details)")
	}
	runningUnexpectedlyBecause := func(reason, message string) {
		batch.SetCondition(controlledJob, batch.ConditionTypeRunningUnexpectedly, metav1.ConditionTrue, reason, message)
	}

	notRunningExpectedlyIsUnknown := func() {
		batch.SetCondition(controlledJob, batch.ConditionTypeNotRunningExpectedly, metav1.ConditionUnknown, "CannotDetermine",
			"Job either running, or it's not running but we expect it to be (see other conditions for details)")
	}
	notRunningExpectedlyBecause := func(reason, message string) {
		batch.SetCondition(controlledJob, batch.ConditionTypeNotRunningExpectedly, metav1.ConditionTrue, reason, message)
	}

	notRunningUnexpectedlyIsUnknown := func() {
		batch.SetCondition(controlledJob, batch.ConditionTypeNotRunningUnexpectedly, metav1.ConditionUnknown, "CannotDetermine",
			"Job either not running, or it is running and we expect it to be (see other conditions for details)")
	}
	notRunningUnexpectedlyBecause := func(reason, message string) {
		batch.SetCondition(controlledJob, batch.ConditionTypeNotRunningUnexpectedly, metav1.ConditionTrue, reason, message)
	}

	// Now override that to True for the case(s) that do hold
	if jobExistsAndNotFailed {
		// JOB IS RUNNING
		if shouldBeRunning {
			runningExpectedlyBecause("RunningBasedOnSchedule", "Job is running, and that's expected because of the schedule")
			runningUnexpectedlyIsUnknown()
			notRunningExpectedlyIsUnknown()
			notRunningUnexpectedlyIsUnknown()
		} else if jobManuallyScheduled {
			runningExpectedlyBecause("RunningManually", "Job is running, and that's expected because the user has manually scheduled a job")
			runningUnexpectedlyIsUnknown()
			notRunningExpectedlyIsUnknown()
			notRunningUnexpectedlyIsUnknown()
		} else {
			// AND IT SHOULDN'T BE
			runningExpectedlyIsUnknown()
			runningUnexpectedlyBecause("RunningUnexpectedly", "Job is running, but it should not be - neither inside the scheduled times, nor manually scheduled")
			notRunningExpectedlyIsUnknown()
			notRunningUnexpectedlyIsUnknown()
		}
	} else {
		// JOB IS NOT RUNNING
		if shouldBeRunning {
			if jobFailed {
				runningExpectedlyIsUnknown()
				runningUnexpectedlyIsUnknown()
				notRunningExpectedlyIsUnknown()
				notRunningUnexpectedlyBecause("JobFailed", "Job failed, but according to the schedule it should currently be running")
			} else {
				runningExpectedlyIsUnknown()
				runningUnexpectedlyIsUnknown()
				notRunningExpectedlyIsUnknown()
				notRunningUnexpectedlyBecause("NoJobExists", "No job currently exists, but according to the schedule it should currently be running")
			}
		} else if jobManuallyScheduled {
			if jobFailed {
				runningExpectedlyIsUnknown()
				runningUnexpectedlyIsUnknown()
				notRunningExpectedlyIsUnknown()
				notRunningUnexpectedlyBecause("JobFailed", "Job failed after the user scheduled it")
			} else {
				// Fairly sure this case can never be hit, as we determine if the job is manually scheduled from the job itself...
				// But, no harm including it for completeness
				runningExpectedlyIsUnknown()
				runningUnexpectedlyIsUnknown()
				notRunningExpectedlyIsUnknown()
				notRunningUnexpectedlyBecause("NoJobExists", "The user requested to schedule a Job, but one hasn't been created")
			}
		} else {
			// AND IT SHOULDN'T BE
			runningExpectedlyIsUnknown()
			runningUnexpectedlyIsUnknown()
			notRunningExpectedlyBecause("NotRunningExpectedly", "No job exists, and that's expected as we're outside the scheduled times, and the user has not manually scheduled a job")
			notRunningUnexpectedlyIsUnknown()
		}
	}
}

func loadFromCluster(ctx context.Context, target types.NamespacedName, client clientadapter.ControlledJobClient) (*batch.ControlledJob, *kbatch.JobList, error) {
	log := log.FromContext(ctx)

	// Load details of the controlled job pointed at by target
	var controlledJob *batch.ControlledJob
	var err error
	var ok bool
	if controlledJob, ok, err = client.GetControlledJob(ctx, target); !ok {
		if controlledJob == nil {
			// This is the only time we log an error in this function, because we swallow the error and don't return it
			// In other cases the error will be logged by the caller (or its caller) when it sees the error
			log.Info("Received reconcile request, but could not find target ControlledJob. Assuming it's been deleted.", "target", target, "err", err)
			return nil, nil, nil
		}
		return nil, nil, errors.Wrap(err, fmt.Sprintf("failed to find target ControlledJob %s in namespace %s", target.Name, target.Namespace))
	}

	// Load details of the jobs associated with the target ControlledJob
	jobList, err := client.ListJobsForControlledJob(ctx, target)
	if err != nil {
		return nil, nil, events.WrapError(err, events.FailedToListJobs, fmt.Sprintf("Failed to list jobs for controlled job %s in namespace %s", controlledJob.Name, controlledJob.Namespace))
	}

	return controlledJob, &jobList, err
}

func recordFailedReconcile(ctx context.Context, controlledJob *batch.ControlledJob, err error, eventHandler events.Handler) {
	if err == nil {
		// No error
		return
	}
	log.FromContext(ctx).Error(err, "failed to reconcile")

	// If we failed before loading the state, or did not manage to load the controlled job
	// then we have nothing to record the error against
	if controlledJob == nil {
		return
	}

	// Try to extract the specific event that failed from the error
	// Fallback to the generic 'FailedToReconcile' if there's no specific error
	event := events.FailedToReconcile
	var errWithEvent *events.ErrorWithEvent
	if ok := errors.As(err, &errWithEvent); ok {
		event = errWithEvent.Event()
	}
	eventHandler.RecordEvent(ctx, controlledJob, events.NewFailedAction(event, err))
}
