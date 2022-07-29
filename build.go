package pipenv

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface InstallProcess --output fakes/install_process.go
//go:generate faux --interface SitePackageProcess --output fakes/site_package_process.go
//go:generate faux --interface SBOMGenerator --output fakes/sbom_generator.go

// InstallProcess defines the interface for installing the pipenv dependency into a layer.
type InstallProcess interface {
	Execute(version, destLayerPath string) error
}

// SitePackageProcess defines the interface for looking up site packages within a layer.
type SitePackageProcess interface {
	Execute(targetLayerPath string) (string, error)
}

type SBOMGenerator interface {
	Generate(dir string) (sbom.SBOM, error)
}

// Build will return a packit.BuildFunc that will be invoked during the build
// phase of the buildpack lifecycle.
//
// Build will find the right pipenv dependency to install, install it in a
// layer, and generate Bill-of-Materials. It also makes use of the checksum of
// the dependency to reuse the layer when possible.
func Build(
	installProcess InstallProcess,
	siteProcess SitePackageProcess,
	sbomGenerator SBOMGenerator,
	logger scribe.Emitter,
	clock chronos.Clock,
) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		logger.Process("Resolving Pipenv version")

		config, err := cargo.NewBuildpackParser().Parse(filepath.Join(context.CNBPath, "buildpack.toml"))
		if err != nil {
			return packit.BuildResult{}, err
		}

		entries := context.Plan.Entries
		entries = append(entries, packit.BuildpackPlanEntry{
			Name: Pipenv,
			Metadata: map[string]interface{}{
				"version":        config.Metadata.DefaultVersions[Pipenv],
				"version-source": DefaultVersions,
			},
		})

		planner := draft.NewPlanner()
		entry, sortedEntries := planner.Resolve(Pipenv, entries, Priorities)
		logger.Candidates(sortedEntries)

		version := entry.Metadata["version"].(string)
		source, ok := entry.Metadata["version-source"].(string)
		if !ok {
			source = "<unknown>"
		}
		logger.Subprocess("Selected Pipenv version (using %s): %s", source, version)

		pipenvLayer, err := context.Layers.Get(Pipenv)
		if err != nil {
			return packit.BuildResult{}, err
		}

		launch, build := planner.MergeLayerTypes(Pipenv, context.Plan.Entries)

		cachedPipenvVersion, ok := pipenvLayer.Metadata[PipenvVersion].(string)
		if ok && cachedPipenvVersion == version {
			logger.Process("Reusing cached layer %s", pipenvLayer.Path)
			pipenvLayer.Launch, pipenvLayer.Build, pipenvLayer.Cache = launch, build, build

			return packit.BuildResult{
				Layers: []packit.Layer{pipenvLayer},
			}, nil
		}

		pipenvLayer, err = pipenvLayer.Reset()
		if err != nil {
			return packit.BuildResult{}, err
		}

		pipenvLayer.Launch, pipenvLayer.Build, pipenvLayer.Cache = launch, build, build

		logger.Process("Executing build process")
		logger.Subprocess(fmt.Sprintf("Installing Pipenv %s", version))

		duration, err := clock.Measure(func() error {
			return installProcess.Execute(version, pipenvLayer.Path)
		})

		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		logger.GeneratingSBOM(pipenvLayer.Path)
		var sbomContent sbom.SBOM
		duration, err = clock.Measure(func() error {
			sbomContent, err = sbomGenerator.Generate(pipenvLayer.Path)
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
			PipenvVersion: version,
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
		}, nil
	}
}
