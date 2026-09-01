import { getAddress } from "ethers";
import { ChildProcess, spawn } from "node:child_process";
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

// A completed custom bootstrap item reported by the bootstrap step child.
interface BootstrapRecordMessage {
  type: "lineth-bootstrap-record";
  id: string;
  record: {
    itemId: string;
    kind: "sign" | "presigned" | "script";
    chain: "l1" | "l2";
    transactionHash: string;
    blockNumber: number;
    chainId: string;
    address?: string;
  };
}

// A custom bootstrap item the child is about to broadcast, reported before any
// on-chain action so the parent can persist an in-flight intent first.
interface BootstrapIntentMessage {
  type: "lineth-bootstrap-intent";
  id: string;
  intent: {
    itemId: string;
    kind: "sign" | "presigned" | "script";
    chain: "l1" | "l2";
    nonce?: number;
  };
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

function isBootstrapRecordMessage(message: unknown): message is BootstrapRecordMessage {
  if (typeof message !== "object" || message === null) return false;
  const candidate = message as Partial<BootstrapRecordMessage>;
  return (
    candidate.type === "lineth-bootstrap-record" &&
    typeof candidate.id === "string" &&
    !!candidate.record &&
    typeof candidate.record.itemId === "string"
  );
}

function isBootstrapIntentMessage(message: unknown): message is BootstrapIntentMessage {
  if (typeof message !== "object" || message === null) return false;
  const candidate = message as Partial<BootstrapIntentMessage>;
  return (
    candidate.type === "lineth-bootstrap-intent" &&
    typeof candidate.id === "string" &&
    !!candidate.intent &&
    typeof candidate.intent.itemId === "string" &&
    (candidate.intent.kind === "sign" || candidate.intent.kind === "presigned" || candidate.intent.kind === "script") &&
    (candidate.intent.chain === "l1" || candidate.intent.chain === "l2") &&
    (candidate.intent.nonce === undefined || typeof candidate.intent.nonce === "number")
  );
}

function expectedDeployment(step: DeploymentStep, contractName: string) {
  const matches = step.deployments.filter((deployment) => deployment.contractName === contractName);
  if (matches.length !== 1) {
    throw new Error(`${step.id} emitted unexpected or ambiguous deployment ${contractName}`);
  }
  return matches[0]!;
}

interface RunChildScriptInput {
  scriptPath: string;
  environment: NodeJS.ProcessEnv;
  sensitiveValues: readonly string[];
  /** Used in "failed to capture output for X" and "X exited with code N" errors. */
  errorContextLabel: string;
  /**
   * Invoked for every IPC message the child sends, in order (never
   * concurrently). Throwing aborts the run: the child is killed and the
   * thrown error propagates once the child has fully exited.
   */
  onMessage: (message: unknown, child: ChildProcess) => Promise<void> | void;
}

/**
 * Owns the spawn/stdio/close/kill lifecycle shared by every forge-deployer
 * child script (deployment steps and the bootstrap step): pipes stdout/stderr
 * through `sanitizeText`, serializes IPC message handling so persistence
 * never races itself, and kills the child if message handling throws.
 * Callers only need to supply an `onMessage` handler for their own message
 * shape and any post-run invariants (e.g. expected record counts).
 */
async function runChildScript(input: RunChildScriptInput): Promise<void> {
  const child = spawn(process.execPath, [input.scriptPath], {
    env: childEnvironment(input.environment),
    stdio: ["ignore", "pipe", "pipe", "ipc"],
  });
  const { stdout, stderr } = child;
  if (!stdout || !stderr) throw new Error(`failed to capture output for ${input.errorContextLabel}`);

  const closePromise = new Promise<number>((resolve, reject) => {
    child.once("error", reject);
    child.once("close", (code) => resolve(code ?? 1));
  });
  const stderrPromise = (async () => {
    for await (const line of createInterface({ input: stderr })) {
      console.error(sanitizeText(line, input.sensitiveValues));
    }
  })();

  let messageProcessing = Promise.resolve();
  let messageProcessingError: unknown;
  child.on("message", (message: unknown) => {
    messageProcessing = messageProcessing
      .then(() => input.onMessage(message, child))
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
  if (exitCode !== 0) throw new Error(`${input.errorContextLabel} exited with code ${exitCode}`);
}

export async function runStepScript(input: RunStepScriptInput): Promise<void> {
  const recordedKeys = new Set<string>();

  await runChildScript({
    scriptPath: input.scriptPath,
    environment: input.environment,
    sensitiveValues: input.sensitiveValues,
    errorContextLabel: `${input.step.id} deploy script`,
    onMessage: async (message, child) => {
      if (!isDeploymentIntentMessage(message) && !isDeploymentRecordMessage(message)) return;
      try {
        if (isDeploymentIntentMessage(message)) {
          const { id, contractName } = message;
          const expected = expectedDeployment(input.step, contractName);
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
        const expected = expectedDeployment(input.step, record.contractName);
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
          error: error instanceof Error ? error.message : "deployment checkpoint persistence failed",
        });
        throw error;
      }
    },
  });

  if (recordedKeys.size !== input.step.deployments.length) {
    throw new Error(
      `${input.step.id} emitted ${recordedKeys.size} deployment records; expected ${input.step.deployments.length}`,
    );
  }
}

interface RunBootstrapScriptInput {
  scriptPath: string;
  environment: NodeJS.ProcessEnv;
  checkpoint: DeploymentCheckpoint;
  store: CheckpointStore;
  sensitiveValues: readonly string[];
  // Keys of bootstrap items the child is expected to complete (not already done).
  pendingItemKeys: string[];
}

// Runs the custom bootstrap step child. Like runStepScript, the child reports
// a `lineth-bootstrap-intent` before it may broadcast so the parent can persist
// an in-flight record first (fail-closed on rerun); each completed item then
// arrives via `lineth-bootstrap-record`, which atomically replaces the intent.
export async function runBootstrapScript(input: RunBootstrapScriptInput): Promise<void> {
  await runChildScript({
    scriptPath: input.scriptPath,
    environment: input.environment,
    sensitiveValues: input.sensitiveValues,
    errorContextLabel: "bootstrap step script",
    onMessage: async (message, child) => {
      if (!isBootstrapIntentMessage(message) && !isBootstrapRecordMessage(message)) return;
      try {
        if (isBootstrapIntentMessage(message)) {
          const { id, intent } = message;
          const key = `bootstrap.${intent.itemId}`;
          if (!input.pendingItemKeys.includes(key)) {
            throw new Error(`bootstrap step requested intent for unexpected or already-recorded item ${intent.itemId}`);
          }
          if (input.checkpoint.bootstrap[key] || input.checkpoint.inFlightBootstrap[key]) {
            throw new Error(`bootstrap step requested intent for ${intent.itemId} twice`);
          }
          input.checkpoint.inFlightBootstrap[key] = {
            kind: intent.kind,
            chain: intent.chain,
            ...(intent.nonce !== undefined ? { nonce: intent.nonce } : {}),
          };
          await input.store.save(input.checkpoint);
          child.send({ type: "lineth-bootstrap-intent-ack", id });
          return;
        }

        const { id, record } = message;
        const key = `bootstrap.${record.itemId}`;
        if (!input.pendingItemKeys.includes(key)) {
          throw new Error(`bootstrap step emitted unexpected or already-recorded item ${record.itemId}`);
        }
        if (input.checkpoint.bootstrap[key]) {
          throw new Error(`bootstrap step emitted ${record.itemId} twice`);
        }
        if (!input.checkpoint.inFlightBootstrap[key]) {
          throw new Error(`bootstrap step emitted ${record.itemId} without a durable bootstrap intent`);
        }
        const expectedChainId = input.checkpoint.chainIds[record.chain];
        if (record.chainId !== expectedChainId) {
          throw new Error(
            `bootstrap item ${record.itemId} emitted chain ID ${record.chainId}; expected ${expectedChainId}`,
          );
        }
        input.checkpoint.bootstrap[key] = {
          kind: record.kind,
          chain: record.chain,
          transactionHash: record.transactionHash,
          blockNumber: record.blockNumber,
          chainId: record.chainId,
          ...(record.address ? { address: getAddress(record.address) } : {}),
        };
        delete input.checkpoint.inFlightBootstrap[key];
        await input.store.save(input.checkpoint);
        child.send({ type: "lineth-bootstrap-record-ack", id });
      } catch (error) {
        child.send({
          type: isBootstrapIntentMessage(message) ? "lineth-bootstrap-intent-ack" : "lineth-bootstrap-record-ack",
          id: message.id,
          error: error instanceof Error ? error.message : "bootstrap checkpoint failed",
        });
        throw error;
      }
    },
  });
}
