package reconciletests

// This is our main integration  package - given a ControlledJob
// in a particular state and a set of existing jobs in the cluster, what should the reconciler do?
// - What jobs should we create/delete?
// - How should the ControlledJob status be updated?
// - When should we ask the controlled runtime to next reschedule that ControlledJob?
// etc.
// Add as many test cases as you need here. Add a test case when there's a bug we're fixing
// Add a test case for weird edge cases where we've had to make a call on what should happen

// Each test should create an instance of testContext via newTest, which keeps track of
// the cluster state, and observed actions. This allows tests to be written using the Given/When/Then
// helpers defined below in this file. e.g.
// tc := newTest(t)
//
// tc.GivenAControlledJob(WithScheduledEvent(v1.EventTypeStop, "Mon-Fri", "17:00"))
// tc.GivenAnExistingJob(
//   WithJobName("foo"),
//   WithActiveCount(1))
// tc.WhenReconcileIsRunAt(now)
//
// // Should not have deleted the old job or create a new one
// tc.ShouldNotHaveDeletedAJob()
// tc.ShouldNotHaveCreatedAJob()

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/G-Research/controlled-job/pkg/mutators"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/clientadapter"
	"github.com/G-Research/controlled-job/pkg/events"
	"github.com/G-Research/controlled-job/pkg/metadata"
	"github.com/G-Research/controlled-job/pkg/reconciliation"
	"github.com/G-Research/controlled-job/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	fakeNow = time.Now()
)

func init() {
	events.NowFunc = func() *time.Time { return &fakeNow }
}

type testContext struct {
	*testing.T
	// What's the current state of the cluster?
	controlledJob *batch.ControlledJob
	existingJobs  []kbatch.Job

	// Mocks of the K8s interaction
	client        *clientadapter.ControlledJobClientMock
	eventsHandler events.Handler

	// Records of the result of each reconcile run
	currentReconcileRun *reconcileRun
	reconcileRuns       []*reconcileRun

	// Mock mutators
	mutator *testMutator
}

type reconcileRun struct {
	now             time.Time
	result          controllerruntime.Result
	error           error
	jobsCreated     []*kbatch.Job
	jobsSuspended   []*kbatch.Job
	jobsUnsuspended []*kbatch.Job
	jobsDeleted     []struct {
		job         *kbatch.Job
		propagation metav1.DeletionPropagation
	}
	events []recordedEvent
	status batch.ControlledJobStatus
}

type recordedEvent struct {
	EventType string
	Reason    string
	Message   string
}

type testMutator struct {
	image  string
	err    error
	called bool
}

func (t *testMutator) Name() string {
	return "testMutator"
}

func (t *testMutator) Apply(ctx context.Context, job *kbatch.Job) error {
	t.called = true
	for i := range job.Spec.Template.Spec.Containers {
		job.Spec.Template.Spec.Containers[i].Image = t.image
	}
	return t.err
}

func Run(t *testing.T, name string, test func(tc *testContext)) {
	t.Run(name, func(t *testing.T) {
		tc := newTest(t)
		test(tc)
	})
}

func (tc *testContext) Run(name string, test func(tc *testContext)) {
	Run(tc.T, name, test)
}

