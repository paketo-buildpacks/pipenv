package main

import (
	"bytes"
	"path/filepath"
	"testing"

	v2Logger "github.com/buildpack/libbuildpack/logger"
	v3Logger "github.com/cloudfoundry/libcfbuildpack/logger"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/detect"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/test"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitDetect(t *testing.T) {
	spec.Run(t, "Detect", testDetect, spec.Report(report.Terminal{}))
}

func testDetect(t *testing.T, when spec.G, it spec.S) {
	var factory *test.DetectFactory

	it.Before(func() {
		RegisterTestingT(t)
		factory = test.NewDetectFactory(t)
	})

	when("there is no Pipfile", func() {
		it("should fail", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).ToNot(HaveOccurred())
			Expect(code).To(Equal(detect.FailStatusCode))
		})
	})

	when("there is a Pipfile", func() {
		it.Before(func() {
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile"), 0666, "")).To(Succeed())
		})

		it("passes with pipenv and python for build and launch", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(detect.PassStatusCode))
			Expect(factory.Output).To(Equal(buildplan.BuildPlan{
				"pipenv": buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"build": true,
					},
				},
				"python": buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"build":  true,
						"launch": true,
					},
				},
				"python_packages": buildplan.Dependency{
					Metadata: nil,
				},
				"python_packages_cache": buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"cacheable": nil,
					},
				},
			}))
		})

		it("has a requirements.txt", func() {
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "requirements.txt"), 0666, "")).To(Succeed())
			code, err := runDetect(factory.Detect)
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(detect.FailStatusCode))

		})

	})

	when("there is a Pipfile.lock", func() {
		it.Before(func() {
			pipfileLock := `
			{
			 "_meta": {
			     "requires": {
			         "python_version": "some-python-version"
			     }
			 }
			}`
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile.lock"), 0666, pipfileLock)).To(Succeed())
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile"), 0666, "")).To(Succeed())
		})

		it("passes, and adds a cacheable sha to the python_packages buildplan", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(detect.PassStatusCode))
			Expect(factory.Output).To(HaveKeyWithValue("python_packages_cache", buildplan.Dependency{
				Metadata: buildplan.Metadata{
					"cacheable": "8e9cdde2f43fd066709a5e012dee4f08fc0808cdcdbe72e0a3e71c9fb2a4abd2",
				}}))
		})
	})

	when("the python version in the context and Pipfile.lock match", func() {
		it.Before(func() {
			pipfileLock := `
			{
			 "_meta": {
			     "requires": {
			         "python_version": "some-python-version"
			     }
			 }
			}`
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile.lock"), 0666, pipfileLock)).To(Succeed())
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile"), 0666, "")).To(Succeed())
			factory.Detect.BuildPlan = buildplan.BuildPlan{
				"python": buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"launch": true,
						"build":  true,
					},
					Version: "some-python-version",
				},
			}
		})

		it("the python version should be in the buildplan and there should be no warning", func() {
			_, err := runDetect(factory.Detect)
			Expect(err).ToNot(HaveOccurred())
			Expect(factory.Output).To(HaveKeyWithValue("python", buildplan.Dependency{
				Metadata: buildplan.Metadata{
					"build":  true,
					"launch": true,
				},
				Version: "some-python-version",
			}))
			Expect(factory.Detect.Logger.String()).ToNot(ContainSubstring("There is a mismatch of your python version between your context and Pipfile.lock"))
		})
	})

	when("the python version in the context and Pipfile.lock are different", func() {
		var (
			buf = bytes.Buffer{}
		)

		it.Before(func() {
			factory.Detect.Logger = v3Logger.Logger{Logger: v2Logger.NewLogger(&buf, &buf)}

			pipfileLock := `
			{
			 "_meta": {
			     "requires": {
			         "python_version": "python-version-in-pipfile.lock"
			     }
			 }
			}`
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile.lock"), 0666, pipfileLock)).To(Succeed())
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile"), 0666, "")).To(Succeed())
			factory.Detect.BuildPlan = buildplan.BuildPlan{
				"python": buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"launch": true,
						"build":  true,
					},
					Version: "python-version-in-context",
				},
			}
		})

		it("the context's python version should be in the buildplan and there should be a warning", func() {
			_, err := runDetect(factory.Detect)
			Expect(err).ToNot(HaveOccurred())
			Expect(factory.Output).To(HaveKeyWithValue("python",
				buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"build":  true,
						"launch": true,
					},
					Version: "python-version-in-context",
				}))

			Expect(buf.String()).To(ContainSubstring("There is a mismatch of your python version between your buildpack.yml and Pipfile.lock"))
		})
	})

	// version in pipfile, but not in buildplan
	when("there is a python version in the Pipfile.lock and no python version in the context", func() {
		it.Before(func() {
			pipfileLock := `
			{
			 "_meta": {
			     "requires": {
			         "python_version": "python-version-in-pipfile.lock"
			     }
			 }
			}`
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile.lock"), 0666, pipfileLock)).To(Succeed())
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile"), 0666, "")).To(Succeed())
			factory.Detect.BuildPlan = buildplan.BuildPlan{
				"python": buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"launch": true,
						"build":  true,
					},
					Version: "",
				},
			}
		})

		it("uses the Pipfile.lock's python version", func() {
			_, err := runDetect(factory.Detect)
			Expect(err).ToNot(HaveOccurred())
			Expect(factory.Output)
			Expect(factory.Output).To(HaveKeyWithValue("python", buildplan.Dependency{
				Metadata: buildplan.Metadata{
					"build":  true,
					"launch": true,
				},
				Version: "python-version-in-pipfile.lock",
			}))
		})
	})

	// version in buildplan, but not in pipfile
	when("there is a python version in the context and no python version in the Pipfile.lock", func() {
		var (
			buf = bytes.Buffer{}
		)

		it.Before(func() {
			factory.Detect.Logger = v3Logger.Logger{Logger: v2Logger.NewLogger(&buf, &buf)}

			pipfileLock := `
			{
			 "_meta": {
			     "requires": {
			     }
			 }
			}`
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile.lock"), 0666, pipfileLock)).To(Succeed())
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile"), 0666, "")).To(Succeed())
			factory.Detect.BuildPlan = buildplan.BuildPlan{
				"python": buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"launch": true,
						"build":  true,
					},
					Version: "python-version-in-context",
				},
			}
		})

		it("uses the context's python version and does not print a warning", func() {
			_, err := runDetect(factory.Detect)
			Expect(err).ToNot(HaveOccurred())
			Expect(factory.Output).To(HaveKeyWithValue("python",
				buildplan.Dependency{
					Metadata: buildplan.Metadata{
						"build":  true,
						"launch": true,
					},
					Version: "python-version-in-context",
				}))

			Expect(buf.String()).ToNot(ContainSubstring("There is a mismatch of your python version between either your buildpack.yml and Pipfile.lock"))
		})
	})

}
