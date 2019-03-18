package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/detect"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/pipenv-cnb/pipenv"
)

func main() {
	context, err := detect.DefaultDetect()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create default detect context: %s", err)
		os.Exit(100)
	}

	if err := context.BuildPlan.Init(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to initialize Build Plan: %s\n", err)
		os.Exit(101)
	}

	code, err := runDetect(context)
	if err != nil {
		context.Logger.Info(err.Error())
	}

	os.Exit(code)
}

func runDetect(context detect.Detect) (int, error) {
	exists, err := helper.FileExists(filepath.Join(context.Application.Root, "Pipfile"))
	if err != nil {
		return detect.FailStatusCode, err
	} else if !exists {
		context.Logger.Info("no Pipfile found")
		return detect.FailStatusCode, nil
	}

	if exists, err := helper.FileExists(filepath.Join(context.Application.Root, "requirements.txt")); err != nil {
		return detect.FailStatusCode, err
	} else if exists {
		context.Logger.Error("found Pipfile + requirements.txt")
		return detect.FailStatusCode, nil
	}

	pipfileLockExists, err := helper.FileExists(filepath.Join(context.Application.Root, "Pipfile.lock"))
	if err != nil {
		return detect.FailStatusCode, errors.Wrap(err, "error checking for pipfile.lock")
	}

	pythonVersion := context.BuildPlan[pipenv.PythonLayer].Version
	if pipfileLockExists {
		pipfileVersion, err := pipenv.GetPythonVersionFromPipfileLock(filepath.Join(context.Application.Root, "Pipfile.lock"))
		if err != nil {
			return detect.FailStatusCode, errors.Wrap(err, "error reading python version from pipfile.lock")
		}

		if pythonVersion == "" && pipfileVersion != "" {
			pythonVersion = pipfileVersion
		} else if pythonVersion != "" && pipfileVersion != "" && pythonVersion != pipfileVersion {
			context.Logger.Info("There is a mismatch of your python version between either your buildpack.yml and Pipfile.lock")
		}
	}

	return context.Pass(buildplan.BuildPlan{
		pipenv.PythonLayer: buildplan.Dependency{
			Version:  pythonVersion,
			Metadata: buildplan.Metadata{"build": true, "launch": true},
		},
		pipenv.Layer: buildplan.Dependency{
			Metadata: buildplan.Metadata{"build": true},
		},
		pipenv.PythonPackagesLayer: buildplan.Dependency{
			Metadata: buildplan.Metadata{"build": true, "launch": true},
		},
	})
}
