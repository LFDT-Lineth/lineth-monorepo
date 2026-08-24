import { getAddress } from "ethers";
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";

import { DeploymentStep } from "./address-plan";
import { DeploymentCheckpoint } from "./checkpoint";
import { sanitizeText } from "./config";
import { CheckpointStore } from "./store";
import { ParsedDeploymentRecord } from "../../common/helpers/deploymentRecord";

interface RunStepScriptInput {
  scriptPath: string;
  environment: NodeJS.ProcessEnv;
  step: DeploymentStep;
  checkpoint: DeploymentCheckpoint;
  store: CheckpointStore;
  sensitiveValues: readonly string[];
}

interface DeploymentRecordMessage {
  type: "lineth-deployment-record";
  id: string;
  record: ParsedDeploymentRecord;
}

interface DeploymentIntentMessage {
  type: "lineth-deployment-intent";
  id: string;
  contractName: string;
}

const INHERITED_CHILD_ENVIRONMENT = [
  "HOME",
  "HTTP_PROXY",
  "HTTPS_PROXY",
  "NODE_EXTRA_CA_CERTS",
  "NO_PROXY",
  "PATH",
  "SSL_CERT_FILE",
  "TMPDIR",
  "TZ",
] as const;

export function childEnvironment(
  environment: NodeJS.ProcessEnv,
  parentEnvironment: NodeJS.ProcessEnv = process.env,
): NodeJS.ProcessEnv {
  const inherited = Object.fromEntries(
    INHERITED_CHILD_ENVIRONMENT.flatMap((name) => {
      const value = parentEnvironment[name];
      return value === undefined ? [] : [[name, value]];
    }),
  );
  return { ...inherited, ...environment };
}

function isDeploymentRecordMessage(message: unknown): message is DeploymentRecordMessage {
  if (typeof message !== "object" || message === null) return false;
  const candidate = message as Partial<DeploymentRecordMessage>;
  return candidate.type === "lineth-deployment-record" && typeof candidate.id === "string" && !!candidate.record;
}

function isDeploymentIntentMessage(message: unknown): message is DeploymentIntentMessage {
  if (typeof message !== "object" || message === null) return false;
  const candidate = message as Partial<DeploymentIntentMessage>;
  return (
    candidate.type === "lineth-deployment-intent" &&
    typeof candidate.id === "string" &&
    typeof candidate.contractName === "string"
  );
}

function expectedDeployment(input: RunStepScriptInput, contractName: string) {
  const matches = input.step.deployments.filter((deployment) => deployment.contractName === contractName);
  if (matches.length !== 1) {
    throw new Error(`${input.step.id} emitted unexpected or ambiguous deployment ${contractName}`);
  }
  return matches[0]!;
}

export async function runStepScript(input: RunStepScriptInput): Promise<void> {
  const child = spawn(process.execPath, [input.scriptPath], {
    env: childEnvironment(input.environment),
    stdio: ["ignore", "pipe", "pipe", "ipc"],
  });
  const { stdout, stderr } = child;
  if (!stdout || !stderr) throw new Error(`failed to capture output for ${input.step.id}`);

  const closePromise = new Promise<number>((resolve, reject) => {
    child.once("error", reject);
    child.once("close", (code) => resolve(code ?? 1));
  });
  const stderrPromise = (async () => {
    for await (const line of createInterface({ input: stderr })) {
      console.error(sanitizeText(line, input.sensitiveValues));
    }
  })();

  const recordedKeys = new Set<string>();
  let messageProcessing = Promise.resolve();
  let messageProcessingError: unknown;
  child.on("message", (message: unknown) => {
    if (!isDeploymentIntentMessage(message) && !isDeploymentRecordMessage(message)) return;
    messageProcessing = messageProcessing
      .then(async () => {
        try {
          if (isDeploymentIntentMessage(message)) {
            const { id, contractName } = message;
            const expected = expectedDeployment(input, contractName);
            if (input.checkpoint.deployments[expected.key] || input.checkpoint.inFlightDeployments[expected.key]) {
              throw new Error(`${input.step.id} requested ${contractName} twice`);
            }
            input.checkpoint.inFlightDeployments[expected.key] = {
              stepId: input.step.id,
              chain: input.step.chain,
              contractName: expected.contractName,
              nonce: expected.nonce,
              expectedAddress: getAddress(expected.expectedAddress),
            };
            await input.store.save(input.checkpoint);
            child.send({ type: "lineth-deployment-intent-ack", id });
            return;
          }

          const { id, record } = message;
          const expected = expectedDeployment(input, record.contractName);
          if (recordedKeys.has(expected.key) || input.checkpoint.deployments[expected.key]) {
            throw new Error(`${input.step.id} emitted ${record.contractName} twice`);
          }
          if (!input.checkpoint.inFlightDeployments[expected.key]) {
            throw new Error(`${input.step.id} emitted ${record.contractName} without a durable deployment intent`);
          }
          if (getAddress(record.address) !== getAddress(expected.expectedAddress)) {
            throw new Error(
              `${input.step.id} deployed ${record.contractName} at ${record.address}; expected ${expected.expectedAddress}`,
            );
          }
          const expectedChainId = input.checkpoint.chainIds[input.step.chain];
          if (record.chainId !== expectedChainId) {
            throw new Error(
              `${input.step.id} emitted chain ID ${record.chainId} for ${record.contractName}; expected ${expectedChainId}`,
            );
          }

          input.checkpoint.deployments[expected.key] = {
            address: getAddress(record.address),
            transactionHash: record.transactionHash,
            blockNumber: record.blockNumber,
            chainId: record.chainId,
            recovered: false,
          };
          delete input.checkpoint.inFlightDeployments[expected.key];
          await input.store.save(input.checkpoint);
          recordedKeys.add(expected.key);
          child.send({ type: "lineth-deployment-record-ack", id });
        } catch (error) {
          child.send({
            type: isDeploymentIntentMessage(message) ? "lineth-deployment-intent-ack" : "lineth-deployment-record-ack",
            id: message.id,
            error: "deployment checkpoint persistence failed",
          });
          throw error;
        }
      })
      .catch((error: unknown) => {
        messageProcessingError ??= error;
        child.kill("SIGTERM");
      });
  });

  try {
    for await (const line of createInterface({ input: stdout })) {
      console.log(sanitizeText(line, input.sensitiveValues));
    }
  } catch (error) {
    child.kill("SIGTERM");
    await Promise.allSettled([closePromise, stderrPromise]);
    throw error;
  }

  const [exitCode] = await Promise.all([closePromise, stderrPromise]);
  await messageProcessing;
  if (messageProcessingError) throw messageProcessingError;
  if (exitCode !== 0) throw new Error(`${input.step.id} deploy script exited with code ${exitCode}`);
  if (recordedKeys.size !== input.step.deployments.length) {
    throw new Error(
      `${input.step.id} emitted ${recordedKeys.size} deployment records; expected ${input.step.deployments.length}`,
    );
  }
}
