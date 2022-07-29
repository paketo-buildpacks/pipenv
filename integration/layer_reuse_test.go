package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testLayerReuse(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker

		imageIDs     map[string]struct{}
		containerIDs map[string]struct{}

		name   string
		source string
	)

	it.Before(func() {
		var err error
		name, err = occam.RandomName()
		Expect(err).NotTo(HaveOccurred())

		pack = occam.NewPack()
		docker = occam.NewDocker()

		imageIDs = map[string]struct{}{}
		containerIDs = map[string]struct{}{}
	})

	it.After(func() {
		for id := range containerIDs {
			Expect(docker.Container.Remove.Execute(id)).To(Succeed())
		}

		for id := range imageIDs {
			Expect(docker.Image.Remove.Execute(id)).To(Succeed())
		}

		Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())

		Expect(os.RemoveAll(source)).To(Succeed())
	})

	context("when the app is rebuilt and the same pipenv version is required", func() {
		it("reuses the cached pipenv layer", func() {
			var (
				err    error
				logs   fmt.Stringer
				source string

				firstImage  occam.Image
				secondImage occam.Image

				secondContainer occam.Container
			)

			source, err = occam.Source(filepath.Join("testdata", "default_app"))
			Expect(err).ToNot(HaveOccurred())

			firstImage, logs, err = pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					settings.Buildpacks.CPython,
					settings.Buildpacks.Pip,
					settings.Buildpacks.Pipenv,
					settings.Buildpacks.BuildPlan,
				).
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), logs.String)

			imageIDs[firstImage.ID] = struct{}{}

			secondImage, logs, err = pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					settings.Buildpacks.CPython,
					settings.Buildpacks.Pip,
					settings.Buildpacks.Pipenv,
					settings.Buildpacks.BuildPlan,
				).
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), logs.String)

			imageIDs[secondImage.ID] = struct{}{}

			Expect(logs).To(ContainLines(
				fmt.Sprintf("  Reusing cached layer /layers/%s/pipenv", strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_")),
			))

			secondContainer, err = docker.Container.Run.
				WithCommand("pipenv --version").
				Execute(secondImage.ID)
			Expect(err).ToNot(HaveOccurred())

			containerIDs[secondContainer.ID] = struct{}{}

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(secondContainer.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(MatchRegexp(`pipenv, version \d+\.\d+\.\d+`))

			Expect(secondImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
			Expect(secondImage.Buildpacks[2].Layers["pipenv"].SHA).To(Equal(firstImage.Buildpacks[2].Layers["pipenv"].SHA))
		})
	})

	context("when the app is rebuilt and a different pipenv version is required", func() {
		it("rebuilds", func() {
			var (
				err    error
				logs   fmt.Stringer
				source string

				firstImage  occam.Image
				secondImage occam.Image

				secondContainer occam.Container
			)

			source, err = occam.Source(filepath.Join("testdata", "default_app"))
			Expect(err).ToNot(HaveOccurred())

			firstImage, logs, err = pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					settings.Buildpacks.CPython,
					settings.Buildpacks.Pip,
					settings.Buildpacks.Pipenv,
					settings.Buildpacks.BuildPlan,
				).
				WithEnv(map[string]string{"BP_PIPENV_VERSION": buildpackInfo.Metadata.DefaultVersions.Pipenv}).
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), logs.String)

			secondImage, logs, err = pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					settings.Buildpacks.CPython,
					settings.Buildpacks.Pip,
					settings.Buildpacks.Pipenv,
					settings.Buildpacks.BuildPlan,
				).
				WithEnv(map[string]string{"BP_PIPENV_VERSION": "2022.7.4"}).
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), logs.String)

			imageIDs[secondImage.ID] = struct{}{}

			Expect(logs).To(ContainLines(
				MatchRegexp(fmt.Sprintf(`%s \d+\.\d+\.\d+`, buildpackInfo.Buildpack.Name)),
				"  Resolving Pipenv version",
				"    Candidate version sources (in priority order):",
				`      BP_PIPENV_VERSION -> "2022.7.4"`,
				`      default-versions  -> "2022.7.24"`,
				`      <unknown>         -> ""`,
			))
			Expect(logs).To(ContainLines(
				`    Selected Pipenv version (using BP_PIPENV_VERSION): 2022.7.4`,
			))
			Expect(logs).To(ContainLines(
				"  Executing build process",
				MatchRegexp(`    Installing Pipenv \d+\.\d+\.\d+`),
				MatchRegexp(`      Completed in ([0-9]*(\.[0-9]*)?[a-z]+)+`),
			))
			Expect(logs).To(ContainLines(
				"  Configuring build environment",
				MatchRegexp(fmt.Sprintf(`    PYTHONPATH -> "\/layers\/%s\/pipenv\/lib\/python\d+\.\d+\/site-packages:\$PYTHONPATH"`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))),
				"",
				"  Configuring launch environment",
				MatchRegexp(fmt.Sprintf(`    PYTHONPATH -> "\/layers\/%s\/pipenv\/lib\/python\d+\.\d+\/site-packages:\$PYTHONPATH"`, strings.ReplaceAll(buildpackInfo.Buildpack.ID, "/", "_"))),
			))

			secondContainer, err = docker.Container.Run.
				WithCommand("pipenv --version").
				Execute(secondImage.ID)
			Expect(err).ToNot(HaveOccurred())

			containerIDs[secondContainer.ID] = struct{}{}

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(secondContainer.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(MatchRegexp(`pipenv, version \d+\.\d+\.\d+`))

			Expect(secondImage.Buildpacks[2].Key).To(Equal(buildpackInfo.Buildpack.ID))
			Expect(secondImage.Buildpacks[2].Layers["pipenv"].SHA).ToNot(Equal(firstImage.Buildpacks[2].Layers["pipenv"].SHA))
		})
	})
}
