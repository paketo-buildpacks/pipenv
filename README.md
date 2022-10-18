# Pipenv Cloud Native Buildpack
The Paketo Buildpack for Pipenv is a Cloud Native Buildpack that installs
[pipenv](https://pypi.org/project/pipenv) into a layer and makes it available
on the `PATH`.

The buildpack is published for consumption at `gcr.io/paketo-buildpacks/pipenv`.

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
| Environment Variable | Description                                                                                                                                                                                    |
|----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `$BP_PIPENV_VERSION` | Configure the version of pipenv to install. Buildpack releases (and the supported pipenv versions for each release) can be found [here](https://github.com/paketo-buildpacks/pipenv/releases). |

## Integration

The Pipenv CNB provides pipenv as a dependency. Downstream buildpacks can
require the pipenv dependency by generating a [Build Plan
TOML](https://github.com/buildpacks/spec/blob/master/buildpack.md#build-plan-toml)
file that looks like the following:

```toml
[[requires]]

  # The name of the Pipenv dependency is "pipenv". This value is considered
  # part of the public API for the buildpack and will not change without a plan
  # for deprecation.
  name = "pipenv"

  # The Pipenv buildpack supports some non-required metadata options.
  [requires.metadata]
    
    # Use `version` to request a specific version of `pipenv`.
    # This buildpack supports specifying a semver constraint in the form of "2018.*", "2018.11.*",
    # or even "2018.11.26".
    # Optional, defaults to the latest version of `pipenv` found in the `buildpack.toml` file.
    version = "2018.11.26"

    # When `build` is true, this buildpack will ensure that `pipenv` is available
    # on the `$PATH` for later buildpacks.
    # Optional, default false.
    build = true

    # When `launch` is true, this buildpack will ensure that `pipenv` is available
    # on the `$PATH` for the running application.
    # Optional, default false.
    launch = true
```

## Limitations

This buildpack requires internet connectivity to install `pipenv`.
Installation in an air-gapped environment is not supported.

The dependency contained in `buildpack.toml` is not actually used, as `pipenv` is installed directly from the internet.
However, the rest of the dependency metadata is used (e.g. for generating an SBOM).
This will be addressed in upcoming work which will change the way dependencies are consumed by buildpacks.

## Usage

To package this buildpack for consumption:
```shell
$ ./scripts/package.sh --version x.x.x
```

This will create a `build/buildpackage.cnb` file which you can use to build your app as follows:

```shell
pack build <app-name> \
  --path <path-to-app> \
  --buildpack <cpython-buildpack> \
  --buildpack <pip-buildpack> \
  --buildpack build/buildpackage.cnb \
  --buildpack <some-pipenv-consumer-buildpack>
```

To run the unit and integration tests for this buildpack:
```shell
$ ./scripts/unit.sh && ./scripts/integration.sh
```
