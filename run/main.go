package main

import (
	"os"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/pipenv"
)

func main() {
	entryResolver := draft.NewPlanner()
	dependencyManager := postal.NewService(cargo.NewTransport())
	installProcess := pipenv.NewPipenvInstallProcess(pexec.NewExecutable("pip"))
	siteProcess := pipenv.NewSiteProcess(pexec.NewExecutable("python"))
	logs := scribe.NewEmitter(os.Stdout)

	packit.Run(
		pipenv.Detect(),
		pipenv.Build(entryResolver, dependencyManager, installProcess, siteProcess, logs, chronos.DefaultClock),
	)
}
