import { exec } from "child_process";

import { createTestLogger } from "../../config/logger";

const logger = createTestLogger();

export async function execDockerCommand(command: string, containerName: string): Promise<string> {
  const dockerCommand = `docker ${command} ${containerName}`;
  logger.debug(`Executing ${dockerCommand}...`);
  return new Promise((resolve, reject) => {
    exec(dockerCommand, (error, stdout, stderr) => {
      if (error) {
        logger.error(`Error executing (${dockerCommand}). error=${stderr}`);
        reject(error);
      }
      logger.debug(`Execution success (${dockerCommand}). output=${stdout}`);
      resolve(stdout);
    });
  });
}

export async function getDockerImageTag(containerName: string, imageRepoName: string): Promise<string> {
  const inspectJsonOutput = JSON.parse(await execDockerCommand("inspect", containerName));
  return inspectJsonOutput[0]["Config"]["Image"].replace(`${imageRepoName}:`, "");
}
