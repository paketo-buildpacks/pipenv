package pipenv

import (
	"bytes"
	"fmt"
	"os"

	"github.com/paketo-buildpacks/packit/pexec"
)

//go:generate faux --interface Executable --output fakes/executable.go

// Executable defines the interface for invoking an executable.
type Executable interface {
	Execute(pexec.Execution) error
}

type PipenvInstallProcess struct {
	executable Executable
}

// NewPipenvInstallProcess creates a PipenvInstallProcess instance.
func NewPipenvInstallProcess(executable Executable) PipenvInstallProcess {
	return PipenvInstallProcess{
		executable: executable,
	}
}

// Execute installs the pipenv binary from source code located in the given
// srcPath into the layer path designated by targetLayerPath.
func (p PipenvInstallProcess) Execute(srcPath, targetLayerPath string) error {
	buffer := bytes.NewBuffer(nil)

	err := p.executable.Execute(pexec.Execution{
		// Install pipenv from source with the pip that comes from a previous buildpack
		Args: []string{"install", "pipenv", "--user", fmt.Sprintf("--find-links=%s", srcPath)},
		// Set the PYTHONUSERBASE to ensure that pip is installed to the newly created target layer.
		Env:    append(os.Environ(), fmt.Sprintf("PYTHONUSERBASE=%s", targetLayerPath)),
		Stdout: buffer,
		Stderr: buffer,
	})

	if err != nil {
		return fmt.Errorf("failed to configure pipenv:\n%s\nerror: %w", buffer.String(), err)
	}

	return nil
}
