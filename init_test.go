package pipenv_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitPipenv(t *testing.T) {
	suite := spec.New("pipenv", spec.Report(report.Terminal{}))
	suite("Detect", testDetect)
	suite("Build", testBuild)
	suite("InstallProcess", testPipenvInstallProcess)
	suite("SiteProcess", testSiteProcess)
	suite.Run(t)
}
