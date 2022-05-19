# Pipenv Cloud Native Buildpack
The Paketo Pipenv Buildpack is a Cloud Native Buildpack that installs
[pipenv](https://pypi.org/project/pipenv) into a layer and makes it available
on the `PATH`.

The buildpack is published for consumption at `gcr.io/paketo-buildpacks/pipenv`
and `paketocommunity/pipenv`.

## Behavior
This buildpack always participates.

The buildpack will do the following:
* At build time:
  - Contributes the `pipenv` binary to a layer
  - Prepends the `pipenv` layer to the `PYTHONPATH`
  - Adds the newly installed pipenv location to `PATH`
* At run time:
  - Does nothing

## Configuration
| Environment Variable | Description
| -------------------- | -----------
| `$BP_PIPENV_VERSION` | Configure the version of pipenv to install. Buildpack releases (and the pipenv versions for each release) can be found [here](https://github.com/paketo-buildpacks/pipenv/releases).

## Integration

The Pipenv CNB provides pipenv as a dependency. Downstream buildpacks, can
require the pipenv dependency by generating a [Build Plan
TOML](https://github.com/buildpacks/spec/blob/master/buildpack.md#build-plan-toml)
file that looks like the following:

```toml
[[requires]]

  # The name of the Pipenv dependency is "pipenv". This value is considered
  # part of the public API for the buildpack and will not change without a plan
  # for deprecation.
  name = "pipenv"

  # The version of the Pipenv dependency is not required. In the case it
  # is not specified, the buildpack will provide the default version, which can
  # be seen in the buildpack.toml file.
  # If you wish to request a specific version, the buildpack supports
  # specifying a semver constraint in the form of "2018.*", "2018.11.*", or even
  # "2018.11.26".
  version = "2018.11.26"

  # The Pipenv buildpack supports some non-required metadata options.
  [requires.metadata]

    # Setting the build flag to true will ensure that the Pipenv
    # depdendency is available on the $PATH for subsequent buildpacks during
    # their build phase. If you are writing a buildpack that needs to run Pipenv
    # during its build process, this flag should be set to true.
    build = true

    # Setting the launch flag to true will ensure that the Pipenv
    # dependency is available on the $PATH for the running application. If you are
    # writing an application that needs to run Pipenv at runtime, this flag should
    # be set to true.
    launch = true
```

## Limitations

This buildpack requires internet connectivity to install `pipenv`. Installation
in an air-gapped environment is not supported.

## Usage

To package this buildpack for consumption:
```
$ ./scripts/package.sh --version x.x.x
```
This will create a `buildpackage.cnb` file under the build directory which you
can use to build your app as follows: `pack build <app-name> -p <path-to-app> -b <cpython-buildpack> -b <pip-buildpack> -b build/buildpackage.cnb -b <some-pipenv-consumer-buildpack>`.

To run the unit and integration tests for this buildpack:
```
$ ./scripts/unit.sh && ./scripts/integration.sh
```
