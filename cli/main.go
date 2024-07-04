package main

import (
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/G-Research/controlled-job/cli/util"
)

var app = &cli.App{
	Name:  "ControlledJob CLI",
	Usage: "CLI for the ControlledJob custom resource type in Kubernetes",
	Commands: []*cli.Command{
		utilCommand,
	},
}

var utilCommand = &cli.Command{
	Name:        "util",
	Usage:       "various helpers related to ControlledJobs",
	Description: "A collection of utility helpers",
	Subcommands: []*cli.Command{
		{
			Name:        "generate-job",
			Usage:       "generate the manifest for a job for a ControlledJob",
			Description: "The canonical way to generate the Job manifest for a ControlledJob. Ensures consistency between various tooling. Supply a value for all of the flags, and write a complete ControlledJob manifest in json to stdin",
			Flags: []cli.Flag{
				&cli.TimestampFlag{
					Name:     "scheduled-at",
					Usage:    "Timestamp that the Job is scheduled at, in RFC3339 format, e.g. 2022-11-03T11:01:01Z",
					Layout:   time.RFC3339,
					Required: true,
				},
				&cli.IntFlag{
					Name:     "job-run-id",
					Usage:    "The Job Run Id is an incrementing index of jobs for a single period (e.g. a single day) of a ControlledJob",
					Required: true,
				},
				&cli.BoolFlag{
					Name:  "manually-scheduled",
					Usage: "Is this job manually scheduled (i.e. started by the user via the API) or automatically by the operator?",
				},
				&cli.BoolFlag{
					Name:  "start-suspended",
					Usage: "Should the job be started in a suspended state?",
				},
				&cli.StringFlag{
					Name:  "job-admission-webhook-url",
					Usage: "If set, new jobs will be sent to this URL prior to creation. The remote service is expected to behave like a K8s MutatingAdmissionWebhook and return a patch to be applied",
				},
			},
			Action: util.DoGenerateJob,
		},
	},
}

var (
	Version     = "dev"
	GitRevision = "local"
)

func main() {
	app.Version = fmt.Sprintf("%s-%s", Version, GitRevision)
	if err := app.Run(os.Args); err != nil {
		panic(err)
	}
}
