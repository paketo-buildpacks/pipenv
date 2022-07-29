package pipenv_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/fs"

	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/paketo-buildpacks/pipenv"
	"github.com/paketo-buildpacks/pipenv/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir string
		cnbDir    string

		installProcess *fakes.InstallProcess
		siteProcess    *fakes.SitePackageProcess
		sbomGenerator  *fakes.SBOMGenerator

		buffer *bytes.Buffer

		logEmitter scribe.Emitter

		build        packit.BuildFunc
		buildContext packit.BuildContext
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		Expect(fs.Copy("buildpack.toml", filepath.Join(cnbDir, "buildpack.toml"))).To(Succeed())

		installProcess = &fakes.InstallProcess{}
		siteProcess = &fakes.SitePackageProcess{}

		// Syft SBOM
		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		buffer = bytes.NewBuffer(nil)
		logEmitter = scribe.NewEmitter(buffer)

		siteProcess.ExecuteCall.Returns.String = filepath.Join(layersDir, "pipenv", "lib", "python3.8", "site-packages")

		build = pipenv.Build(
			installProcess,
			siteProcess,
			sbomGenerator,
			logEmitter,
			chronos.DefaultClock,
		)

		buildContext = packit.BuildContext{
			BuildpackInfo: packit.BuildpackInfo{
				Name:        "Some Buildpack",
				Version:     "some-version",
				SBOMFormats: []string{sbom.CycloneDXFormat, sbom.SPDXFormat},
			},
			CNBPath: cnbDir,
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name: "pipenv",
					},
				},
			},
			Platform: packit.Platform{Path: "some-platform-path"},
			Layers:   packit.Layers{Path: layersDir},
			Stack:    "some-stack",
		}
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	it("returns a result that installs pipenv", func() {
		result, err := build(buildContext)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(HaveLen(1))
		layer := result.Layers[0]

		Expect(layer.Name).To(Equal("pipenv"))

		Expect(layer.Path).To(Equal(filepath.Join(layersDir, "pipenv")))

		Expect(layer.SharedEnv).To(HaveLen(2))
		Expect(layer.SharedEnv["PYTHONPATH.delim"]).To(Equal(":"))
		Expect(layer.SharedEnv["PYTHONPATH.prepend"]).To(Equal(filepath.Join(layersDir, "pipenv", "lib/python3.8/site-packages")))

		Expect(layer.BuildEnv).To(BeEmpty())
		Expect(layer.LaunchEnv).To(BeEmpty())
		Expect(layer.ProcessLaunchEnv).To(BeEmpty())

		Expect(layer.Build).To(BeFalse())
		Expect(layer.Launch).To(BeFalse())
		Expect(layer.Cache).To(BeFalse())

		Expect(layer.Metadata).To(HaveLen(1))
		Expect(layer.Metadata[pipenv.PipenvVersion]).To(Equal("2022.7.24"))

		Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
			{
				Extension: sbom.Format(sbom.CycloneDXFormat).Extension(),
				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
			},
			{
				Extension: sbom.Format(sbom.SPDXFormat).Extension(),
				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
			},
		}))

		Expect(installProcess.ExecuteCall.Receives.Version).To(ContainSubstring("2022.7.24"))
		Expect(installProcess.ExecuteCall.Receives.DestLayerPath).To(Equal(filepath.Join(layersDir, "pipenv")))
	})

	context("when build plan entries require pipenv at build/launch", func() {
		it.Before(func() {
			buildContext.Plan.Entries[0].Metadata = make(map[string]interface{})
			buildContext.Plan.Entries[0].Metadata["build"] = true
			buildContext.Plan.Entries[0].Metadata["launch"] = true
		})

		it("makes the layer available at the right times", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(1))
			layer := result.Layers[0]

			Expect(layer.Name).To(Equal("pipenv"))

			Expect(layer.Build).To(BeTrue())
			Expect(layer.Launch).To(BeTrue())
			Expect(layer.Cache).To(BeTrue())
		})
	})

	context("when rebuilding a layer", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(layersDir, fmt.Sprintf("%s.toml", pipenv.Pipenv)), []byte(fmt.Sprintf(`[metadata]
			%s = "2022.7.24"
			built_at = "some-build-time"
			`, pipenv.PipenvVersion)), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			buildContext.Plan.Entries[0].Metadata = make(map[string]interface{})
			buildContext.Plan.Entries[0].Metadata["build"] = true
			buildContext.Plan.Entries[0].Metadata["launch"] = false
		})

		it("skips the build process if the cached dependency sha matches the selected dependency sha", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(1))
			layer := result.Layers[0]

			Expect(layer.Name).To(Equal("pipenv"))

			Expect(layer.Build).To(BeTrue())
			Expect(layer.Launch).To(BeFalse())
			Expect(layer.Cache).To(BeTrue())

			Expect(buffer.String()).To(ContainSubstring("Reusing cached layer"))

			Expect(installProcess.ExecuteCall.CallCount).To(Equal(0))
		})
	})

	context("failure cases", func() {
		context("when pipenv layer cannot be fetched", func() {
			it.Before(func() {
				Expect(os.Chmod(layersDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when pipenv layer cannot be reset", func() {
			it.Before(func() {
				Expect(os.MkdirAll(filepath.Join(layersDir, pipenv.Pipenv), os.ModePerm))
				Expect(os.Chmod(layersDir, 0500)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when dependency cannot be installed", func() {
			it.Before(func() {
				installProcess.ExecuteCall.Returns.Error = errors.New("failed to install dependency")
			})
			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("failed to install dependency")))
			})
		})

		context("when the site packages cannot be found", func() {
			it.Before(func() {
				siteProcess.ExecuteCall.Returns.Error = errors.New("failed to find site-packages dir")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("failed to find site-packages dir")))
			})
		})

		context("when the layer does not have a site-packages directory", func() {
			it.Before(func() {
				siteProcess.ExecuteCall.Returns.String = ""
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("pipenv installation failed: site packages are missing from the pipenv layer")))
			})
		})

		context("when generating the SBOM returns an error", func() {
			it.Before(func() {
				buildContext.BuildpackInfo.SBOMFormats = []string{"random-format"}
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(`unsupported SBOM format: 'random-format'`))
			})
		})

		context("when formatting the SBOM returns an error", func() {
			it.Before(func() {
				sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate SBOM")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("failed to generate SBOM")))
			})
		})
	})
}
