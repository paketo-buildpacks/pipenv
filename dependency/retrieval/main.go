package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/libdependency/retrieve"
	"github.com/paketo-buildpacks/libdependency/upstream"
	"github.com/paketo-buildpacks/libdependency/versionology"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type PyPiProductMetadataRaw struct {
	Releases map[string][]struct {
		PackageType string            `json:"packagetype"`
		URL         string            `json:"url"`
		UploadTime  string            `json:"upload_time_iso_8601"`
		Digests     map[string]string `json:"digests"`
	} `json:"releases"`
}

type PipenvRelease struct {
	version      *semver.Version
	SourceURL    string
	UploadTime   time.Time
	SourceSHA256 string
}

func (release PipenvRelease) Version() *semver.Version {
	return release.version
}

func getAllVersions() (versionology.VersionFetcherArray, error) {
	var pipenvMetadata PyPiProductMetadataRaw
	err := upstream.GetAndUnmarshal("https://pypi.org/pypi/pipenv/json", &pipenvMetadata)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve new versions from upstream: %w", err)
	}

	var allVersions versionology.VersionFetcherArray

	for version, releasesForVersion := range pipenvMetadata.Releases {
		for _, release := range releasesForVersion {
			if release.PackageType != "sdist" {
				continue
			}

			fmt.Printf("Parsing semver version %s\n", version)

			newVersion, err := semver.NewVersion(version)
			if err != nil {
				continue
			}

			uploadTime, err := time.Parse(time.RFC3339, release.UploadTime)
			if err != nil {
				return nil, fmt.Errorf("could not parse upload time '%s' as date for version %s: %w", release.UploadTime, version, err)
			}

			allVersions = append(allVersions, PipenvRelease{
				version:      newVersion,
				SourceSHA256: release.Digests["sha256"],
				SourceURL:    release.URL,
				UploadTime:   uploadTime,
			})
		}
	}

	return allVersions, nil
}

func generateMetadata(versionFetcher versionology.VersionFetcher) ([]versionology.Dependency, error) {
	version := versionFetcher.Version().String()
	pipenvRelease, ok := versionFetcher.(PipenvRelease)
	if !ok {
		return nil, errors.New("expected a PipenvRelease")
	}

	configMetadataDependency := cargo.ConfigMetadataDependency{
		CPE:            fmt.Sprintf("cpe:2.3:a:python-pipenv:pipenv:%s:*:*:*:*:python:*:*", version),
		Checksum:       fmt.Sprintf("sha256:%s", pipenvRelease.SourceSHA256),
		ID:             "pipenv",
		Licenses:       retrieve.LookupLicenses(pipenvRelease.SourceURL, upstream.DefaultDecompress),
		Name:           "Pipenv",
		PURL:           retrieve.GeneratePURL("pipenv", version, pipenvRelease.SourceSHA256, pipenvRelease.SourceURL),
		Source:         pipenvRelease.SourceURL,
		SourceChecksum: fmt.Sprintf("sha256:%s", pipenvRelease.SourceSHA256),
		Stacks:         []string{"*"},
		URI:            pipenvRelease.SourceURL,
		Version:        version,
	}

	return []versionology.Dependency{{
		ConfigMetadataDependency: configMetadataDependency,
		SemverVersion:            versionFetcher.Version(),
	}}, nil
}

func main() {
	retrieve.NewMetadata("pipenv", getAllVersions, generateMetadata)
}
