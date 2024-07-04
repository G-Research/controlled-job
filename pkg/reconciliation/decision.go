package reconciliation

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"

	v1 "github.com/G-Research/controlled-job/api/v1"
	jobpkg "github.com/G-Research/controlled-job/pkg/job"
	"github.com/G-Research/controlled-job/pkg/metadata"
	"github.com/G-Research/controlled-job/pkg/schedule"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	kbatch "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Decision struct {
	JobsToCreate    []*kbatch.Job
	JobsToDelete    []*kbatch.Job
	JobsToSuspend   []*kbatch.Job
	JobsToUnsuspend []*kbatch.Job
	RequeueAt       time.Time
}

func (d *Decision) AddToLog(log logr.Logger) logr.Logger {

	jobsToCreate := make([]string, len(d.JobsToCreate))
	for i, job := range d.JobsToCreate {
		jobsToCreate[i] = job.Name
	}
	jobsToDelete := make([]string, len(d.JobsToDelete))
	for i, job := range d.JobsToDelete {
		jobsToDelete[i] = job.Name
	}
	return log.
		WithValues("jobsToCreate", jobsToCreate).
		WithValues("jobsToDelete", jobsToDelete).
		WithValues("requeueAt", d.RequeueAt)
}

func makeDecision(ctx context.Context, controlledJob *v1.ControlledJob, childJobs *kbatch.JobList, now time.Time, enableAutoRecreateJobsOnSpecChange bool) (decision Decision, err error) {
	var state *state
	state, err = buildState(ctx, controlledJob, childJobs, now)
	if err != nil {
		return
	}

	log := log.FromContext(ctx).
		WithValues("controlledJob", controlledJob.Name).
		WithValues("namespace", controlledJob.Namespace).
		WithValues("childJobsCount", len(state.AllJobs))

	controlledJob.Status.IsSuspended = &state.IsSuspended
	var lastScheduledStateTime *metav1.Time = nil
	if state.StartOfCurrentRunPeriod != nil {
		t := metav1.NewTime(*state.StartOfCurrentRunPeriod)
		lastScheduledStateTime = &t
	}
	controlledJob.Status.LastScheduledStartTime = lastScheduledStateTime

	setShouldBeRunningStatus(controlledJob, state)
	shouldBeRunning := false
	if state.ShouldBeRunning != nil {
		shouldBeRunning = *state.ShouldBeRunning
	}

	// Are we suspended?
	if state.IsSuspended {
		log.V(1).Info("ControlledJob is suspended, deleting any running jobs")
		decision.JobsToDelete = state.AllJobs
		v1.SetCondition(controlledJob, v1.ConditionTypeSuspended, metav1.ConditionTrue, "Suspended", "IsSuspended flag set")
		// We're suspended, so nothing more to do
		return
	}
	v1.SetCondition(controlledJob, v1.ConditionTypeSuspended, metav1.ConditionFalse, "NotSuspended", "IsSuspended flag not set")

	// Gate restart on spec changes on both env var and spec change policy
	restartOnSpecChange := enableAutoRecreateJobsOnSpecChange && state.AutoRestartIsEnabled

	// record a high watermark for the job run id, so we know what new job run id to use if we end up creating a new
	// job
	maxJobRunId := -1

	// Some conditions we want to record based on a single job - it would be confusing to record a 'JobFailed' condition if a previous job failed, but the currently
	// running one is in progress
	var jobToRecordMetricsAgainst *kbatch.Job = nil

	for _, job := range state.AllJobs {
		runId, err := metadata.GetJobRunId(job)
		if err != nil {
			// For now, skip if we can't read the run id
			continue
		}
		if runId > maxJobRunId {
			maxJobRunId = runId
		}
		if jobToRecordMetricsAgainst == nil || isBetterCandidateJob(job, jobToRecordMetricsAgainst, state) {
			jobToRecordMetricsAgainst = job
		}
	}

	setConditionsForAllJobs(controlledJob, state.AllJobs)
	setJobConditions(controlledJob, jobToRecordMetricsAgainst)

	// The chosenJob is the single job (if any) which is allowed to be running. Any other jobs will be deleted
	// We try hard to make this stable and avoid unnecessary restarts. For example, ties are decided deterministically
	// based on job name, and jobs which are already running with the correct spec are preferred over ones running with
	// the wrong spec
	var chosenJob *kbatch.Job = nil
	defer func() {
		isRunning := chosenJob != nil
		controlledJob.Status.IsRunning = &isRunning
	}()
	numberOfPotentiallyRunningJobs := 0
	expiredJobs := []*kbatch.Job{}
	nonExpiredJobs := []*kbatch.Job{}
	controlledJob.Status.Active = make([]corev1.ObjectReference, 0)
	for _, job := range state.AllJobs {

		// Decide what to do with this job (if anything) and whether it's the chosenJob
		if metadata.IsJobPotentiallyRunning(job) {
			numberOfPotentiallyRunningJobs++
			scheme := runtime.NewScheme()
			_ = kbatch.AddToScheme(scheme)
			objectReference, err := reference.GetReference(scheme, job)
			if err == nil && objectReference != nil {
				controlledJob.Status.Active = append(controlledJob.Status.Active, *objectReference)
			}
		}

		/*
		 * Is the job expired (we've passed its stop time)?
		 *
		 * If the job's start time (scheduled-at annotation) is before the
		 * most recent stop time in the schedule, then it is expired and needs
		 * to be deleted
		 */
		if state.LastStopTime == nil {
			log.V(1).Info("ControlledJob has no recent stop events, so job will not be marked as expired")
		} else {
			jobStartTime, err := metadata.GetScheduledTime(job)
			if err != nil {
				err = errors.Wrap(err, "Could not determine start time of job - this is invalid and should not happen. Will delete it.")
				log.V(1).Error(err, "", "job", job.Name)
				expiredJobs = append(expiredJobs, job)
				continue
			}
			if jobStartTime.Before(*state.LastStopTime) {
				log.V(1).Info("Job is expired. Will delete it.", "job", job.Name, "jobStartTime", jobStartTime, "lastScheduledStopTime", state.LastStopTime)
				expiredJobs = append(expiredJobs, job)
				continue
			}
		}

		/*
		 * 2. Job is not expired, so it's a candidate to be the chosen job
		 */
		nonExpiredJobs = append(nonExpiredJobs, job)
		if chosenJob == nil || isBetterCandidateJob(job, chosenJob, state) {
			chosenJob = job
		}
	}

	/*
	 * Job is current, but the schedule says we shouldn't be running
	 *
	 * In this case we delete the job, unless it is marked as manually scheduled
	 * in which case the user has decided to run the job outside of its schedule
	 * so let them
	 */
	shouldBeStopped := !shouldBeRunning
	isNotManuallyScheduled := chosenJob != nil && !metadata.IsManuallyScheduledJob(chosenJob)
	if shouldBeStopped && isNotManuallyScheduled {
		log.V(1).Info("We expect to be stopped but found a non-manually scheduled job. Will delete it", "job", chosenJob.Name)
		// Setting chosenJob to nil means that this job will be added to the JobsToDelete list at the end of this method
		chosenJob = nil
	}

	/*
	 * Job is out of date (it's spec no longer matches the controlled job template)
	 */
	isOutOfDate := chosenJob != nil && isOutOfDate(chosenJob, state)
	outOfDateReason := ""
	outOfDateMessage := ""
	if chosenJob == nil {
		outOfDateReason = "NoRunningJob"
		outOfDateMessage = "Not currently running"
	} else {
		outOfDateReason = "NotOutOfDate"
		outOfDateMessage = "Running job matches desired spec"
	}
	if isOutOfDate {
		if !metadata.IsJobPotentiallyRunning(chosenJob) || metadata.WasJobStoppedByTheUser(chosenJob) {
			log.V(1).Info("Job is out of date, but is not running so ignoring",
				"job", chosenJob.Name)
			outOfDateReason = "JobIsNotRunning"
			outOfDateMessage = "Job is out of date, but is not running so ignoring"
		} else if metadata.IsJobBeingDeleted(chosenJob) {
			// This is a bit of a subtle edge case. If the job is being deleted, but has an out of date spec, then we
			// should _not_ recreate it, because the most likely situation is that the user has issued a stop request
			// not a restart request, and so would be surprised if the job then started back up underneath them.
			log.V(1).Info("Job is out of date, but is being deleted so ignoring",
				"job", chosenJob.Name)
			outOfDateReason = "JobIsBeingDeleted"
			outOfDateMessage = "Job is out of date, but is being deleted so ignoring"
		} else if !restartOnSpecChange {
			log.V(1).Info("Job is out of date, but auto-recreation is not enabled so will leave it running as is",
				"job", chosenJob.Name,
				"enabledOnControlledJob", state.AutoRestartIsEnabled,
				"enabledGloballyInOperator", enableAutoRecreateJobsOnSpecChange)
			outOfDateReason = "ShouldNotAutoRestart"
			outOfDateMessage = "Job is out of date, but auto-recreation is not enabled so will leave it running as is"
		} else {
			log.V(1).Info("Job is out of date, will recreate it with the latest spec", "job", chosenJob.Name)

			newJob, e := jobpkg.RecreateJobWithNewSpec(ctx, chosenJob, controlledJob, maxJobRunId+1, true)
			if e != nil {
				err = errors.Wrap(e, "Failed to create job")
				return
			}
			decision.JobsToCreate = append(decision.JobsToCreate, newJob)
			numberOfPotentiallyRunningJobs++
			nonExpiredJobs = append(nonExpiredJobs, newJob)
			chosenJob = newJob
			// Given we're recreating the job, we can mark ourselves not out of date
			isOutOfDate = false
		}
	}
	outOfDateStatus := metav1.ConditionFalse
	if isOutOfDate {
		outOfDateStatus = metav1.ConditionTrue
	}
	v1.SetCondition(controlledJob, v1.ConditionTypeOutOfDate, outOfDateStatus, outOfDateReason, outOfDateMessage)

	// If we should be running according to the schedule, make sure we actually are.
	//
	// Note we only care about whether a job exists, we don't care if they're actually running or completed/suspended/failed. This is for
	// various reasons:
	// - When a job completes within its run period, we consider that expected and don't want to automatically restart it
	// - Users don't necessarily want failing jobs to keep restarting themselves, in case that causes issues (we can't know what their code does)
	// - Users need a way to stop a ControlledJob for a period. This is achieved by suspending a running job. In that case we
	//   don't want to treat that as 'not running' or we'd immediately restart a stopped ControlledJob!
	//
	// Users do already have external control over what happens when their jobs fail or complete during scheduled hours:
	// - They can use restartPolicy: OnFailure, and backoffLimit in their jobTemplate spec in order to auto restart on pod failure
	// - They can monitor the status of their jobs in Prometheus and alert if jobs are not running when they should
	// - They can use application level monitoring to ensure their system has the correct number of running instances
	if shouldBeRunning && chosenJob == nil {
		if startingDeadlineSecondsExceeded(controlledJob, state.StartOfCurrentRunPeriod, now) {
			log.V(1).Info("We expect to be running, but there is no job")
			v1.SetCondition(controlledJob, v1.ConditionTypeStartingDeadlineExceeded, metav1.ConditionTrue, "StartingDeadlineExceeded", "We expect to be running, but have exceeded the starting deadline")
			err = errors.New("Tried to create a job, but we have exceeded the specified StartingDeadlineSeconds after the scheduled start time")
			return
		} else {
			v1.SetCondition(controlledJob, v1.ConditionTypeStartingDeadlineExceeded, metav1.ConditionFalse, "StartingDeadlineNotExceeded", "Still in time to start a new job")
		}

		log.V(1).Info("We expect to be running, but there is no job, so will create one")
		if state.StartOfCurrentRunPeriod == nil {
			err = errors.New("Tried to create a job, but the StartOfCurrentRunPeriod is nil, which was unexpected")
			return
		}
		newJob, e := jobpkg.BuildForControlledJob(ctx, controlledJob, *state.StartOfCurrentRunPeriod, 0, false, true)
		if e != nil {
			err = errors.Wrap(e, "Failed to create job")
			return
		}
		decision.JobsToCreate = append(decision.JobsToCreate, newJob)
		numberOfPotentiallyRunningJobs++
		nonExpiredJobs = append(nonExpiredJobs, newJob)
		chosenJob = newJob
	} else {
		v1.SetCondition(controlledJob, v1.ConditionTypeStartingDeadlineExceeded, metav1.ConditionUnknown, "NoNewJobRequired", "We're not trying to start a job at the moment")
	}

	/*
	 *	Make sure everything but the chosenJob is either deleted or completed
	 *	We allow multiple completed jobs, because when users start and stop jobs we want to allow them
	 *	to see previous runs that day in k8s using kubectl get jobs
	 */
	// All expired jobs get deleted
	for _, job := range expiredJobs {
		if metadata.IsJobBeingDeleted(job) {
			continue
		}
		decision.JobsToDelete = append(decision.JobsToDelete, job)
	}

	// Non-expired jobs that aren't the chosen job and aren't completed get deleted
	for _, job := range nonExpiredJobs {
		if metadata.IsJobBeingDeleted(job) {
			continue
		}

		if job == chosenJob || metadata.IsJobCompleted(job) {
			continue
		}
		decision.JobsToDelete = append(decision.JobsToDelete, job)
	}

	/*
	 * Finally, if we're sure there's only one job that's not completed, and it's suspended
	 * then we're safe to unsuspend it.
	 *
	 * The reason for this paranoia is that for jobs that don't have a definite completion condition
	 * we can't know that they don't have a Pod running under the hood, and our contract states that we
	 * must only allow at most one Pod to be running at any time.
	 *
	 * So the only way to guarantee that is to ensure that Jobs are always created in a suspended state, and when
	 * we are certain it's the only job, unsuspend them
	 */
	if numberOfPotentiallyRunningJobs == 1 && // there's exactly one running job
		chosenJob != nil && // we want to be running a job (i.e. we're not in a stopped state)
		metadata.IsJobSuspended(chosenJob) && // the job we want to run is suspended, but...
		!metadata.IsJobBeingDeleted(chosenJob) && // ... not being deleted, and ...
		!metadata.WasJobStoppedByTheUser(chosenJob) { // ... wasn't explicitly stopped by the user

		// Simplify things. If we're about to _create_ this job and we want to unsuspend it, don't do both, just clear
		// the suspend flag before creating it
		if len(decision.JobsToCreate) == 1 && decision.JobsToCreate[0] == chosenJob {
			chosenJob.Spec.Suspend = nil
		} else {
			decision.JobsToUnsuspend = append(decision.JobsToUnsuspend, chosenJob)
		}
	}

	// Set requeue at next event time
	if state.NextEventTime != nil {
		decision.RequeueAt = *state.NextEventTime
	}

	decision.AddToLog(log).V(1).Info("Made decision")

	return
}

