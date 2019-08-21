package main

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/buildpackplan"
	"github.com/cloudfoundry/libcfbuildpack/runner"
	"github.com/cloudfoundry/pipenv-cnb/pipenv"
)

func main() {
	context, err := build.DefaultBuild()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create default build context: %s", err)
		os.Exit(100)
	}

	code, err := runBuild(context, runner.CommandRunner{})
	if err != nil {
		context.Logger.Info(err.Error())
	}

	os.Exit(code)
}

func runBuild(context build.Build, runner runner.Runner) (int, error) {
	context.Logger.FirstLine(context.Logger.PrettyIdentity(context.Buildpack))

	contributor, willContribute, err := pipenv.NewContributor(context, runner)
	if err != nil {
		return context.Failure(102), err
	}

	if willContribute {
		if err := contributor.ContributePipenv(); err != nil {
			return context.Failure(103), err
		}

		if err := contributor.ContributeRequirementsTxt(); err != nil {
			return context.Failure(104), err
		}
	}

	return context.Success(buildpackplan.Plan{})
}
