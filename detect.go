package pipenv

import (
	"os"

	"github.com/paketo-buildpacks/packit/v2"
)

// BuildPlanMetadata is the buildpack specific data included in build plan
// requirements.
type BuildPlanMetadata struct {
	// VersionSource denotes where dependency version came from (e.g. an
	// environment variable).
	VersionSource string `toml:"version-source"`

	// Version denotes the version of a dependency, if there is one.
	Version string `toml:"version"`

	// Build denotes the dependency is needed at build-time.
	Build bool `toml:"build"`

	// Launch denotes the dependency is needed at runtime.
	Launch bool `toml:"launch"`
}

// Detect will return a packit.DetectFunc that will be invoked during the
// detect phase of the buildpack lifecycle.
//
// This buildpack always passes detection and will contribute a Build Plan that
// provides pipenv.
//
// If a version is provided via the $BP_PIPENV_VERSION environment variable,
// that version of pipenv will be a requirement.
func Detect() packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {

		requirements := []packit.BuildPlanRequirement{
			{
				Name: Pip,
				Metadata: BuildPlanMetadata{
					Build: true,
				},
			},
			{
				Name: CPython,
				Metadata: BuildPlanMetadata{
					Build: true,
				},
			},
		}

		pipEnvVersion, ok := os.LookupEnv("BP_PIPENV_VERSION")
		if ok {
			requirements = append(requirements, packit.BuildPlanRequirement{
				Name: Pipenv,
				Metadata: BuildPlanMetadata{
					Version:       pipEnvVersion,
					VersionSource: "BP_PIPENV_VERSION",
				},
			})
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: Pipenv},
				},
				Requires: requirements,
			},
		}, nil
	}
}