func setShouldBeRunningStatus(controlledJob *v1.ControlledJob, state *state) {
	shouldBeRunningStatus := metav1.ConditionUnknown
	shouldBeRunningReason := ""
	shouldBeRunningMessage := ""
	if state.ShouldBeRunning != nil {
		controlledJob.Status.ShouldBeRunning = state.ShouldBeRunning
		if *state.ShouldBeRunning {
			shouldBeRunningStatus = metav1.ConditionTrue
			shouldBeRunningReason = "InsideRunPeriod"
			shouldBeRunningMessage = "Currently between a start and stop time in the schedule"
		} else {
			shouldBeRunningStatus = metav1.ConditionFalse
			shouldBeRunningReason = "OutsideRunPeriod"
			shouldBeRunningMessage = "Currently outside of a start and stop time in the schedule"
		}
	} else {
		// To retain compatibility with old behaviour, we don't ever set shouldBeRunning to null
		no := false
		controlledJob.Status.ShouldBeRunning = &no

		shouldBeRunningReason = "NoStartEvent"
		shouldBeRunningMessage = "No start events defined"
	}

	v1.SetCondition(controlledJob, v1.ConditionTypeShouldBeRunning, shouldBeRunningStatus, shouldBeRunningReason, shouldBeRunningMessage)
}

