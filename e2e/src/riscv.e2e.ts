import { describe, expect, it } from "@jest/globals";
import { execFile } from "node:child_process";
import { posix } from "node:path";
import { promisify } from "node:util";
import { encodeFunctionData, keccak256, stringToHex } from "viem";

import { awaitUntil, estimateLineaGas, sendTransactionWithRetry } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";

import type { Hex, TransactionReceipt } from "viem";

const context = createTestContext();
const client = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });
// prover-ray (root) writes owner-only files, so read them inside its container; the host user can't.
const PROVER_CONTAINER = "prover-ray";
const executionDir = "/data/prover/riscv/execution";
const execFileAsync = promisify(execFile);

// Must match the prover-ray config mounted by `make start-env-with-riscv`.
const proverMode = process.env.RISCV_PROVER_MODE ?? "dev-mock";
const proofTimeoutMs = proverMode === "dev-zkvm" ? 20 * 60_000 : 2 * 60_000;
const BATCH_STATUS_PROVEN = 2;

// Together these execute every EVM opcode and precompile.
const OPCODE_TESTER_FUNCTIONS = [
  "executeAllOpcodes",
  "executePrecompiles",
  "executeG1BLSPrecompiles",
  "executeG2BLSPrecompiles",
  "executeBLSPairingAndMapPrecompiles",
] as const;

type ExecutionRequest = {
  guestProgramId: Hex;
  metadata: { startBlockNumber: number; endBlockNumber: number };
  proofRequest: {
    chainConfig: { chainId: number; forkName: string };
    payloads: {
      statelessInput: {
        newPayloadRequest: { executionPayload: { blockNumber: number; blockHash: Hex; transactions: Hex[] } };
        executionWitness: { codes: Hex[] };
      };
    }[];
  };
};

type ExecutionResponse = {
  proverVersion: string;
  startBlockNumber: number;
  proof: Hex;
  programVk: Hex;
  publicInputs: {
    parentBlockHash: Hex;
    endBlockHash: Hex;
    endBlockNumber: number;
    endBlockTimestamp: number;
    blockCount: number;
  };
};

type Batch = { startBlockNumber: number; endBlockNumber: number; status: number };

async function queryBatches(): Promise<Batch[]> {
  const { stdout } = await execFileAsync(
    "docker",
    [
      "exec",
      "postgres",
      "psql",
      "-U",
      "postgres",
      "-d",
      "linea_coordinator",
      "-tAF,",
      "-c",
      "SELECT start_block_number, end_block_number, status FROM batches ORDER BY start_block_number",
    ],
    { timeout: 10_000 },
  );
  return stdout
    .trim()
    .split("\n")
    .filter((line) => line.length > 0)
    .map((line) => {
      const [startBlockNumber, endBlockNumber, status] = line.split(",").map(Number);
      return { startBlockNumber, endBlockNumber, status };
    });
}

const FAILURE_SUFFIX = /\.failure\.\d+$/;

class ProverFailureError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ProverFailureError";
  }
}

const retryUnlessProverFailed = (error: unknown) => !(error instanceof ProverFailureError);

async function proverExec(script: string, path: string): Promise<string> {
  const { stdout } = await execFileAsync("docker", ["exec", PROVER_CONTAINER, "sh", "-c", script, "sh", path], {
    timeout: 10_000,
    maxBuffer: 64 * 1024 * 1024,
  });
  return stdout;
}

async function readdirIfExists(subdir: string): Promise<string[]> {
  const stdout = await proverExec('if [ -d "$1" ]; then ls -1A "$1"; fi', posix.join(executionDir, subdir));
  return stdout.split("\n").filter((name) => name.length > 0);
}

async function readExecutionFile(subdir: string, name: string): Promise<string> {
  return proverExec('cat "$1"', posix.join(executionDir, subdir, name));
}

async function throwOnProverFailure(): Promise<void> {
  const failed = (await readdirIfExists("requests-done")).filter((name) => FAILURE_SUFFIX.test(name));
  if (failed.length === 0) return;

  const responseName = failed[0].replace(FAILURE_SUFFIX, "");
  const response = await readExecutionFile("responses", responseName).catch(() => "<no response body>");
  throw new ProverFailureError(
    `prover-ray failed ${failed.length} request(s), first ${failed[0]}: ${response.slice(0, 2_000)}`,
  );
}

// prover-ray moves answered requests to requests-done/ with a .success or .failure.<n> suffix.
async function awaitArchivedRequest(blockNumber: bigint): Promise<string> {
  const archived = await awaitUntil(
    async () => {
      await throwOnProverFailure();
      return (await readdirIfExists("requests-done")).find((name) => {
        const match = /^(\d+)-(\d+)-.*getZkL2ExecutionProof\.json\.success$/.exec(name);
        return match !== null && BigInt(match[1]) <= blockNumber && BigInt(match[2]) >= blockNumber;
      });
    },
    (name) => name !== undefined,
    { timeoutMs: proofTimeoutMs, pollingIntervalMs: 2_000, shouldRetry: retryUnlessProverFailed },
  );
  return archived!;
}

