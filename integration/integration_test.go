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
	it.Before(func() {
		RegisterTestingT(t)
		Expect(dagger.BuildCFLinuxFS3()).To(Succeed())
	})

	when("building a simple pipenv app without a pipfile lock", func() {
		it("builds and runs", func() {
			pythonBPPath, pipBPPath, pipenvBPPath := getBPPaths()

			app, err := dagger.PackBuild(filepath.Join("testdata", "without_pipfile_lock"), pythonBPPath, pipenvBPPath, pipBPPath)
			Expect(err).NotTo(HaveOccurred())

			//app.SetHealthCheck("", "3s", "1s")
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
			pythonBPPath, pipBPPath, pipenvBPPath := getBPPaths()

			app, err := dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonBPPath, pipenvBPPath, pipBPPath)
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
			pythonBPPath, pipBPPath, pipenvBPPath := getBPPaths()

			app, err := dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonBPPath, pipenvBPPath, pipBPPath)
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
			pythonBPPath, pipBPPath, pipenvBPPath := getBPPaths()

			_, err := dagger.PackBuild(filepath.Join("testdata", "pipfile_requirements"), pythonBPPath, pipenvBPPath, pipBPPath)

			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("Pipenv Buildpack: fail"))

		})
	})
}

func getBPPaths() (string, string, string) {
	pipenvBPPath, err := dagger.PackageBuildpack()
	Expect(err).ToNot(HaveOccurred())

	pythonBPPath, err := dagger.GetLatestBuildpack("python-cnb")
	Expect(err).ToNot(HaveOccurred())

	pipBPPath, err := dagger.GetLatestBuildpack("pip-cnb")
	Expect(err).ToNot(HaveOccurred())

	return pythonBPPath, pipBPPath, pipenvBPPath
}