func setConditionsForAllJobs(controlledJob *v1.ControlledJob, allJobs []*kbatch.Job) {

	if len(allJobs) == 0 {
		v1.SetCondition(controlledJob, v1.ConditionTypeJobManuallyScheduled, metav1.ConditionUnknown, "NoCurrentJob", "No jobs")
		v1.SetCondition(controlledJob, v1.ConditionTypeJobBeingDeleted, metav1.ConditionUnknown, "NoCurrentJob", "No jobs")
	} else {

		setConditionIfTrueForAnyJob(controlledJob, v1.ConditionTypeJobManuallyScheduled, allJobs, metadata.IsManuallyScheduledJob,
			"CreatedByUser", "The user has scheduled a job manually",
			"NoManuallyScheduledJobs", "There are no manually scheduled jobs",
		)

		setConditionIfTrueForAnyJob(controlledJob, v1.ConditionTypeJobBeingDeleted, allJobs, metadata.IsJobBeingDeleted,
			"JobBeingDeleted", "A job is being deleted",
			"JobNotBeingDeleted", "No jobs are being deleted")
	}
}

func setJobConditions(controlledJob *v1.ControlledJob, job *kbatch.Job) {
	if job == nil {
		v1.SetCondition(controlledJob, v1.ConditionTypeJobExists, metav1.ConditionFalse, "NoCurrentJob", "No jobs")
		// Set everything else to unknown
		v1.SetCondition(controlledJob, v1.ConditionTypeJobRunning, metav1.ConditionUnknown, "NoCurrentJob", "No jobs")
		v1.SetCondition(controlledJob, v1.ConditionTypeJobComplete, metav1.ConditionUnknown, "NoCurrentJob", "No jobs")
		v1.SetCondition(controlledJob, v1.ConditionTypeJobFailed, metav1.ConditionUnknown, "NoCurrentJob", "No jobs")
		v1.SetCondition(controlledJob, v1.ConditionTypeJobSuspended, metav1.ConditionUnknown, "NoCurrentJob", "No jobs")
		v1.SetCondition(controlledJob, v1.ConditionTypeJobStoppedByUser, metav1.ConditionUnknown, "NoCurrentJob", "No jobs")
	} else {

		// We definitely have a job
		v1.SetCondition(controlledJob, v1.ConditionTypeJobExists, metav1.ConditionTrue, "JobExists", "At least one job exists")

		// Is it running? i.e. not complete and has at least one ready pod?
		if metadata.JobHasReadyStatus(job) {
			v1.SetConditionBasedOnFlag(controlledJob, v1.ConditionTypeJobRunning, metadata.IsJobRunning(job),
				"ReadyCountSufficient", "Job has the expected number of ready pods and hasn't completed",
				"ReadyCountNotSufficient", "Job does not yet have the expected number of ready pods (or it's completed). It could be struggling to start",
			)
		} else {
			v1.SetCondition(controlledJob, v1.ConditionTypeJobRunning, metav1.ConditionUnknown, "CannotDetermine", "Job has no ready status, so we can't determine if it's running")
		}

		// proxy the JobComplete and JobFailed conditions from the Job itself
		completeCondition := metadata.GetJobCondition(job, kbatch.JobComplete)
		if completeCondition != nil {
			v1.SetCondition(controlledJob, v1.ConditionTypeJobComplete, metav1.ConditionStatus(completeCondition.Status), v1.JobConditionToReason(*completeCondition, "JobComplete"), "Relaying JobComplete status from the Job")
		} else {
			v1.SetCondition(controlledJob, v1.ConditionTypeJobComplete, metav1.ConditionUnknown, "JobCompleteUnkown", "Job has not reported a complete condition yet")
		}

		failedCondition := metadata.GetJobCondition(job, kbatch.JobFailed)
		if failedCondition != nil {
			v1.SetCondition(controlledJob, v1.ConditionTypeJobFailed, metav1.ConditionStatus(failedCondition.Status), v1.JobConditionToReason(*failedCondition, "JobFailed"), "Relaying JobFailed status from the Job")
		} else {
			v1.SetCondition(controlledJob, v1.ConditionTypeJobFailed, metav1.ConditionUnknown, "JobFailedUnkown", "Job has not reported a failed condition yet")
		}

		// It could be suspended if we're waiting for a previous job to stop
		v1.SetConditionBasedOnFlag(controlledJob, v1.ConditionTypeJobSuspended, metadata.IsJobSuspended(job),
			"JobSuspended", "The current job is suspended",
			"JobNotSuspended", "The current job is not suspended")
		v1.SetConditionBasedOnFlag(controlledJob, v1.ConditionTypeJobStoppedByUser, metadata.WasJobStoppedByTheUser(job),
			"JobStoppedByUser", "The current job was manually stopped by a user",
			"JobNotStoppedByUser", "The current job was not stopped by a user")
	}
}

