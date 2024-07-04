package util

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	v1 "github.com/G-Research/controlled-job/api/v1"
	jobpkg "github.com/G-Research/controlled-job/pkg/job"
	"github.com/G-Research/controlled-job/pkg/mutators"
	"github.com/urfave/cli/v2"
)

func DoGenerateJob(c *cli.Context) error {
	scheduledAt := c.Timestamp("scheduled-at")
	jobRunId := c.Int("job-run-id")
	manuallyScheduled := c.Bool("manually-scheduled")
	startSuspended := c.Bool("start-suspended")
	remoteWebhookUrl := c.String("job-admission-webhook-url")

	if len(remoteWebhookUrl) > 0 {
		if err := mutators.EnableRemoteMutator(remoteWebhookUrl); err != nil {
			panic(err)
		}
	}

	stdin, err := io.ReadAll(os.Stdin)

	if err != nil {
		panic(err)
	}

	controlledJob := &v1.ControlledJob{}
	err = json.Unmarshal(stdin, controlledJob)

	if err != nil {
		panic(err)
	}

	job, err := jobpkg.BuildForControlledJob(c.Context, controlledJob, *scheduledAt, jobRunId, manuallyScheduled, startSuspended)
	if err != nil {
		panic(err)
	}

	jobJson, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(jobJson))

	return nil
}
