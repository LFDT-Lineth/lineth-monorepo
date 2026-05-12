# Tools to test Linea stack
## Getting Started

ManualLoadTest main method provides a way to generate traffic based on file describing scenarios.

options for the parameters can be retrieved using --help
There are two of them:
 _ pk: which must be set with the private key of the wallet containing the funds that will be used
to pay the generated txs.
 _ request: the path to the file describing the traffic to generate.

## Run a load test against Devnet
* Run `./gradlew :testing-tools:traffic-generator:run --args="-request devnet/%JSON_FILE% -pk %PRIVATE_KEY%"`
  * Note that you have to specify the correct private key for the account id you put into a json
  * You have to put the right file name instead of `%JSON_FILE%` too
