package test

import (
	batch "github.com/G-Research/controlled-job/api/v1"
	kbatch "k8s.io/api/batch/v1"
)

/*
These funtions are expected to be used in
  Expect(...)
or
  Eventually(...)
etc. calls.

Be careful in Eventually/Consistently calls:

// THIS IS WRONG
Eventually(ListOfControlledJobs()).Should(...)

will not do what you expect. It will call ListOfControlledJobs() once, and then expect that return value
to eventually (magically) change to some target value. Instead do

Eventually(func() []batch.ControlledJob { return ListOfControlledJobs() })


They return functions (which must then be invoked) to avoid
accidentally freezing the return value on first call.

e.g. in the following code:

func BrokenListAllControlledJobs() []batch.ControlledJob {
	return client.ListControlledJobs()
}
Eventually(BrokenListAllControlledJobs(), time.Second * 60, time.Second)

client.ListControlledJobs() ends up getting called just once, when Eventually() is called, not every second for a minute as intended
*/

func ListOfControlledJobs() []batch.ControlledJob {
	var allControlledJobs batch.ControlledJobList
	err := k8sClient.List(ctx, &allControlledJobs)
	if err != nil {
		panic(err)
	}
	return allControlledJobs.Items
}

func ListOfJobsForControlledJob(controlledJobName string) []kbatch.Job {
	var allJobs kbatch.JobList
	err := k8sClient.List(ctx, &allJobs)
	if err != nil {
		panic(err)
	}
	filteredJobs := make([]kbatch.Job, 0)
	for _, job := range allJobs.Items {
		for _, ownerRef := range job.OwnerReferences {
			// 	{
			// 		APIVersion: "batch.gresearch.co.uk/v1",
			// 		Kind: "ControlledJob",
			// 		Name: "test-controlled-job",
			// 		UID: "f959343b-5d4e-4c1b-90e9-79a4b4e969d8",
			// 		Controller: true,
			// 		BlockOwnerDeletion: true,
			// },
			if ownerRef.Kind == "ControlledJob" && ownerRef.Name == controlledJobName {
				filteredJobs = append(filteredJobs, job)
			}
		}
	}
	return filteredJobs

}

func DeleteAllControlledJobs() {
	for _, item := range ListOfControlledJobs() {
		err := k8sClient.Delete(ctx, &item)
		if err != nil {
			panic(err)
		}
	}
}