func setConditionIfTrueForAnyJob(controlledJob *v1.ControlledJob, conditionType v1.ControlledJobConditionType,
	jobs []*kbatch.Job, test func(*kbatch.Job) bool,
	reasonWhenTrue, messageWhenTrue,
	reasonWhenFalse, messageWhenFalse string) {
	conditionIsTrue := false
	for _, job := range jobs {
		if test(job) {
			conditionIsTrue = true
			break
		}
	}
	v1.SetConditionBasedOnFlag(controlledJob, conditionType, conditionIsTrue,
		reasonWhenTrue, messageWhenTrue,
		reasonWhenFalse, messageWhenFalse,
	)
}

func isOutOfDate(job *kbatch.Job, state *state) bool {
	actualHash := job.ObjectMeta.Annotations[metadata.TemplateHashAnnotation]
	return actualHash != "" && actualHash != state.DesiredHash
}

func isBetterCandidateJob(job *kbatch.Job, currentCandidate *kbatch.Job, state *state) bool {
	// jobs that are not being deleted are better than ones being deleted
	if metadata.IsJobBeingDeleted(job) != metadata.IsJobBeingDeleted(currentCandidate) {
		return !metadata.IsJobBeingDeleted(job)
	}

	// jobs with up to date specs are better than out of date jobs
	jobHash := job.ObjectMeta.Annotations[metadata.TemplateHashAnnotation]
	currentCandidateHash := currentCandidate.ObjectMeta.Annotations[metadata.TemplateHashAnnotation]
	desiredHash := state.DesiredHash
	if (jobHash == desiredHash) != (currentCandidateHash == desiredHash) {
		return jobHash == desiredHash
	}

	// Otherwise, the one with the greater name lexicographically wins
	return job.Name > currentCandidate.Name
}

func startingDeadlineSecondsExceeded(controlledJob *v1.ControlledJob, scheduledStartTime *schedule.RunPeriodStartTime, now time.Time) bool {
	if controlledJob.Spec.StartingDeadlineSeconds == nil || *controlledJob.Spec.StartingDeadlineSeconds < 1 {
		// No starting deadline set
		return false
	}

	if scheduledStartTime == nil {
		// No scheduled start time
		return false
	}

	startTime := time.Time(*scheduledStartTime)
	deadline := startTime.Add(time.Second * time.Duration(*controlledJob.Spec.StartingDeadlineSeconds))

	// Check if we're after the deadline
	return now.After(deadline)
}
