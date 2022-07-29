package main

import (
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/pipenv"
)

type SBOMGenerator struct{}

func (s SBOMGenerator) Generate(path string) (sbom.SBOM, error) {
	return sbom.Generate(path)
}

func main() {
	logger := scribe.NewEmitter(os.Stdout).WithLevel(os.Getenv("BP_LOG_LEVEL"))

	packit.Run(
		pipenv.Detect(),
		pipenv.Build(
			pipenv.NewPipenvInstallProcess(pexec.NewExecutable("pip")),
			pipenv.NewSiteProcess(pexec.NewExecutable("python")),
			SBOMGenerator{},
			logger,
			chronos.DefaultClock),
	)
}
