# End to end tests
## Setup
Run `pnpm install` to setup typechain

Run `make fresh-start-all` from root directory to spin up local environment


## Run tests
| ENV | Command | Description |
|----|---|---|
| Local | `pnpm run test:e2e:local` | Uses already running docker environment and deployed smart contracts |
| DEV | `pnpm run test:e2e:dev` | Uses DEV env, may need to update constants in `constants.dev.ts`  |
| UAT | `pnpm run test:e2e:uat` | Uses UAT env, may need to update constants in `constants.uat.ts` |

## Remote workflows
Workflow options:
- `e2e-tests-with-ssh` - Enable to run `Setup upterm session` step, manually ssh into the github actions workflow using
the steps output, can be used to debug containers.
  - The step will output a string used to connect to the workflow.
  - Example: `ssh XTpun7OCRZMgaCZkiHqU:MWNlNmQ0OGEudm0udXB0ZXJtLmludGVybmFsOjIyMjI=@uptermd.upterm.dev`
  - After connecting create a new file called `continue` in the root directory: `touch continue`
- `e2e-tests-logs-dump` - Enable to print logs after e2e tests have ran


## Debugging test in vscode 
Install the `vscode-jest` plugin and open `linea-monorepo/e2e/` directory. Use the following config in `linea-monorepo/e2e/.vscode/settings.json` 
```
{
  "jest.autoRun": { "watch": false },
  "jest.jestCommandLine": "pnpm run test:e2e:vscode --",
}
```
and the following config in `linea-monorepo/e2e/.vscode/launch.json` 
```
{
    "configurations": [
        {
            "type": "node",
            "name": "vscode-jest-tests.v2",
            "request": "launch",
            "program": "${workspaceFolder}/node_modules/.bin/jest",
            "args": [
                "--config",
                "./jest.vscode.config.js",
                "--detectOpenHandles",
                "--runInBand",
                "--watchAll=false",
                "--testNamePattern",
                "${jest.testNamePattern}",
                "--runTestsByPath",
                "${jest.testFile}"
            ],
            "cwd": "${workspaceFolder}",
            "console": "integratedTerminal",
            "internalConsoleOptions": "neverOpen",
            "disableOptimisticBPs": true,
            "windows": {
                "program": "${workspaceFolder}/node_modules/jest/bin/jest"
            }
        }
    
    ]
}
```
Now you should be able to run and debug individual tests from the `Testing` explorer tab.