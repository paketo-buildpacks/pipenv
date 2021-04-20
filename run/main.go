package main

import (
	"os"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/cargo"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/draft"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/paketo-buildpacks/packit/scribe"
	"github.com/paketo-community/pipenv"
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
