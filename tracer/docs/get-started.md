## Get started

### Development setup

#### Step 1: Install Java 21

```
brew install openjdk@21
```

#### Step 2: Install the Go toolchain

#### Step 3: Install Rust

```
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Use local git executable to fetch from repos (needed for private repos)
echo "net.git-fetch-with-cli=true" >> .cargo/config.toml
```

#### Step 4: Install Corset

```shell
cargo install --git ssh://git@github.com/ConsenSys/corset --locked --force
```

#### Step 5: Update constraints [submodule](https://github.com/Consensys/linea-constraints/)

```shell
git submodule update --init --recursive
```

Note: Windows user may have to run 'git config core.protectNTFS false' command within the linea-constraints folder to bypass CON.* file names being reserved.

#### Step 6: Install [pre-commit](https://pre-commit.com/)

```shell
pip install --user pre-commit

# For macOS users.
brew install pre-commit
```

Then run `pre-commit install` to set up git hook scripts.
Used hooks can be found [here](.pre-commit-config.yaml).

______________________________________________________________________

NOTE

> `pre-commit` aids in running checks (end of file fixing,
> markdown linting, linting, runs tests, json validation, etc.)
> before you perform your git commits.

______________________________________________________________________

### Run tests

```shell
# Run all tests
./gradlew clean test

# Run only unit tests
./gradlew clean unitTests

# Run only acceptance tests
./gradlew clean acceptanceTests

# Generate EVM test suite BlockchainTests
./gradlew :reference-tests:generateBlockchainReferenceTests 

# Run EVM test suite BlockchainTests
./gradlew clean referenceBlockchainTests

# Generate EVM test suite GeneralStateTests
./gradlew :reference-tests:generateGeneralStateReferenceTests 

# Run EVM test suite GeneralStateTests
./gradlew clean referenceGeneralStateTests

# Run all EVM test suite reference tests
./gradlew clean referenceTests

# Run single reference test via gradle, e.g for net.consensys.linea.generated.blockchain.BlockchainReferenceTest_583
./gradlew :reference-tests:referenceTests --tests "net.consensys.linea.generated.blockchain.BlockchainReferenceTest_583"
```

______________________________________________________________________

NOTE

> Please be aware if the reference test code generation tasks `blockchainReferenceTests` and
> `generalStateReferenceTests` do not generate any Java code, then probably you are missing the Ethereum tests
> submodule which you can clone via `git submodule update --init --recursive`.

______________________________________________________________________

### Capture a replay

For debugging and inspection purposes, it is possible to capture a _replay_, _i.e._ all the minimal information required to replay a series of blocks as they played on the blockchain, which is done with `scripts/capture.pl`.

A typical invocation would be:

```
scripts/capture.pl --start 1300923
```

which would capture a replay of block #1300923 and store it in `arithmetization/src/test/resources/replays`. More options are available, refer to `scripts/capture.pl -h`.

## Set up IntelliJ IDEA 

### Enable annotation processing

- Go to `Settings | Build, Execution, Deployment | Compiler | Annotation Processors` and tick the following
  checkbox:

  ![idea_enable_annotation_processing_setting.png](images/idea_enable_annotation_processing_setting.png)

______________________________________________________________________

NOTE

> This setting is required to avoid IDE compilation errors because of the [Lombok](https://projectlombok.org/features/)
> library used for code generation of boilerplate Java code such as:
>
> - Getters/Setters (via [`@Getter/@Setter`](https://projectlombok.org/features/GetterSetter))
> - Class log instances (via [`@Slf4j`](https://projectlombok.org/features/log))
> - Builder classes (via [`@Builder`](https://projectlombok.org/features/Builder))
> - Constructors (
>   via [`@NoArgsConstructor/@RequiredArgsConstructor/@AllArgsConstructor`](https://projectlombok.org/features/constructor))
> - etc.
>
> Learn more about how Java annotation processing
> works [here](https://www.baeldung.com/java-annotation-processing-builder).

______________________________________________________________________

### Set up IDE code re-formatting

- Install [Checkstyle](https://plugins.jetbrains.com/plugin/1065-checkstyle-idea) plugin and set IDE code
  reformatting to comply with the project's Checkstyle configuration:

  - Go to `Settings | Editor | Code Style | Java | <hamburger menu> | Import Scheme | Checkstyle configuration`:

    ![idea_checkstyle_reformat.png](images/idea_checkstyle_reformat.png)

    and select `<project_root>/config/checkstyle.xml`.

### Install optional plugins

- Install [Spotless Gradle](https://plugins.jetbrains.com/plugin/18321-spotless-gradle) plugin to re-format through
  the IDE according to spotless configuration.

## Debug traces

- JSON files can be debugged with the following command:

```shell
corset check -T <JSON_FILE> -v linea-constraints/zkevm.bin
```

## Disable Corset expansion

Corset expansion means that generated traces are checked as accurately
as possible. However, this slows testing down to some extent. It can
be easily disabled in IntelliJ:
   
   - Go to `Run | Edit Configurations`
   
   ![idea_disable_corset_expansion.png](images/idea_disable_corset_expansion.png)

   and add `CORSET_FLAGS=` under `Environment Variables`.  This turns
   off all expansion modes, including field arithmetic.
