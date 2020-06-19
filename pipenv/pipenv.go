package pipenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cloudfoundry/libcfbuildpack/build"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/layers"
	"github.com/cloudfoundry/libcfbuildpack/logger"
	"github.com/cloudfoundry/libcfbuildpack/runner"
	"github.com/pkg/errors"
)

const (
	Dependency        = "pipenv"
	PythonLayer       = "python"
	RequirementsLayer = "requirements"
	Pipfile           = "Pipfile"
	LockFile          = "Pipfile.lock"
	RequirementsFile  = "requirements.txt"
)

type PipfileLock struct {
	Meta struct {
		Requires struct {
			Version string `json:"python_version"`
		} `json:"requires"`
		Sources []struct {
			URL string
		}
	} `json:"_meta"`
	Default map[string]struct {
		Version string
	}
}

type Contributor struct {
	requirementsMetadata logger.Identifiable
	context              build.Build
	runner               runner.Runner
	requirementsLayer    layers.Layer
	buildContribution    bool
	launchContribution   bool
}

type Metadata struct {
	Name string
	Hash string
}

func (m Metadata) Identity() (name string, version string) {
	return m.Name, m.Hash
}

func NewContributor(context build.Build, runner runner.Runner) (Contributor, bool, error) {
	plan, willContribute, err := context.Plans.GetShallowMerged(Dependency)
	if err != nil || !willContribute {
		return Contributor{}, false, err
	}

	contributor := Contributor{
		context:              context,
		runner:               runner,
		requirementsLayer:    context.Layers.Layer(RequirementsLayer),
		requirementsMetadata: Metadata{RequirementsLayer, strconv.FormatInt(time.Now().UnixNano(), 16)},
	}

	contributor.buildContribution, _ = plan.Metadata["build"].(bool)
	contributor.launchContribution, _ = plan.Metadata["launch"].(bool)

	return contributor, true, nil
}

func GetPythonVersionFromPipfileLock(fullPath string) (string, error) {
	file, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	pipfileLock := PipfileLock{}
	err = json.Unmarshal(file, &pipfileLock)
	if err != nil {
		return "", err
	}

	return pipfileLock.Meta.Requires.Version, nil

}

func (c Contributor) ContributePipenv() error {
	deps, err := c.context.Buildpack.Dependencies()
	if err != nil {
		return err
	}

	dep, err := deps.Best(Dependency, "*", c.context.Stack)
	if err != nil {
		return err
	}

	layer := c.context.Layers.DependencyLayer(dep)

	return layer.Contribute(func(artifact string, layer layers.DependencyLayer) error {
		layer.Logger.Body("Expanding to %s", layer.Root)
		if err := helper.ExtractTarGz(artifact, layer.Root, 0); err != nil {
			return errors.Wrap(err, "problem extracting")
		}

		if err := c.runner.Run("python", layer.Root, "-m", "pip", "install", "pipenv", "--find-links="+layer.Root); err != nil {
			return errors.Wrap(err, "problem installing pipenv")
		}

		return nil
	}, c.flags()...)
}

func (c Contributor) ContributeRequirementsTxt() error {
	c.context.Logger.Info("Generating requirements.txt")

	lockPath := filepath.Join(c.context.Application.Root, LockFile)
	hasLockFile, err := helper.FileExists(lockPath)
	if err != nil {
		return err
	}

	var requirementsContent []byte
	if hasLockFile {
		c.context.Logger.Info(fmt.Sprintf("Generating %s from %s", RequirementsFile, LockFile))
		requirementsContent, err = pipfileLockToRequirementsTxt(lockPath)
		if err != nil {
			return errors.Wrapf(err, "problem generating %s from %s", RequirementsFile, LockFile)
		}
	} else {
		if err := c.runner.Run("pipenv", c.context.Application.Root, "lock", "--requirements"); err != nil {
			return errors.Wrap(err, "problem generating initial Pipfile.lock")
		}

		// When we run this a second time, we get the output we care about without extraneous logging
		requirementsContent, err = c.runner.RunWithOutput("pipenv", c.context.Application.Root, "lock", "--requirements")
		if err != nil {
			return errors.Wrapf(err, "problem with reading requirements from %s", LockFile)
		}
	}

	return c.requirementsLayer.Contribute(c.requirementsMetadata, func(layer layers.Layer) error {
		layer.Touch()
		layer.Logger.Body("Writing %s to %s", RequirementsFile, layer.Root)
		requirementsPath := filepath.Join(layer.Root, RequirementsFile)

		if err = helper.WriteFile(requirementsPath, 0644, "%s", requirementsContent); err != nil {
			return errors.Wrap(err, "problem writing requirements")
		}

		return helper.CopyFile(requirementsPath, filepath.Join(c.context.Application.Root, RequirementsFile))
	})
}

func pipfileLockToRequirementsTxt(pipfileLockPath string) ([]byte, error) {
	lockContents, err := ioutil.ReadFile(pipfileLockPath)
	if err != nil {
		return []byte{}, err
	}

	lockFile := PipfileLock{}
	err = json.Unmarshal(lockContents, &lockFile)
	if err != nil {
		return []byte{}, err
	}

	buf := &bytes.Buffer{}

	for _, source := range lockFile.Meta.Sources {
		fmt.Fprintf(buf, "-i %s\n", source.URL)
	}

	for pkg, obj := range lockFile.Default {
		fmt.Fprintf(buf, "%s%s\n", pkg, obj.Version)
	}

	return buf.Bytes(), nil
}

func (c Contributor) flags() []layers.Flag {
	flags := []layers.Flag{layers.Cache}

	if c.buildContribution {
		flags = append(flags, layers.Build)
	}

	if c.launchContribution {
		flags = append(flags, layers.Launch)
	}
	return flags
}