func newTest(t *testing.T) *testContext {
	client := &clientadapter.ControlledJobClientMock{}
	recorder := &events.EventRecorderMock{}
	eventsHandler := events.NewHandler(recorder)

	tc := &testContext{
		T:                   t,
		controlledJob:       testhelpers.NewControlledJob("my-controlled-job"),
		existingJobs:        []kbatch.Job{},
		client:              client,
		currentReconcileRun: nil,
		reconcileRuns:       []*reconcileRun{},
		eventsHandler:       eventsHandler,
	}

	client.GetControlledJobFunc = func(ctx context.Context, namespacedName types.NamespacedName) (*batch.ControlledJob, bool, error) {
		if namespacedName.Name == tc.controlledJob.Name && namespacedName.Namespace == tc.controlledJob.Namespace {
			return tc.controlledJob, true, nil
		}
		// Not found
		return nil, false, nil
	}
	client.ListJobsForControlledJobFunc = func(ctx context.Context, namespacedName types.NamespacedName) (kbatch.JobList, error) {
		if namespacedName.Name == tc.controlledJob.Name && namespacedName.Namespace == tc.controlledJob.Namespace {
			return kbatch.JobList{Items: tc.existingJobs}, nil
		}
		return kbatch.JobList{}, nil
	}
	client.CreateJobFunc = func(ctx context.Context, job *kbatch.Job) error {
		tc.currentReconcileRun.jobsCreated = append(tc.currentReconcileRun.jobsCreated, job)
		return nil
	}
	client.SuspendJobFunc = func(ctx context.Context, job *kbatch.Job) error {
		t := true
		job.Spec.Suspend = &t
		tc.currentReconcileRun.jobsSuspended = append(tc.currentReconcileRun.jobsSuspended, job)
		return nil
	}
	client.UnsuspendJobFunc = func(ctx context.Context, job *kbatch.Job) error {
		f := false
		job.Spec.Suspend = &f
		tc.currentReconcileRun.jobsUnsuspended = append(tc.currentReconcileRun.jobsUnsuspended, job)
		return nil
	}
	client.DeleteJobFunc = func(ctx context.Context, job *kbatch.Job, propagation metav1.DeletionPropagation) error {
		tc.currentReconcileRun.jobsDeleted = append(tc.currentReconcileRun.jobsDeleted, struct {
			job         *kbatch.Job
			propagation metav1.DeletionPropagation
		}{job: job, propagation: propagation})
		return nil
	}
	client.UpdateStatusFunc = func(ctx context.Context, controlledJob *batch.ControlledJob) error {
		tc.currentReconcileRun.status = *controlledJob.Status.DeepCopy()
		return nil
	}

	recorder.EventFunc = func(object runtime.Object, eventtype, reason, message string) {
		tc.currentReconcileRun.events = append(tc.currentReconcileRun.events, recordedEvent{
			EventType: eventtype,
			Reason:    reason,
			Message:   message,
		})
	}

	return tc
}

func (tc *testContext) GivenAControlledJob(opts ...testhelpers.ControlledJobOption) {
	for _, opt := range opts {
		opt(tc.controlledJob)
	}
}

func (tc *testContext) GivenExistingJobs(jobs ...*kbatch.Job) {
	for _, job := range jobs {
		tc.existingJobs = append(tc.existingJobs, *job)
	}
}

func (tc *testContext) GivenAnExistingJob(opts ...testhelpers.JobOption) {
	newJob := testhelpers.NewJob("my-controlled-job", opts...)
	tc.existingJobs = append(tc.existingJobs, *newJob)
}

// WhenReconcileIsRunAt initiates a run of the reconcile logic at the given time. It will record the result, any error, and
// any jobs that were created, updated or deleted
func (tc *testContext) WhenReconcileIsRunAt(now time.Time) *reconcileRun {
	// Initialize a new instance of the reconcileRun to record e.g. jobs that got created in this run
	tc.currentReconcileRun = &reconcileRun{
		now: now,
	}
	tc.reconcileRuns = append(tc.reconcileRuns, tc.currentReconcileRun)

	namespacedName := types.NamespacedName{
		Namespace: tc.controlledJob.Namespace,
		Name:      tc.controlledJob.Name}

	logBuffer := bytes.Buffer{}

	ctx := context.Background()
	logger := zap.New(zap.WriteTo(&logBuffer))
	log.SetLogger(logger)

	actual := reconciliation.Reconcile(ctx, namespacedName, now, tc.client, tc.eventsHandler)
	actualResult, actualErr := actual.AsControllerResultAndError()
	tc.currentReconcileRun.result = actualResult
	tc.currentReconcileRun.error = actualErr
	tc.Log(logBuffer.String())

	tc.Log("error", actual.Error)

	return tc.currentReconcileRun
}

func (tc *testContext) WithTestMutator(image string, err error) *testMutator {
	tc.mutator = &testMutator{
		image: image,
		err:   err,
	}
	if err := mutators.Register(tc.mutator); err != nil {
		tc.Fatal(err)
	}
	tc.Cleanup(func() {
		if err := mutators.Unregister(tc.mutator); err != nil {
			tc.Error(err)
		}
	})
	return tc.mutator
}

