package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/buildpack/libbuildpack/buildplan"
	"github.com/cloudfoundry/libcfbuildpack/detect"
	"github.com/cloudfoundry/libcfbuildpack/helper"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/paketo-community/pipenv/pipenv"
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

			var versionSource, pythonVersion string
			Expect(factory.Plans.Plan).To(Equal(
				buildplan.Plan{
					Provides: []buildplan.Provided{
						{
							Name: pipenv.Dependency,
						},
						{
							Name: pipenv.RequirementsLayer,
						},
					},
					Requires: []buildplan.Required{
						{
							Name:     pipenv.PythonLayer,
							Version:  pythonVersion,
							Metadata: buildplan.Metadata{"build": true, "version-source": versionSource},
						},
						{
							Name:     pipenv.Dependency,
							Metadata: buildplan.Metadata{"build": true},
						},
					},
				}))
		})

		it("has a requirements.txt", func() {
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "requirements.txt"), 0666, "")).To(Succeed())
			code, err := runDetect(factory.Detect)
			Expect(err).To(HaveOccurred())
			Expect(code).To(Equal(detect.FailStatusCode))
		})

	})

	when("there is a Pipfile.lock", func() {
		var pythonVersion = "some-python-version"

		it.Before(func() {
			pipfileLock := fmt.Sprintf(`
			{
			 "_meta": {
			     "requires": {
			         "python_version": "%s"
			     }
			 }
			}`, pythonVersion)

			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile.lock"), 0666, pipfileLock)).To(Succeed())
			Expect(helper.WriteFile(filepath.Join(factory.Detect.Application.Root, "Pipfile"), 0666, "")).To(Succeed())
		})

		it("passes, and requires a python version", func() {
			code, err := runDetect(factory.Detect)
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(detect.PassStatusCode))

			versionSource := pipenv.LockFile
			Expect(factory.Plans.Plan).To(Equal(
				buildplan.Plan{
					Provides: []buildplan.Provided{
						{
							Name: pipenv.Dependency,
						},
						{
							Name: pipenv.RequirementsLayer,
						},
					},
					Requires: []buildplan.Required{
						{
							Name:     pipenv.PythonLayer,
							Version:  pythonVersion,
							Metadata: buildplan.Metadata{"build": true, "version-source": versionSource},
						},
						{
							Name:     pipenv.Dependency,
							Metadata: buildplan.Metadata{"build": true},
						},
					},
				}))
		})
	})
}