async function queryBatchesOrThrowOnProverFailure(): Promise<Batch[]> {
  await throwOnProverFailure();
  return queryBatches();
}

async function expectProvenBatch(receipt: TransactionReceipt) {
  const archivedName = await awaitArchivedRequest(receipt.blockNumber);
  expect(archivedName).toMatch(/\.success$/);
  const fileName = archivedName.replace(/\.success$/, "");

  const request: ExecutionRequest = JSON.parse(await readExecutionFile("requests-done", archivedName));
  const { startBlockNumber, endBlockNumber } = request.metadata;
  expect(request.guestProgramId).toMatch(/^0x[0-9a-f]{64}$/i);
  expect(request.proofRequest.chainConfig).toMatchObject({ chainId: context.getL2ChainId(), forkName: "Amsterdam" });
  const input = request.proofRequest.payloads
    .map((payload) => payload.statelessInput)
    .find((entry) => BigInt(entry.newPayloadRequest.executionPayload.blockNumber) === receipt.blockNumber);
  if (!input) throw new Error(`Block ${receipt.blockNumber} missing from ${archivedName}`);
  expect(input.newPayloadRequest.executionPayload.blockHash).toBe(receipt.blockHash);
  expect(input.newPayloadRequest.executionPayload.transactions.map((tx) => keccak256(tx))).toContain(
    receipt.transactionHash,
  );
  expect(input.executionWitness.codes.length).toBeGreaterThan(0);

  const response: ExecutionResponse = JSON.parse(await readExecutionFile("responses", fileName));
  expect(response).not.toHaveProperty("error");
  expect(response).toMatchObject({
    proverVersion: `riscv-local-${proverMode}`,
    startBlockNumber,
    publicInputs: { endBlockNumber },
    programVk: request.guestProgramId,
    proof: stringToHex(`dev-proof:${proverMode}`),
  });

  if (proverMode === "dev-zkvm") {
    // dev-zkvm derives the public inputs by executing the guest.
    const [parentBlock, endBlock] = await Promise.all([
      client.getBlock({ blockNumber: BigInt(startBlockNumber - 1) }),
      client.getBlock({ blockNumber: BigInt(endBlockNumber) }),
    ]);
    expect(response.publicInputs).toMatchObject({
      parentBlockHash: parentBlock.hash,
      endBlockHash: endBlock.hash,
      endBlockTimestamp: Number(endBlock.timestamp),
      blockCount: endBlockNumber - startBlockNumber + 1,
    });
  }

  // A response file alone does not prove that the coordinator consumed it.
  await awaitUntil(
    async () =>
      (await queryBatchesOrThrowOnProverFailure()).find((batch) => batch.startBlockNumber === startBlockNumber),
    (batch) => batch?.endBlockNumber === endBlockNumber && batch.status === BATCH_STATUS_PROVEN,
    { pollingIntervalMs: 1_000, shouldRetry: retryUnlessProverFailed },
  );
}

describe(`RISC-V execution proving (${proverMode})`, () => {
  it(
    "proves batches executing every EVM opcode and precompile",
    async () => {
      const accounts = await context.getL2AccountManager().generateAccounts(OPCODE_TESTER_FUNCTIONS.length);
      const opcodeTester = context.l2Contracts.opcodeTester(client);

      const receipts = await Promise.all(
        OPCODE_TESTER_FUNCTIONS.map(async (functionName, index) => {
          const account = accounts[index];
          const wallet = context.l2WalletClient({ account });
          const data = encodeFunctionData({ abi: opcodeTester.abi, functionName });
          const estimate = await estimateLineaGas(client, { account, to: opcodeTester.address, data });
          const { receipt } = await sendTransactionWithRetry(client, (fees) =>
            wallet.sendTransaction({ to: opcodeTester.address, data, ...estimate, ...fees }),
          );
          expect(receipt.status).toBe("success");
          return receipt;
        }),
      );

      for (const receipt of receipts) {
        await expectProvenBatch(receipt);
      }
    },
    proofTimeoutMs + 5 * 60_000,
  );

  it(
    "proves a contiguous chain of batches from genesis without prover failures",
    async () => {
      // At least two batches, so contiguity is actually checked.
      const batches = await awaitUntil(queryBatchesOrThrowOnProverFailure, (rows) => rows.length >= 2, {
        timeoutMs: proofTimeoutMs,
        pollingIntervalMs: 2_000,
        shouldRetry: retryUnlessProverFailed,
      });
      expect(batches[0].startBlockNumber).toBe(1);
      batches.forEach((batch, index) => {
        expect(batch.status).toBe(BATCH_STATUS_PROVEN);
        if (index > 0) {
          expect(batch.startBlockNumber).toBe(batches[index - 1].endBlockNumber + 1);
        }
      });

      await throwOnProverFailure();
    },
    proofTimeoutMs + 60_000,
  );
});
