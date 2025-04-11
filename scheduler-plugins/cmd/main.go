package main

import (
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"github.com/NVIDIA/grove/scheduler-plugins/grovescheduling"
)

func main() {
	// Register custom plugins to the scheduler framework.
	// Later they can consist of scheduler profile(s) and hence
	// used by various kinds of workloads.
	command := app.NewSchedulerCommand(
		app.WithPlugin(grovescheduling.Name, grovescheduling.New),
	)

	code := cli.Run(command)
	os.Exit(code)
}

