package pipenv_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/scribe"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/paketo-buildpacks/pipenv"
	"github.com/paketo-buildpacks/pipenv/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir         string
		cnbDir            string
		entryResolver     *fakes.EntryResolver
		dependencyManager *fakes.DependencyManager
		installProcess    *fakes.InstallProcess
		siteProcess       *fakes.SitePackageProcess
		buffer            *bytes.Buffer
		logEmitter        scribe.Emitter
		clock             chronos.Clock
		timeStamp         time.Time

		build packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = ioutil.TempDir("", "layers")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = ioutil.TempDir("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(cnbDir, "buildpack.toml"), []byte(`api = "0.2"
[buildpack]
  id = "org.some-org.some-buildpack"
  name = "Some Buildpack"
  version = "some-version"

[metadata]

  [[metadata.dependencies]]
		id = "pipenv"
    name = "Pipenv"
    sha256 = "some-sha"
    stacks = ["some-stack"]
    uri = "some-uri"
    version = "2020.11.4"
`), 0600)
		Expect(err).NotTo(HaveOccurred())

		entryResolver = &fakes.EntryResolver{}
		entryResolver.ResolveCall.Returns.BuildpackPlanEntry = packit.BuildpackPlanEntry{
			Name: "pipenv",
		}

		dependencyManager = &fakes.DependencyManager{}
		dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{
			ID:      "pipenv",
			Name:    "pipenv-dependency-name",
			SHA256:  "pipenv-dependency-sha",
			Stacks:  []string{"some-stack"},
			URI:     "pipenv-dependency-uri",
			Version: "pipenv-dependency-version",
		}

		dependencyManager.GenerateBillOfMaterialsCall.Returns.BOMEntrySlice = []packit.BOMEntry{
			{
				Name: "pipenv",
				Metadata: packit.BOMMetadata{
					Checksum: packit.BOMChecksum{
						Algorithm: packit.SHA256,
						Hash:      "pipenv-dependency-sha",
					},
					URI:     "pipenv-dependency-uri",
					Version: "pipenv-dependency-version",
				},
			},
		}

		installProcess = &fakes.InstallProcess{}
		siteProcess = &fakes.SitePackageProcess{}

		buffer = bytes.NewBuffer(nil)
		logEmitter = scribe.NewEmitter(buffer)

		timeStamp = time.Now()
		clock = chronos.NewClock(func() time.Time {
			return timeStamp
		})

		siteProcess.ExecuteCall.Returns.String = filepath.Join(layersDir, "pipenv", "lib", "python3.8", "site-packages")
		build = pipenv.Build(entryResolver, dependencyManager, installProcess, siteProcess, logEmitter, clock)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	it("returns a result that installs pipenv", func() {
		result, err := build(packit.BuildContext{
			BuildpackInfo: packit.BuildpackInfo{
				Name:    "Some Buildpack",
				Version: "some-version",
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
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(result).To(Equal(packit.BuildResult{
			Layers: []packit.Layer{
				{
					Name: "pipenv",
					Path: filepath.Join(layersDir, "pipenv"),
					SharedEnv: packit.Environment{
						"PYTHONPATH.delim":   ":",
						"PYTHONPATH.prepend": filepath.Join(layersDir, "pipenv", "lib/python3.8/site-packages"),
					},
					BuildEnv:         packit.Environment{},
					LaunchEnv:        packit.Environment{},
					Build:            false,
					Launch:           false,
					Cache:            false,
					ProcessLaunchEnv: map[string]packit.Environment{},
					Metadata: map[string]interface{}{
						pipenv.DependencySHAKey: "pipenv-dependency-sha",
						"built_at":              timeStamp.Format(time.RFC3339Nano),
					},
				},
			},
		}))

		Expect(entryResolver.ResolveCall.Receives.String).To(Equal("pipenv"))
		Expect(entryResolver.ResolveCall.Receives.BuildpackPlanEntrySlice).To(Equal([]packit.BuildpackPlanEntry{
			{
				Name: "pipenv",
			},
		}))
		Expect(entryResolver.ResolveCall.Receives.InterfaceSlice).To(Equal([]interface{}{"BP_PIPENV_VERSION"}))

		Expect(dependencyManager.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
		Expect(dependencyManager.ResolveCall.Receives.Id).To(Equal("pipenv"))
		Expect(dependencyManager.ResolveCall.Receives.Version).To(Equal(""))
		Expect(dependencyManager.ResolveCall.Receives.Stack).To(Equal("some-stack"))

		Expect(dependencyManager.GenerateBillOfMaterialsCall.Receives.Dependencies).To(Equal([]postal.Dependency{
			{
				ID:      "pipenv",
				Name:    "pipenv-dependency-name",
				SHA256:  "pipenv-dependency-sha",
				Stacks:  []string{"some-stack"},
				URI:     "pipenv-dependency-uri",
				Version: "pipenv-dependency-version",
			},
		}))

		Expect(entryResolver.MergeLayerTypesCall.Receives.String).To(Equal("pipenv"))
		Expect(entryResolver.MergeLayerTypesCall.Receives.BuildpackPlanEntrySlice).To(Equal([]packit.BuildpackPlanEntry{
			{
				Name: "pipenv",
			},
		}))

		Expect(dependencyManager.DeliverCall.Receives.Dependency).To(Equal(postal.Dependency{
			ID:      "pipenv",
			Name:    "pipenv-dependency-name",
			SHA256:  "pipenv-dependency-sha",
			Stacks:  []string{"some-stack"},
			URI:     "pipenv-dependency-uri",
			Version: "pipenv-dependency-version",
		}))
		Expect(dependencyManager.DeliverCall.Receives.CnbPath).To(Equal(cnbDir))
		Expect(dependencyManager.DeliverCall.Receives.DestPath).To(ContainSubstring("pipenv-release"))
		Expect(dependencyManager.DeliverCall.Receives.PlatformPath).To(Equal("some-platform-path"))

		Expect(installProcess.ExecuteCall.Receives.SrcPath).To(ContainSubstring("pipenv-release"))
		Expect(installProcess.ExecuteCall.Receives.DestLayerPath).To(Equal(filepath.Join(layersDir, "pipenv")))
	})

	context("when build plan entries require pipenv at build/launch", func() {
		it.Before(func() {
			entryResolver.MergeLayerTypesCall.Returns.Build = true
			entryResolver.MergeLayerTypesCall.Returns.Launch = true
		})

		it("makes the layer available at the right times", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:    "Some Buildpack",
					Version: "some-version",
				},
				CNBPath: cnbDir,
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "pipenv",
							Metadata: map[string]interface{}{
								"build": true,
							},
						},
						{
							Name: "pipenv",
							Metadata: map[string]interface{}{
								"launch": true,
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
				Stack:  "some-stack",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(Equal([]packit.Layer{
				{
					Name: "pipenv",
					Path: filepath.Join(layersDir, "pipenv"),
					SharedEnv: packit.Environment{
						"PYTHONPATH.delim":   ":",
						"PYTHONPATH.prepend": filepath.Join(layersDir, "pipenv", "lib/python3.8/site-packages"),
					},
					BuildEnv:         packit.Environment{},
					LaunchEnv:        packit.Environment{},
					Build:            true,
					Launch:           true,
					Cache:            true,
					ProcessLaunchEnv: map[string]packit.Environment{},
					Metadata: map[string]interface{}{
						pipenv.DependencySHAKey: "pipenv-dependency-sha",
						"built_at":              timeStamp.Format(time.RFC3339Nano),
					},
				},
			}))
		})
	})

	context("when rebuilding a layer", func() {
		it.Before(func() {
			err := ioutil.WriteFile(filepath.Join(layersDir, fmt.Sprintf("%s.toml", pipenv.Pipenv)), []byte(fmt.Sprintf(`[metadata]
			%s = "pipenv-dependency-sha"
			built_at = "some-build-time"
			`, pipenv.DependencySHAKey)), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = os.MkdirAll(filepath.Join(layersDir, "pipenv", "env"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(layersDir, "pipenv", "env", "PYTHONPATH.prepend"), []byte(fmt.Sprintf("%s/pipenv/lib/python3.8/site-packages", layersDir)), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(layersDir, "pipenv", "env", "PYTHONPATH.delim"), []byte(":"), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		it("skips the build process if the cached dependency sha matches the selected dependency sha", func() {
			result, err := build(packit.BuildContext{
				BuildpackInfo: packit.BuildpackInfo{
					Name:    "Some Buildpack",
					Version: "some-version",
				},
				CNBPath: cnbDir,
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name: "pipenv",
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
				Stack:  "some-stack",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(buffer.String()).ToNot(ContainSubstring("Executing build process"))
			Expect(buffer.String()).To(ContainSubstring("Reusing cached layer"))

			Expect(result.Layers).To(Equal([]packit.Layer{
				{
					Name: "pipenv",
					Path: filepath.Join(layersDir, "pipenv"),
					SharedEnv: packit.Environment{
						"PYTHONPATH.delim":   ":",
						"PYTHONPATH.prepend": filepath.Join(layersDir, "pipenv", "lib/python3.8/site-packages"),
					},
					BuildEnv:         packit.Environment{},
					LaunchEnv:        packit.Environment{},
					Build:            false,
					Launch:           false,
					Cache:            false,
					ProcessLaunchEnv: map[string]packit.Environment{},
					Metadata: map[string]interface{}{
						pipenv.DependencySHAKey: "pipenv-dependency-sha",
						"built_at":              "some-build-time",
					},
				},
			}))

			Expect(dependencyManager.DeliverCall.CallCount).To(Equal(0))
			Expect(installProcess.ExecuteCall.CallCount).To(Equal(0))
		})
	})

	context("failure cases", func() {
		context("when dependency resolution fails", func() {
			it.Before(func() {
				dependencyManager.ResolveCall.Returns.Error = errors.New("failed to resolve dependency")
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: packit.BuildpackInfo{
						Name:    "Some Buildpack",
						Version: "some-version",
					},
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "pip",
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})

				Expect(err).To(MatchError(ContainSubstring("failed to resolve dependency")))
			})
		})

		context("when pipenv layer cannot be fetched", func() {
			it.Before(func() {
				Expect(os.Chmod(layersDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: packit.BuildpackInfo{
						Name:    "Some Buildpack",
						Version: "some-version",
					},
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "pipenv",
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})

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
				_, err := build(packit.BuildContext{
					BuildpackInfo: packit.BuildpackInfo{
						Name:    "Some Buildpack",
						Version: "some-version",
					},
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "pipenv",
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})

				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when dependency cannot be delivered", func() {
			it.Before(func() {
				dependencyManager.DeliverCall.Returns.Error = errors.New("failed to deliver dependency")
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: packit.BuildpackInfo{
						Name:    "Some Buildpack",
						Version: "some-version",
					},
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "pipenv",
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})

				Expect(err).To(MatchError(ContainSubstring("failed to deliver dependency")))
			})
		})

		context("when dependency cannot be installed", func() {
			it.Before(func() {
				installProcess.ExecuteCall.Returns.Error = errors.New("failed to install dependency")
			})
			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: packit.BuildpackInfo{
						Name:    "Some Buildpack",
						Version: "some-version",
					},
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "pipenv",
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})

				Expect(err).To(MatchError(ContainSubstring("failed to install dependency")))
			})
		})

		context("when the site packages cannot be found", func() {
			it.Before(func() {
				siteProcess.ExecuteCall.Returns.Error = errors.New("failed to find site-packages dir")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: packit.BuildpackInfo{
						Name:    "Some Buildpack",
						Version: "some-version",
					},
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "pipenv",
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to find site-packages dir")))
			})
		})

		context("when the layer does not have a site-packages directory", func() {
			it.Before(func() {
				siteProcess.ExecuteCall.Returns.String = ""
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					BuildpackInfo: packit.BuildpackInfo{
						Name:    "Some Buildpack",
						Version: "some-version",
					},
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name: "pipenv",
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
					Stack:  "some-stack",
				})
				Expect(err).To(MatchError(ContainSubstring("pipenv installation failed: site packages are missing from the pipenv layer")))
			})
		})
	})
}
