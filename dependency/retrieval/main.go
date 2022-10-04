package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/joshuatcasey/libdependency/retrieve"
	"github.com/joshuatcasey/libdependency/versionology"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/packit/v2/vacation"
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
	response, err := http.DefaultClient.Get("https://pypi.org/pypi/pipenv/json")
	if err != nil {
		return nil, fmt.Errorf("could not get project metadata: %w", err)
	}

	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}

	var pipenvMetadata PyPiProductMetadataRaw
	err = json.Unmarshal(body, &pipenvMetadata)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal project metadata: %w", err)
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
		Licenses:       retrieve.LookupLicenses(pipenvRelease.SourceURL, defaultDecompress),
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

func defaultDecompress(artifact io.Reader, destination string) error {
	archive := vacation.NewArchive(artifact)

	err := archive.StripComponents(1).Decompress(destination)
	if err != nil {
		return fmt.Errorf("failed to decompress source file: %w", err)
	}

	return nil
}
