# Restructure Pipenv Buildpack 

## Proposal

The existing Pipenv buildpack should be rewritten and restructured to *only*
provide the `pipenv` dependency. Any app dependency installation logic should
be removed.

## Motivation

In keeping with the overarching [Python Buildpack Rearchitecture
RFC](https://github.com/paketo-community/python/blob/main/rfcs/0001-restructure.md),
the Pipenv Buildpack should perform one task, which is installing the `pipenv`
dependency. This is part of the effort in Paketo Buildpacks to reduce the
responsibilities of each buildpack to make them easier to understand and
maintain.

## Implementation

### API

provides: `pipenv` requires: `cpython` and `pip` during build

### Detection

The buildpack will pass detection if a Pipfile exists in the app directory. If
a Pipfile.lock is present, check for a Python version and add it to the
requirements.

### Configuration

#### BP_PIPENV_VERSION

Users may specify a Pipenv version through the `BP_PIPENV_VERSION` environment
variable. This can be set explicitly at build time (e.g. `pack --env`) or
through `project.toml`.

### Dependency Installation

During the build phase, pipenv installation proceeds as outlined below:

Using the pip installation from the preceding `pip` buildpack, the buildpack
downloads the selected Pipenv dependency and extracts it into a temporary
directory (`path/to/pipenv/dependency`).

`PYTHONUSERBASE=<path/to/pipenv/layer/> pip install <path/to/pipenv/dependency>
--user` is run to install the requested version. Setting the `PYTHONUSERBASE`
variable ensures that pipenv is installed to the newly created layer.

Once the installation is complete, the buildpack prepends the
`layers/<pipenv-layer>/lib/python/site-packages` to the `PYTHONPATH`
environment variable and export it as a shared environment variable for
downstream buildpacks. This is necessary so Python looks for `pipenv` in the
pipenv layer, instead of the default location.

