package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/occam"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testVersions(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		pack   occam.Pack
		docker occam.Docker
	)

	it.Before(func() {
		pack = occam.NewPack()
		docker = occam.NewDocker()
	})

	context("when the buildpack is run with pack build", func() {
		var (
			name   string
			source string

			containersMap map[string]interface{}
			imagesMap     map[string]interface{}
		)

		it.Before(func() {
			var err error
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())

			containersMap = map[string]interface{}{}
			imagesMap = map[string]interface{}{}
		})

		it.After(func() {
			for containerID := range containersMap {
				Expect(docker.Container.Remove.Execute(containerID)).To(Succeed())
			}
			for imageID := range imagesMap {
				Expect(docker.Image.Remove.Execute(imageID)).To(Succeed())
			}
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it("builds and runs successfully with both provided dependency versions", func() {
			var err error

			source, err = occam.Source(filepath.Join("testdata", "default_app"))
			Expect(err).NotTo(HaveOccurred())

			firstPipenvVersion := buildpackInfo.Metadata.Dependencies[0].Version
			secondPipenvVersion := buildpackInfo.Metadata.Dependencies[1].Version

			Expect(firstPipenvVersion).NotTo(Equal(secondPipenvVersion))

			firstImage, firstLogs, err := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					settings.Buildpacks.CPython,
					settings.Buildpacks.Pip,
					settings.Buildpacks.Pipenv,
					settings.Buildpacks.BuildPlan,
				).
				WithEnv(map[string]string{"BP_PIPENV_VERSION": firstPipenvVersion}).
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), firstLogs.String)

			imagesMap[firstImage.ID] = nil

			Expect(firstLogs).To(ContainLines(
				ContainSubstring(fmt.Sprintf(`Selected Pipenv version (using BP_PIPENV_VERSION): %s`, firstPipenvVersion)),
			))

			firstContainer, err := docker.Container.Run.
				WithCommand("pipenv --version").
				Execute(firstImage.ID)
			Expect(err).ToNot(HaveOccurred())

			containersMap[firstContainer.ID] = nil

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(firstContainer.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(ContainSubstring(fmt.Sprintf(`pipenv, version %s`, firstPipenvVersion)))

			secondImage, secondLogs, err := pack.WithNoColor().Build.
				WithPullPolicy("never").
				WithBuildpacks(
					settings.Buildpacks.CPython,
					settings.Buildpacks.Pip,
					settings.Buildpacks.Pipenv,
					settings.Buildpacks.BuildPlan,
				).
				WithEnv(map[string]string{"BP_PIPENV_VERSION": secondPipenvVersion}).
				Execute(name, source)
			Expect(err).ToNot(HaveOccurred(), secondLogs.String)

			imagesMap[secondImage.ID] = nil

			Expect(secondLogs).To(ContainLines(
				ContainSubstring(fmt.Sprintf(`Selected Pipenv version (using BP_PIPENV_VERSION): %s`, secondPipenvVersion)),
			))

			secondContainer, err := docker.Container.Run.
				WithCommand("pipenv --version").
				Execute(secondImage.ID)
			Expect(err).ToNot(HaveOccurred())

			containersMap[secondContainer.ID] = nil

			Eventually(func() string {
				cLogs, err := docker.Container.Logs.Execute(secondContainer.ID)
				Expect(err).NotTo(HaveOccurred())
				return cLogs.String()
			}).Should(ContainSubstring(fmt.Sprintf(`pipenv, version %s`, secondPipenvVersion)))
		})
	})
}
