import type { TaskArguments } from "hardhat/types";

function getCliOrEnvValue(cliParam: string, envParam: string): string | undefined {
  const argv = process.argv;

  const cliParamIndex = argv.indexOf(cliParam);

  if (cliParamIndex != -1) {
    return argv[cliParamIndex + 1];
  } else {
    //look in the .env
    const envVariable = process.env[envParam];
    if (envVariable) {
      return envVariable;
    }
  }
  return undefined;
}

function getTaskCliOrEnvValue(taskArgs: TaskArguments, cliParam: string, envParam: string): string | undefined {
  if (taskArgs[cliParam] !== undefined) {
    return taskArgs[cliParam];
  } else {
    //look in the .env
    const envVariable = process.env[envParam];
    if (envVariable) {
      return envVariable;
    }
  }
  return undefined;
}

export { getCliOrEnvValue, getTaskCliOrEnvValue };
