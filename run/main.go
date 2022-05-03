package main

import (
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/pipenv"
)

type Generator struct{}

func (f Generator) GenerateFromDependency(dependency postal.Dependency, path string) (sbom.SBOM, error) {
	return sbom.GenerateFromDependency(dependency, path)
}

func main() {
	logger := scribe.NewEmitter(os.Stdout).WithLevel(os.Getenv("BP_LOG_LEVEL"))

	packit.Run(
		pipenv.Detect(),
		pipenv.Build(
			postal.NewService(cargo.NewTransport()),
			pipenv.NewPipenvInstallProcess(pexec.NewExecutable("pip")),
			pipenv.NewSiteProcess(pexec.NewExecutable("python")),
			Generator{},
			logger,
			chronos.DefaultClock),
	)
}