func (tc *testContext) ShouldHaveBeenRequeuedAt(expectedRequeueAt time.Time) {
	assert.Equal(tc, timeBetweenNowAndThen(tc.currentReconcileRun.now, expectedRequeueAt), tc.currentReconcileRun.result.RequeueAfter, "requeue after")
}

type JobExpectation func(t assert.TestingT, job kbatch.Job)

func (tc *testContext) ShouldHaveCreatedAJob(expectations ...JobExpectation) {
	assert.Equal(tc, 1, len(tc.currentReconcileRun.jobsCreated), "should have created exactly one job")
	if len(tc.currentReconcileRun.jobsCreated) != 1 {
		return
	}
	job := *tc.currentReconcileRun.jobsCreated[0]
	for _, test := range expectations {
		test(tc, job)
	}
}
func (tc *testContext) ShouldHaveSuspendedAJob(expectations ...JobExpectation) {
	assert.Equal(tc, 1, len(tc.currentReconcileRun.jobsSuspended), "should have suspended exactly one job")
	if len(tc.currentReconcileRun.jobsSuspended) != 1 {
		return
	}
	job := *tc.currentReconcileRun.jobsSuspended[0]
	for _, test := range expectations {
		test(tc, job)
	}
}
func (tc *testContext) ShouldHaveUnsuspendedAJob(expectations ...JobExpectation) {
	assert.Equal(tc, 1, len(tc.currentReconcileRun.jobsUnsuspended), "should have suspended exactly one job")
	if len(tc.currentReconcileRun.jobsUnsuspended) != 1 {
		return
	}
	job := *tc.currentReconcileRun.jobsUnsuspended[0]
	for _, test := range expectations {
		test(tc, job)
	}
}
func (tc *testContext) ShouldHaveDeletedAJob(expectations ...JobExpectation) {
	assert.Equal(tc, 1, len(tc.currentReconcileRun.jobsDeleted), "should have deleted exactly one job")
	if len(tc.currentReconcileRun.jobsDeleted) != 1 {
		return
	}
	job := *tc.currentReconcileRun.jobsDeleted[0].job
	for _, test := range expectations {
		test(tc, job)
	}
}

func (tc *testContext) ShouldHaveCalledMutator() {
	assert.True(tc, tc.mutator.called, "mutator not called")
}

func (tc *testContext) ShouldHaveDeletedAJobMatching(expectations ...JobExpectation) {
	assert.True(tc, tc.AJobThatMatchesWasDeleted(expectations...), "expected a matching job to have been deleted")
}

func (tc *testContext) ShouldNotHaveDeletedAJobMatching(expectations ...JobExpectation) {
	assert.False(tc, tc.AJobThatMatchesWasDeleted(expectations...), "expected a matching job not to have been deleted")
}

func (tc *testContext) AJobThatMatchesWasDeleted(expectations ...JobExpectation) bool {
	for _, job := range tc.currentReconcileRun.jobsDeleted {
		dummy := &DummyTestingT{}
		for _, test := range expectations {
			test(dummy, *job.job)
		}
		if len(dummy.errors) == 0 {
			// We found a matching job
			return true
		}
	}
	return false
}

func WithExpectedJobName(expectedName string) JobExpectation {
	return func(t assert.TestingT, job kbatch.Job) {
		assert.Equal(t, expectedName, job.Name, "name should match")
	}
}

func WithExpectedScheduledTime(expectedScheduledTime time.Time) JobExpectation {
	expectedAnnotationValue := expectedScheduledTime.Format(time.RFC3339)
	return func(t assert.TestingT, job kbatch.Job) {
		annotationValue := job.Annotations[metadata.ScheduledTimeAnnotation]
		assert.Equal(t, expectedAnnotationValue, annotationValue, "scheduled time annotation should match")
	}
}

func WithExpectedJobIndex(expectedIndex int) JobExpectation {
	return func(t assert.TestingT, job kbatch.Job) {
		annotationValue := job.Annotations[metadata.JobRunIdAnnotation]
		assert.Equal(t, fmt.Sprintf("%d", expectedIndex), annotationValue, "job run id annotation should match")
	}
}

