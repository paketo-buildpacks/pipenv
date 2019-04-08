package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/dagger"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	spec.Run(t, "Integration", testIntegration, spec.Report(report.Terminal{}))
}

func testIntegration(t *testing.T, when spec.G, it spec.S) {
	var (
		pythonURI, pipURI, pipenvURI string
		err                          error
	)
	it.Before(func() {
		RegisterTestingT(t)

		pythonURI, err = dagger.GetLatestBuildpack("python-cnb")
		Expect(err).ToNot(HaveOccurred())

		pipURI, err = dagger.GetLatestBuildpack("pip-cnb")
		Expect(err).ToNot(HaveOccurred())

		pipenvURI, err = dagger.PackageBuildpack()
		Expect(err).ToNot(HaveOccurred())
	})

	it.After(func() {
		for _, path := range []string{pythonURI, pipURI, pipenvURI} {
			if path != "" {
				Expect(os.RemoveAll(path)).To(Succeed())
			}
		}
	})

	when("building a simple pipenv app without a pipfile lock", func() {
		it("builds and runs", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "without_pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			app.Env["PORT"] = "8080"
			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World with pipenv!"))

			Expect(app.Destroy()).To(Succeed())
		})
	})

	when("building a simple pipenv app with a pipfile lock", func() {
		it("builds and runs", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			app.Env["PORT"] = "8080"
			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World with pipenv!"))

			Expect(app.Destroy()).To(Succeed())
		})

		it("sets python version to version in pipfile.lock", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			app.SetHealthCheck("", "3s", "1s")
			app.Env["PORT"] = "8080"
			err = app.Start()
			Expect(err).ToNot(HaveOccurred())
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World with pipenv!"))

			Expect(app.BuildLogs()).To(ContainSubstring("Python 3.7.2"))
			Expect(app.Destroy()).To(Succeed())
		})
	})

	when("building a simple pipenv app with a pipfile and requirements.txt", func() {
		it("ignores the pipfile", func() {
			_, err := dagger.PackBuild(filepath.Join("testdata", "pipfile_requirements"), pythonURI, pipenvURI, pipURI)

			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("Pipenv Buildpack: fail"))

		})
	})

	when("rebuilding a simple pipenv app", func() {
		it("should cache the pipenv binary but not the requirements.txt", func() {
			app, err := dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			app.SetHealthCheck("", "3s", "1s")
			app.Env["PORT"] = "8080"
			err = app.Start()
			Expect(err).ToNot(HaveOccurred())

			_, imgName, _, _ := app.Info()

			rebuiltApp, err := dagger.PackBuildNamedImage(imgName, filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			Expect(rebuiltApp.BuildLogs()).ToNot(ContainSubstring("Downloading from https://buildpacks.cloudfoundry.org/dependencies/pipenv/pipenv"))
			Expect(rebuiltApp.BuildLogs()).To(MatchRegexp("Pipenv (\\S)+: Reusing cached layer"))
			Expect(rebuiltApp.BuildLogs()).To(ContainSubstring("Generating requirements.txt from Pipfile.lock"))
			files, err := rebuiltApp.Files(filepath.Join("/workspace", "requirements.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(ContainElement(ContainSubstring("requirements.txt")))
			Expect(rebuiltApp.Destroy()).To(Succeed())
		})
	})

	when("when building an app without a pipfile", func() {
		it("should fail during detection", func() {
			_, err := dagger.PackBuild(filepath.Join("testdata", "without_pipfile"), pythonURI, pipenvURI, pipURI)

			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("Pipenv Buildpack: fail"))
		})
	})
}
