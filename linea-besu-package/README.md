# Linea Besu Distribution

This project uses Gradle to manage dependencies, build tasks, and create distributions for linea-besu with
all the necessary plugins to run a node for operators. The build process will also create a Docker image that can be
used to run a node with a specific profile.

## Run with Docker

### Step 1. Download configuration files

You can start with the Docker Compose files located in the [docker](https://github.com/Consensys/linea-monorepo/tree/main/linea-besu-package/docker) directory.

### Step 2. Update the Docker Compose file
In the docker-compose.yaml file, update the --p2p-host command to include your public IP address. For example:
```sh
--p2p-host=103.10.10.10
```

To enable JWT on engine-api, please uncomment the followings and mount the JWT file accordingly:
```sh
--engine-jwt-disabled=false
--engine-jwt-secret=/var/lib/besu/jwt
```

**Network selection and genesis files**
- Profiles (e.g., basic/advanced mainnet or sepolia) already embed the Linea network and do not require a genesis file.
- The `FinalizedTagUpdater` plugin was removed; any older configs referencing it are invalid.

### Step 3. Start the Besu node
```sh
docker compose -f ./linea-besu-package/docker/docker-compose-basic-mainnet.yaml up
```
Alternatively, to run a node with a specific profile, set the `BESU_PROFILE` environment variable to the desired profile name:

```sh
docker run -e BESU_PROFILE=basic-mainnet consensys/linea-besu-package:latest
```
Or use a specific release image tag
```sh
docker run -e BESU_PROFILE=basic-mainnet consensys/linea-besu-package:2.1.0-20250507100634-6dc9db9
```
## Run with a binary distribution

### Step 1. Install Linea Besu from packaged binaries
* Download the [linea-besu-package-&lt;release&gt;.tar.gz](https://github.com/Consensys/linea-monorepo/releases?q=linea-besu-package&expanded=true)
from the assets.
* Unpack the downloaded files and change directory into the `linea-besu/besu`
directory.

Display Besu command line help to confirm installation:
```sh
bin/besu --help
```

### Step 2. Start the Besu client

```sh
bin/besu --profile=advanced-mainnet
```

## Build from source locally

1. Make changes to `linea-besu-package/versions.env` as needed

2. Cd into `linea-besu-package`

3. Run `make clean && make build` (check the Makefile for build options)

4. The docker image (i.e. default as `consensys/linea-besu-package:local`) should be created locally

### Note:

To build with specific platform (e.g. linux/amd64) and image tag (e.g. xxx), do the following:
```
make clean && PLATFORM=linux/amd64 TAG=xxx make build
```

To run the e2e test locally with the locally-built `linea-besu-package` image (e.g. tagged as xxx):

- Pre-requisites:
    - pnpm installed
    - docker version 27.x.x

- The following only needed for first run or e2e failures that require npm package update, on your_repo_root_folder:
    ```
    pnpm i -F contracts -F e2e --frozen-lockfile --prefer-offline
    ```
To run the test locally:
```
TAG=xxx make run-e2e-test
```

## How-To Release

1. Make a branch with changes to `linea-besu-package/versions.env` as needed

2. Go to the [actions tab](https://github.com/Consensys/linea-monorepo/actions) and click on the workflow `linea-besu-package-release` and select the target branch for making a release with besu and plugin versions based on `linea-besu-package/versions.env`

3. If release prefix is not given, `LINEA_TRACER_PLUGIN_VERSION` in the target `versions.env` file will be used, and the resultant release tag would be `linea-besu-package-[releasePrefix]-[YYYYMMDDHHMMSS]-[shortenCommitHash]` and the docker image tag would be `[releasePrefix]-[YYYYMMDDHHMMSS]-[shortenCommitHash]`

4. Once the workflow is done successfully, go to the [releases page](https://github.com/Consensys/linea-monorepo/releases?q=linea-besu-package&expanded=true) and you should find the corresponding release info along with the docker image tag

Additionally, the `latest` tag will be updated to match this release


## Profiles

This project leverages [Besu Profiles](https://besu.hyperledger.org/public-networks/how-to/use-configuration-file/profile) to enable multiple startup configurations for different node types.

During the build process, all TOML files located in the [linea-besu-package/linea-besu/profiles](https://github.com/Consensys/linea-monorepo/tree/main/linea-besu-package/linea-besu/profiles) directory will be incorporated into the package. These profiles are crucial for configuring the node, as each one specifies the necessary plugins and CLI options to ensure Besu operates correctly.

Each profile is a TOML file that outlines the plugins and CLI options to be used when starting the node. For example:

```toml
# required plugins to run a sequencer node
plugins=["LineaExtraDataPlugin","LineaEndpointServicePlugin","LineaTransactionPoolValidatorPlugin","LineaTransactionSelectorPlugin"]

# required options to configure the plugins above
plugin-linea-module-limit-file-path="config/trace-limits.mainnet.toml"
# Other required plugin options
# ...

# Other Besu options
# ...
```

Currently, the following profiles are available:

| Profile Name                                                                                                              | Description                                                               | Network |
|---------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------|---------|
| [`basic-mainnet`](https://github.com/Consensys/linea-monorepo/blob/main/linea-besu-package/linea-besu/profiles/basic-mainnet.toml)       | Creates a basic Linea Node.                                               | Mainnet |
| [`advanced-mainnet`](https://github.com/Consensys/linea-monorepo/blob/main/linea-besu-package/linea-besu/profiles/advanced-mainnet.toml) | Creates a Linea Node with extra features such as `linea_estimateGas`. | Mainnet |
| [`basic-sepolia`](https://github.com/Consensys/linea-monorepo/blob/main/linea-besu-package/linea-besu/profiles/basic-sepolia.toml)       | Creates a basic Linea Node.                                               | Sepolia |
| [`advanced-sepolia`](https://github.com/Consensys/linea-monorepo/blob/main/linea-besu-package/linea-besu/profiles/advanced-mainnet.toml) | Creates a Linea Node with extra features such as `linea_estimateGas`. | Sepolia |
