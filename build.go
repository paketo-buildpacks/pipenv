package pipenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface DependencyManager --output fakes/dependency_manager.go
//go:generate faux --interface InstallProcess --output fakes/install_process.go
//go:generate faux --interface SitePackageProcess --output fakes/site_package_process.go
//go:generate faux --interface SBOMGenerator --output fakes/sbom_generator.go

// DependencyManager defines the interface for picking the best matching
// dependency and installing it.
type DependencyManager interface {
	Resolve(path, id, version, stack string) (postal.Dependency, error)
	Deliver(dependency postal.Dependency, cnbPath, destPath, platformPath string) error
	GenerateBillOfMaterials(dependencies ...postal.Dependency) []packit.BOMEntry
}

// InstallProcess defines the interface for installing the pipenv dependency into a layer.
type InstallProcess interface {
	Execute(srcPath, destLayerPath string) error
}

// SitePackageProcess defines the interface for looking up site packages within a layer.
type SitePackageProcess interface {
	Execute(targetLayerPath string) (string, error)
}

type SBOMGenerator interface {
	GenerateFromDependency(dependency postal.Dependency, dir string) (sbom.SBOM, error)
}

// Build will return a packit.BuildFunc that will be invoked during the build
// phase of the buildpack lifecycle.
//
// Build will find the right pipenv dependency to install, install it in a
// layer, and generate Bill-of-Materials. It also makes use of the checksum of
// the dependency to reuse the layer when possible.
func Build(
	dependencyManager DependencyManager,
	installProcess InstallProcess,
	siteProcess SitePackageProcess,
	sbomGenerator SBOMGenerator,
	logger scribe.Emitter,
	clock chronos.Clock,
) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		planner := draft.NewPlanner()

		logger.Process("Resolving Pipenv version")
		entry, sortedEntries := planner.Resolve(Pipenv, context.Plan.Entries, Priorities)

		logger.Candidates(sortedEntries)

		version, _ := entry.Metadata["version"].(string)

		dependency, err := dependencyManager.Resolve(filepath.Join(context.CNBPath, "buildpack.toml"), entry.Name, version, context.Stack)
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.SelectedDependency(entry, dependency, clock.Now())

		legacySBOM := dependencyManager.GenerateBillOfMaterials(dependency)
		launch, build := planner.MergeLayerTypes(Pipenv, context.Plan.Entries)

		var launchMetadata packit.LaunchMetadata
		if launch {
			launchMetadata.BOM = legacySBOM
		}

		var buildMetadata packit.BuildMetadata
		if build {
			buildMetadata.BOM = legacySBOM
		}

		pipenvLayer, err := context.Layers.Get(Pipenv)
		if err != nil {
			return packit.BuildResult{}, err
		}

		cachedSHA, ok := pipenvLayer.Metadata[DependencySHAKey].(string)
		if ok && cachedSHA == dependency.SHA256 {
			logger.Process("Reusing cached layer %s", pipenvLayer.Path)
			pipenvLayer.Launch, pipenvLayer.Build, pipenvLayer.Cache = launch, build, build

			return packit.BuildResult{
				Layers: []packit.Layer{pipenvLayer},
				Build:  buildMetadata,
				Launch: launchMetadata,
			}, nil
		}

		pipenvLayer, err = pipenvLayer.Reset()
		if err != nil {
			return packit.BuildResult{}, err
		}

		pipenvLayer.Launch, pipenvLayer.Build, pipenvLayer.Cache = launch, build, build

		// Install the pipenv source to a temporary dir, since we only need access to
		// it as an intermediate step when installing pipenv.
		// It doesn't need to go into a layer, since we won't need it in future builds.
		pipEnvReleaseDir, err := os.MkdirTemp("", "pipenv-release")
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Process("Executing build process")
		logger.Subprocess(fmt.Sprintf("Installing Pipenv %s", dependency.Version))

		duration, err := clock.Measure(func() error {
			err = dependencyManager.Deliver(dependency, context.CNBPath, pipEnvReleaseDir, context.Platform.Path)
			if err != nil {
				return err
			}

			return installProcess.Execute(pipEnvReleaseDir, pipenvLayer.Path)
		})

		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		logger.GeneratingSBOM(pipenvLayer.Path)
		var sbomContent sbom.SBOM
		duration, err = clock.Measure(func() error {
			sbomContent, err = sbomGenerator.GenerateFromDependency(dependency, pipenvLayer.Path)
			return err
		})
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		logger.FormattingSBOM(context.BuildpackInfo.SBOMFormats...)
		pipenvLayer.SBOM, err = sbomContent.InFormats(context.BuildpackInfo.SBOMFormats...)
		if err != nil {
			return packit.BuildResult{}, err
		}

		pipenvLayer.Metadata = map[string]interface{}{
			DependencySHAKey: dependency.SHA256,
		}

		// Look up the site packages path and prepend it onto $PYTHONPATH
		sitePackagesPath, err := siteProcess.Execute(pipenvLayer.Path)
		if err != nil {
			return packit.BuildResult{}, err
		}

		if sitePackagesPath == "" {
			return packit.BuildResult{}, fmt.Errorf("pipenv installation failed: site packages are missing from the pipenv layer")
		}

		pipenvLayer.SharedEnv.Prepend("PYTHONPATH", strings.TrimRight(sitePackagesPath, "\n"), ":")

		logger.EnvironmentVariables(pipenvLayer)

		return packit.BuildResult{
			Layers: []packit.Layer{pipenvLayer},
			Build:  buildMetadata,
			Launch: launchMetadata,
		}, nil
	}
}
