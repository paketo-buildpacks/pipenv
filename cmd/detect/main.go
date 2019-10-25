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

	code, err := runDetect(context)
	if err != nil {
		context.Logger.Info(err.Error())
	}

	os.Exit(code)
}

func runDetect(context detect.Detect) (int, error) {
	exists, err := helper.FileExists(filepath.Join(context.Application.Root, pipenv.Pipfile))
	if err != nil {
		return detect.FailStatusCode, err
	} else if !exists {
		context.Logger.Info(fmt.Sprintf("no %s found", pipenv.Pipfile))
		return detect.FailStatusCode, nil
	}

	if exists, err := helper.FileExists(filepath.Join(context.Application.Root, pipenv.RequirementsFile)); err != nil {
		return detect.FailStatusCode, err
	} else if exists {
		context.Logger.Error(fmt.Sprintf("found %s + %s", pipenv.Pipfile, pipenv.RequirementsFile))
		return detect.FailStatusCode, fmt.Errorf("found %s + %s", pipenv.Pipfile, pipenv.RequirementsFile)
	}

	pipfileLockPath := filepath.Join(context.Application.Root, pipenv.LockFile)
	pipfileLockExists, err := helper.FileExists(pipfileLockPath)
	if err != nil {
		return detect.FailStatusCode, errors.Wrap(err, "error checking for "+pipenv.LockFile)
	}

	var (
		pipfileVersion string
		versionSource  string
	)

	if pipfileLockExists {
		pipfileVersion, err = pipenv.GetPythonVersionFromPipfileLock(pipfileLockPath)
		if err != nil {
			return detect.FailStatusCode, errors.Wrapf(err, "error reading python version from %s", pipenv.LockFile)
		}
		versionSource = pipenv.LockFile
	}

	return context.Pass(buildplan.Plan{
		Provides: []buildplan.Provided{
			{
				Name: pipenv.Dependency,
			},
			{
				Name: pipenv.RequirementsLayer,
			},
		},
		Requires: []buildplan.Required{
			{
				Name:     pipenv.PythonLayer,
				Version:  pipfileVersion,
				Metadata: buildplan.Metadata{"build": true, "version-source": versionSource},
			},
			{
				Name:     pipenv.Dependency,
				Metadata: buildplan.Metadata{"build": true},
			},
		},
	})
}
