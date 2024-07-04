package test

import (
	"time"

	v1 "github.com/G-Research/controlled-job/api/v1"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	kbatch "k8s.io/api/batch/v1"
)

var _ = Describe("ControlledJob controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ControlledJobName      = "test-controlled-job"
		ControlledJobNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	AfterEach(func() {
		DeleteAllControlledJobs()
	}, 60)

	Context("Basic create/delete", func() {
		It("Should be able to create and delete controlledJobs", func() {
			controlledJob := NewControlledJobInNamepsace(ControlledJobName, ControlledJobNamespace, WithEventInTheFuture(v1.EventTypeStart), WithDefaultJobTemplate())

			Expect(ListOfControlledJobs()).To(BeEmpty())

			By("By creating a new ControlledJob")
			Expect(k8sClient.Create(ctx, controlledJob)).Should(Succeed())

			Expect(ListOfControlledJobs()).To(HaveLen(1))

			By("By deleting the ControlledJob")
			Expect(k8sClient.Delete(ctx, controlledJob)).Should(Succeed())

			Expect(ListOfControlledJobs()).To(BeEmpty())
		})

	})

	Context("Starting jobs", func() {
		var name string
		var controlledJob *v1.ControlledJob

		// Create the controleld job to each test's specification
		JustBeforeEach(func() {
			Expect(k8sClient.Create(ctx, controlledJob)).Should(Succeed())
		})
		BeforeEach(func() {
			name = RandomControlledJobName("test-starting-jobs-")
			controlledJob = NewControlledJobInNamepsace(name, ControlledJobNamespace, WithEventInThePast(v1.EventTypeStart), WithEventInTheFuture(v1.EventTypeStop), WithDefaultJobTemplate())
		})

		It("Should spin up a new job", func() {
			Eventually(func() []kbatch.Job { return ListOfJobsForControlledJob(name) }, timeout, interval).Should(HaveLen(1))
		})
	})

})
