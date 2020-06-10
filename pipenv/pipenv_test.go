package pipenv_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/libcfbuildpack/helper"

	"github.com/golang/mock/gomock"

	"github.com/paketo-community/pipenv/pipenv"

	"github.com/cloudfoundry/libcfbuildpack/buildpackplan"
	"github.com/cloudfoundry/libcfbuildpack/test"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

//go:generate mockgen -destination mocks_test.go  -package pipenv_test github.com/cloudfoundry/libcfbuildpack/runner Runner

func TestUnitPipenv(t *testing.T) {
	spec.Run(t, "Contributor", testPipenv, spec.Report(report.Terminal{}))
}

func testPipenv(t *testing.T, when spec.G, it spec.S) {
	var (
		f          *test.BuildFactory
		mockCtrl   *gomock.Controller
		mockRunner *MockRunner
	)
	it.Before(func() {
		RegisterTestingT(t)
		mockCtrl = gomock.NewController(t)
		mockRunner = NewMockRunner(mockCtrl)

		f = test.NewBuildFactory(t)
	})
	it.After(func() {
		mockCtrl.Finish()
	})

	when("modules.NewContributor", func() {
		it("does not contribute when pipenv is not in the build plan", func() {
			_, willContribute, err := pipenv.NewContributor(f.Build, mockRunner)
			Expect(err).ToNot(HaveOccurred())
			Expect(willContribute).To(BeFalse())
		})

		it("does contribute when pipenv is in the buildplan", func() {
			f.AddPlan(buildpackplan.Plan{
				Name:     pipenv.Dependency,
				Metadata: buildpackplan.Metadata{"build": true},
			})

			_, willContribute, err := pipenv.NewContributor(f.Build, mockRunner)

			Expect(err).ToNot(HaveOccurred())
			Expect(willContribute).To(BeTrue())
		})
	})

	when("ContributePipenv", func() {
		it("installs pipenv", func() {
			mockRunner.EXPECT().Run("python", f.Build.Layers.Layer(pipenv.Dependency).Root, "-m", "pip", "install", "pipenv", "--find-links="+f.Build.Layers.Layer(pipenv.Dependency).Root)
			pipenvStub := filepath.Join("testdata", "stub-pipenv.tar.gz")

			f.AddPlan(buildpackplan.Plan{
				Name:     pipenv.Dependency,
				Metadata: buildpackplan.Metadata{"build": true},
			})
			f.AddDependency(pipenv.Dependency, pipenvStub)

			contributor, _, err := pipenv.NewContributor(f.Build, mockRunner)

			Expect(err).ToNot(HaveOccurred())
			Expect(contributor.ContributePipenv()).To(Succeed())

			layer := f.Build.Layers.Layer("pipenv")
			Expect(layer).To(test.HaveLayerMetadata(true, true, false))
			Expect(filepath.Join(layer.Root, "stub-dir", "stub.txt")).To(BeARegularFile())
		})
	})

	when("ContributeRequirementsTxt", func() {
		it("generates a requirements.txt", func() {
			mockRunner.EXPECT().Run("pipenv", f.Build.Application.Root, "lock", "--requirements")
			mockRunner.EXPECT().RunWithOutput("pipenv", f.Build.Application.Root, "lock", "--requirements")
			pipenvStub := filepath.Join("testdata", "stub-pipenv.tar.gz")
			f.AddPlan(buildpackplan.Plan{
				Name:     pipenv.Dependency,
				Metadata: buildpackplan.Metadata{"build": true},
			})
			f.AddDependency(pipenv.Dependency, pipenvStub)

			contributor, _, err := pipenv.NewContributor(f.Build, mockRunner)

			Expect(err).ToNot(HaveOccurred())
			Expect(contributor.ContributeRequirementsTxt()).To(Succeed())
			Expect(filepath.Join(f.Build.Application.Root, "requirements.txt")).To(BeAnExistingFile())
		})
	})

	when("GetPythonVersionFromPipfileLock for pipfile.lock", func() {
		it("reads pipfile.lock and extracts Python version", func() {
			tmpFile, err := ioutil.TempFile("", "Pipfile.lock")
			Expect(err).NotTo(HaveOccurred())
			lock := pipenv.PipfileLock{}
			lock.Meta.Requires.Version = "1.2.3"
			lockString, err := json.Marshal(lock)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(tmpFile.Name(), []byte(lockString), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			version, err := pipenv.GetPythonVersionFromPipfileLock(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.2.3"))
		})

		it("reads pipfile.lock without Python version", func() {
			tmpFile, err := ioutil.TempFile("", "Pipfile.lock")
			Expect(err).NotTo(HaveOccurred())
			lock := pipenv.PipfileLock{}
			lockString, err := json.Marshal(lock)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(tmpFile.Name(), []byte(lockString), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			version, err := pipenv.GetPythonVersionFromPipfileLock(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
		})
	})

	when("GetPythonVersionFromPipfileLock for pipfile", func() {
		it("reads pipfile and extracts Python version", func() {
			tmpFile, err := ioutil.TempFile("", "Pipfile.lock")
			Expect(err).NotTo(HaveOccurred())

			lock := pipenv.PipfileLock{}
			lock.Meta.Requires.Version = "1.2.3"
			lockString, err := json.Marshal(lock)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(tmpFile.Name(), []byte(lockString), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			version, err := pipenv.GetPythonVersionFromPipfileLock(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.2.3"))
		})

		it("reads pipfile.lock without Python version", func() {
			tmpFile, err := ioutil.TempFile("", "Pipfile.lock")
			Expect(err).NotTo(HaveOccurred())
			lock := pipenv.PipfileLock{}
			lockString, err := json.Marshal(lock)
			Expect(err).NotTo(HaveOccurred())
			err = ioutil.WriteFile(tmpFile.Name(), []byte(lockString), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			version, err := pipenv.GetPythonVersionFromPipfileLock(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
		})
	})

	when("Generating Requirements.txt from Pipfile.lock", func() {
		it("return contents of Requirements.txt to be saved", func() {
			f.AddPlan(buildpackplan.Plan{
				Name:     pipenv.Dependency,
				Metadata: buildpackplan.Metadata{"build": true},
			})

			lockPath := filepath.Join(f.Build.Application.Root, "Pipfile.lock")
			Expect(helper.CopyFile(filepath.Join("testdata", "Pipfile.lock"), lockPath)).To(Succeed())
			defer os.Remove(lockPath)

			cont, _, _ := pipenv.NewContributor(f.Build, mockRunner)
			Expect(cont.ContributeRequirementsTxt()).To(Succeed())
			requirementsTxtFile := filepath.Join(f.Build.Application.Root, "requirements.txt")
			Expect(requirementsTxtFile).To(BeAnExistingFile())
			requirementsTxtContents, err := ioutil.ReadFile(requirementsTxtFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(requirementsTxtContents)).ToNot(Equal(""))
			Expect(string(requirementsTxtContents)).To(ContainSubstring("flask==1.0.2"))
		})
	})
}
