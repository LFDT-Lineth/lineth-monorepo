# State-manager inspector

The state-manager-inspector is a CLI tool that will fetch state-transition traces 
from Shomei and report the errors it encountered during the process. The errors will be appended to a log file shomei.report summarizing all the issues encountered during the process.

## Compiling

```bash
cd prover
make bin/state-manager-inspector
```

## Usage

```bash
bin/state-manager-inspector --help
```

Will print out

```
fetches and audit a sequence of merkle proofs for a range of blocks

Usage:
  state-manager-inspector [flags]

Flags:
      --block-range int         size of the range to fetch from shomei for each request (default 10)
  -h, --help                    help for state-manager-inspector
      --max-rps duration        minimal time to wait between each request (default 20ms)
      --num-threads int         number of threads to use for verification (default 10)
      --start int               starting block of the range (must be the beginning of a conflated batch)
      --stop int                end of the range to fetch
      --url string              host:port of the shomei state-manager to query (default "https://127.0.0.1:443")
```

## Example

```bash
bin/state-manager-inspector --url https://127.0.0.1:443 --start 0 --stop 1000 --max-rps 20ms --num-threads 3 --block-range 10
```
