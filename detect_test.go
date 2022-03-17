package pipenv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/pipenv"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir string
		detect     packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(filepath.Join(workingDir, "Pipfile"), []byte{}, 0644)
		Expect(err).NotTo(HaveOccurred())

		detect = pipenv.Detect()
	})

	it("returns a plan that provides pipenv", func() {
		result, err := detect(packit.DetectContext{
			WorkingDir: workingDir,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: "pipenv"},
				},
				Requires: []packit.BuildPlanRequirement{
					{
						Name: pipenv.Pip,
						Metadata: pipenv.BuildPlanMetadata{
							Build:  true,
							Launch: false,
						},
					},
					{
						Name: pipenv.CPython,
						Metadata: pipenv.BuildPlanMetadata{
							Build:  true,
							Launch: false,
						},
					},
				},
			},
		}))
	})

	context("when BP_PIPENV_VERSION is set", func() {
		it.Before(func() {
			os.Setenv("BP_PIPENV_VERSION", "1.2.3")
		})

		it.After(func() {
			_ = os.Unsetenv("BP_PIPENV_VERSION")
		})
		it("returns a plan that provides a specific pipenv version", func() {
			result, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.DetectResult{
				Plan: packit.BuildPlan{
					Provides: []packit.BuildPlanProvision{
						{Name: "pipenv"},
					},
					Requires: []packit.BuildPlanRequirement{
						{
							Name: pipenv.Pip,
							Metadata: pipenv.BuildPlanMetadata{
								Build:  true,
								Launch: false,
							},
						},
						{
							Name: pipenv.CPython,
							Metadata: pipenv.BuildPlanMetadata{
								Build:  true,
								Launch: false,
							},
						},
						{
							Name: "pipenv",
							Metadata: pipenv.BuildPlanMetadata{
								Version:       "1.2.3",
								VersionSource: "BP_PIPENV_VERSION",
							},
						},
					},
				},
			}))
		})
	})

}
