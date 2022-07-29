package pipenv

const (
	Pipenv          = "pipenv"
	PipenvVersion   = "pipenv-version"
	CPython         = "cpython"
	Pip             = "pip"
	DefaultVersions = "default-versions"
)

var Priorities = []interface{}{
	"BP_PIPENV_VERSION",
	DefaultVersions,
}
