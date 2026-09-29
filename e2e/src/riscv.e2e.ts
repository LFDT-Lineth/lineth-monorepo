import { describe, expect, it } from "@jest/globals";
import { execFile } from "node:child_process";
import { readdir, readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { promisify } from "node:util";
import { encodeDeployData, encodeFunctionData, keccak256, parseEther } from "viem";

import { withDenyListAddresses } from "./common/test-helpers/deny-list";
import { awaitUntil, estimateLineaGas, sendTransactionWithRetry } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { DummyContractAbi, DummyContractAbiBytecode } from "./generated";

import type { Hex } from "viem";

const context = createTestContext();
const client = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });
const executionDir = resolve(__dirname, "../../tmp/local/prover/riscv/execution");
const execFileAsync = promisify(execFile);

type ExecutionRequest = {
  programVk: Hex;
  metadata: { startBlockNumber: number; endBlockNumber: number; totalGasUsed: number };
  proofRequest: {
    chainConfig: { chainId: number; forkName: string };
    payloads: {
      statelessInput: {
        newPayloadRequest: {
          executionPayload: { blockNumber: number; blockHash: Hex; transactions: Hex[]; blockAccessList: Hex };
        };
        executionWitness: { state: Hex[]; codes: Hex[]; headers: Hex[] };
      };
    }[];
  };
};

describe("RISC-V execution stack", () => {
  it("estimates gas and transfers ETH", async () => {
    const [sender, recipient] = await context.getL2AccountManager().generateAccounts(2);
    const wallet = context.l2WalletClient({ account: sender });
    const balanceBefore = await client.getBalance({ address: recipient.address });
    const transaction = { to: recipient.address, value: parseEther("0.01") };
    const estimate = await estimateLineaGas(client, { account: sender, ...transaction });
    expect(estimate.gas).toBeGreaterThanOrEqual(21_000n);
    const { receipt } = await sendTransactionWithRetry(client, (fees) =>
      wallet.sendTransaction({ ...transaction, ...estimate, ...fees }),
    );
    expect(receipt.status).toBe("success");
    expect(await client.getBalance({ address: recipient.address })).toBe(balanceBefore + transaction.value);
  });

  it("executes a contract call and persists its dummy execution proof", async () => {
    const account = await context.getL2AccountManager().generateAccount();
    const wallet = context.l2WalletClient({ account });
    const deployment = encodeDeployData({ abi: DummyContractAbi, bytecode: DummyContractAbiBytecode });
    const deploymentEstimate = await estimateLineaGas(client, { account, data: deployment });
    const { receipt: deploymentReceipt } = await sendTransactionWithRetry(client, (fees) =>
      wallet.sendTransaction({ data: deployment, ...deploymentEstimate, ...fees }),
    );
    expect(deploymentReceipt.status).toBe("success");
    expect(deploymentReceipt.contractAddress).not.toBeNull();
    const address = deploymentReceipt.contractAddress!;
    const data = encodeFunctionData({ abi: DummyContractAbi, functionName: "setAge", args: [42n] });
    const estimate = await estimateLineaGas(client, { account, to: address, data });
    const { hash, receipt } = await sendTransactionWithRetry(client, (fees) =>
      wallet.sendTransaction({ to: address, data, ...estimate, ...fees }),
    );
    expect(receipt.status).toBe("success");
    expect(await client.readContract({ address, abi: DummyContractAbi, functionName: "age" })).toBe(42n);

    // Select the batch containing this run's transaction, so stale proofs cannot satisfy the test.
    const fileName = await awaitUntil(
      async () =>
        (await readdir(resolve(executionDir, "requests"))).find((name) => {
          const match = /^(\d+)-(\d+)-.*getZkL2ExecutionProof\.json$/.exec(name);
          return match && BigInt(match[1]) <= receipt.blockNumber && BigInt(match[2]) >= receipt.blockNumber;
        }),
      (name) => name !== undefined,
    );
    const request: ExecutionRequest = JSON.parse(await readFile(resolve(executionDir, "requests", fileName!), "utf8"));
    const { startBlockNumber, endBlockNumber } = request.metadata;
    expect(request.proofRequest.chainConfig).toMatchObject({ chainId: context.getL2ChainId(), forkName: "Amsterdam" });
    expect(request.programVk).toMatch(/^0x[0-9a-f]{64}$/i);
    expect(request.metadata.totalGasUsed).toBeGreaterThan(0);
    const inputs = request.proofRequest.payloads.map((payload) => payload.statelessInput);
    expect(inputs.map((input) => input.newPayloadRequest.executionPayload.blockNumber)).toEqual(
      Array.from({ length: endBlockNumber - startBlockNumber + 1 }, (_, index) => startBlockNumber + index),
    );
    const input = inputs.find(
      (entry) => BigInt(entry.newPayloadRequest.executionPayload.blockNumber) === receipt.blockNumber,
    )!;
    expect(input.newPayloadRequest.executionPayload.blockHash).toBe(receipt.blockHash);
    expect(
      input.newPayloadRequest.executionPayload.transactions.map((transaction) => keccak256(transaction)),
    ).toContain(hash);
    expect(input.newPayloadRequest.executionPayload.blockAccessList).toMatch(/^0x[0-9a-f]+$/i);
    expect(input.executionWitness.state.length).toBeGreaterThan(0);
    expect(input.executionWitness.codes.length).toBeGreaterThan(0);
    expect(input.executionWitness.headers.length).toBeGreaterThan(0);

    const response = await awaitUntil(
      async () => JSON.parse(await readFile(resolve(executionDir, "responses", fileName!), "utf8")),
      () => true,
    );
    expect(response).toMatchObject({
      startBlockNumber,
      publicInputs: { endBlockNumber },
      programVk: request.programVk,
      proof: "0x00",
    });
    // A response file alone does not prove that the coordinator consumed it. Status 2 is Batch.Status.Proven.
    await awaitUntil(
      async () => {
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
            "-tAc",
            `SELECT status FROM batches WHERE start_block_number = ${BigInt(startBlockNumber)} AND end_block_number = ${BigInt(endBlockNumber)}`,
          ],
          { timeout: 10_000 },
        );
        return stdout.trim();
      },
      (status) => status === "2",
      { pollingIntervalMs: 1_000 },
    );
  });

  it.each(["sender", "recipient"] as const)("rejects a denylisted %s and accepts after removal", async (denied) => {
    const [sender, recipient] = await context.getL2AccountManager().generateAccounts(2);
    const wallet = context.l2WalletClient({ account: sender });
    const transaction = { to: recipient.address, value: 1n };
    const estimate = await estimateLineaGas(client, { account: sender, ...transaction });
    const serializedTransaction = await wallet.signTransaction({
      ...transaction,
      ...estimate,
      nonce: await client.getTransactionCount({ address: sender.address }),
    });
    await withDenyListAddresses(client, [denied === "sender" ? sender.address : recipient.address], async () => {
      await expect(client.sendRawTransaction({ serializedTransaction })).rejects.toThrow(
        "is blocked as appearing on the SDN",
      );
    });
    const { receipt } = await sendTransactionWithRetry(client, (fees) =>
      wallet.sendTransaction({ ...transaction, ...estimate, ...fees }),
    );
    expect(receipt.status).toBe("success");
  });
});
