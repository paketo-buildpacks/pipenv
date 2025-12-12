package pipenv_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/pipenv"
	"github.com/paketo-buildpacks/pipenv/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testPipenvInstallProcess(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		version       = "1.2.3-some.version"
		destLayerPath string
		executable    *fakes.Executable

		pipenvInstallProcess pipenv.PipenvInstallProcess
	)

	it.Before(func() {
		destLayerPath = t.TempDir()

		executable = &fakes.Executable{}

		pipenvInstallProcess = pipenv.NewPipenvInstallProcess(executable)
	})

	context("Execute", func() {
		context("there is a pipenv dependency to install", func() {
			it("installs it to the pipenv layer", func() {
				err := pipenvInstallProcess.Execute(version, destLayerPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(executable.ExecuteCall.Receives.Execution.Env).To(Equal(append(os.Environ(), fmt.Sprintf("PYTHONUSERBASE=%s", destLayerPath))))
				Expect(executable.ExecuteCall.Receives.Execution.Args).To(Equal([]string{"install", "pipenv==1.2.3-some.version", "--user"}))
			})
		})

		context("failure cases", func() {
			context("the install process fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						_, err := fmt.Fprintln(execution.Stdout, "stdout output")
						Expect(err).NotTo(HaveOccurred())
						_, err = fmt.Fprintln(execution.Stderr, "stderr output")
						Expect(err).NotTo(HaveOccurred())
						return errors.New("installing pipenv failed")
					}
				})

				it("returns an error", func() {
					err := pipenvInstallProcess.Execute(version, destLayerPath)
					Expect(err).To(MatchError(ContainSubstring("installing pipenv failed")))
					Expect(err).To(MatchError(ContainSubstring("stdout output")))
					Expect(err).To(MatchError(ContainSubstring("stderr output")))
				})
			})
		})
	})
}
