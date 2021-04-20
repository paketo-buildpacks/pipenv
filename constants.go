package pipenv

const (
	Pipenv           = "pipenv"
	PipFileLock      = "Pipfile.lock"
	DependencySHAKey = "dependency_sha"
	CPython          = "cpython"
	Pip              = "pip"
)

var Priorities = []interface{}{"BP_PIPENV_VERSION"}