func WithExpectedControlledJobOwner(expectedControlledJobName string) JobExpectation {
	return func(t assert.TestingT, job kbatch.Job) {
		assert.Equal(t, 1, len(job.OwnerReferences), "should have a single owner reference")
		if len(job.OwnerReferences) == 1 {
			assert.Equal(t, "batch.gresearch.co.uk/v1", job.OwnerReferences[0].APIVersion)
			assert.Equal(t, "ControlledJob", job.OwnerReferences[0].Kind)
			assert.Equal(t, expectedControlledJobName, job.OwnerReferences[0].Name)
			assert.True(t, *job.OwnerReferences[0].Controller)
			assert.True(t, *job.OwnerReferences[0].BlockOwnerDeletion)
		}
	}
}

func WithExpectedImage(expected string) JobExpectation {
	return func(t assert.TestingT, job kbatch.Job) {
		for _, c := range job.Spec.Template.Spec.Containers {
			assert.Equal(t, c.Image, expected)
		}
	}
}

func ThatShouldBeSuspended() JobExpectation {
	t := true
	return WithExpectedSuspendedFlag(&t)
}
func ThatShouldBeUnsuspended() JobExpectation {
	f := false
	return WithExpectedSuspendedFlag(&f)
}

func WithExpectedSuspendedFlag(expectedFlag *bool) JobExpectation {
	return func(t assert.TestingT, job kbatch.Job) {
		if expectedFlag == nil {
			assert.Nil(t, job.Spec.Suspend, "suspend flag on job should be nil")
		} else {
			assert.NotNil(t, job.Spec.Suspend, "suspend flag on job should not be nil")
		}
		if job.Spec.Suspend != nil && expectedFlag != nil {
			assert.Equal(t, *expectedFlag, *job.Spec.Suspend)
		}
	}
}

func WithJobSpecMatching(expectedJobSpec v1beta1.JobTemplateSpec) JobExpectation {
	return func(t assert.TestingT, theJob kbatch.Job) {

		// The suspend flag gets compared elsewhere
		theJob.Spec.Suspend = nil
		expectedJobSpec.Spec.Suspend = nil

		testhelpers.AssertDeepEqualJson(t, expectedJobSpec.Spec, theJob.Spec, "job spec should match")
	}
}

func WithImageMutation(template v1beta1.JobTemplateSpec, image string) v1beta1.JobTemplateSpec {
	for i := range template.Spec.Template.Spec.Containers {
		template.Spec.Template.Spec.Containers[i].Image = image
	}
	return template
}

func (tc *testContext) ShouldNotHaveCreatedAJob() {
	assert.Empty(tc, tc.currentReconcileRun.jobsCreated, "should not have created a job")
}
func (tc *testContext) ShouldNotHaveDeletedAJob() {
	assert.Empty(tc, tc.currentReconcileRun.jobsDeleted, "should not have deleted a job")
}
func (tc *testContext) ShouldNotHaveSuspendedAJob() {
	assert.Empty(tc, tc.currentReconcileRun.jobsSuspended, "should not have suspended a job")
}
func (tc *testContext) ShouldNotHaveUnsuspendedAJob() {
	assert.Empty(tc, tc.currentReconcileRun.jobsUnsuspended, "should not have unsuspended a job")
}

func (tc *testContext) ShouldHaveCondition(conditionType batch.ControlledJobConditionType, expectedStatus metav1.ConditionStatus) {
	condition := batch.FindCondition(tc.currentReconcileRun.status, conditionType)
	if condition == nil {
		assert.Fail(tc, "should have a condition of type %s but doesn't", conditionType)
	} else {
		assert.Equal(tc, condition.Status, expectedStatus, "condition %s should have status %s, but is %s", conditionType, expectedStatus, condition.Status)
	}
}

func timeBetweenNowAndThen(now, requeue time.Time) time.Duration {
	return requeue.Sub(now)
}

// DummyTestingT is a bit of a hack. So we can iterate over a list of jobs and see which one (if any)
// matches a whole set of assertions we implement the assert.TestingT interface by just recording
// any errors it records
type DummyTestingT struct {
	errors []struct {
		format string
		args   []interface{}
	}
}

func (d *DummyTestingT) Errorf(format string, args ...interface{}) {
	d.errors = append(d.errors, struct {
		format string
		args   []interface{}
	}{format: format, args: args})
}
