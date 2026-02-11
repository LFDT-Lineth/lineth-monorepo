import { serialize } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";
import { PrepareTransactionRequestReturnType, SendRawTransactionErrorType, encodeFunctionData, parseGwei } from "viem";

import { getTransactionHash, awaitUntil } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { TestContractAbi } from "./generated";

const context = createTestContext();
const l2AccountManager = context.getL2AccountManager();

describe("Transaction exclusion test suite", () => {
  it.concurrent(
    "Should get the status of the rejected transaction reported from Besu RPC node",
    async () => {
      const transactionExclusionClient = context.l2PublicClient({
        type: L2RpcEndpoint.TransactionExclusion,
        httpConfig: { timeout: 60_000 },
      });

      const l2Account = await l2AccountManager.generateAccount();

      const l2WalletClient = context.l2WalletClient({
        type: L2RpcEndpoint.BesuNode,
        account: l2Account,
        httpConfig: { timeout: 60_000 },
      });
      const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
      const testContract = context.l2Contracts.testContract(l2PublicClient);

      const txRequest = await l2WalletClient.prepareTransactionRequest({
        account: l2Account,
        to: testContract.address,
        data: encodeFunctionData({
          abi: TestContractAbi,
          functionName: "testAddmod",
          args: [13000n, 31n],
        }),
        maxPriorityFeePerGas: parseGwei("1"),
        maxFeePerGas: parseGwei("10"),
      });

      const rejectedTxHash = await getTransactionHash(l2PublicClient, txRequest);

      try {
        const serializedTransaction = await l2WalletClient.signTransaction(
          txRequest as PrepareTransactionRequestReturnType,
        );

        // This shall be rejected by the Besu node due to traces module limit overflow
        await l2WalletClient.sendRawTransaction({ serializedTransaction });
        throw new Error("Transaction was expected to be rejected, but it was not.");
      } catch (err) {
        const e = err as SendRawTransactionErrorType;

        if (e.name === "InvalidInputRpcError") {
          // This shall return error with traces limit overflow
          logger.debug(`sendTransaction expected rejection: ${serialize(err)}`);
          // assert it was indeed rejected by the traces module limit
          expect(e.details).toContain("is above the limit");
        } else {
          throw new Error("Transaction was expected to be rejected with traces limit overflow, but it was not.");
        }
      }

      logger.debug(`Transaction rejected as expected (RPC). transactionHash=${rejectedTxHash}`);

      const exclusionStatus = await awaitUntil(
        async () => {
          const status = await transactionExclusionClient.getTransactionExclusionStatusV1({ txHash: rejectedTxHash });
          logger.debug(`Polling for transaction exclusion status... response=${serialize(status)}`);
          return status;
        },
        (status) => status !== null && status !== undefined,
        { pollingIntervalMs: 1_000, timeoutMs: 100_000 },
      );

      logger.debug(`Transaction exclusion status received. response=${serialize(exclusionStatus)}`);

      expect(exclusionStatus.txHash).toStrictEqual(rejectedTxHash);
      expect(exclusionStatus.txRejectionStage).toStrictEqual("RPC");
      expect(exclusionStatus.from.toLowerCase()).toStrictEqual(l2Account.address.toLowerCase());
    },
    120_000,
  );

  it.skip("Should get the status of the rejected transaction reported from Besu SEQUENCER node", async () => {
    const transactionExclusionClient = context.l2PublicClient({
      type: L2RpcEndpoint.TransactionExclusion,
      httpConfig: { timeout: 60_000 },
    });
    const l2Account = await l2AccountManager.generateAccount();
    const walletClient = context.l2WalletClient({
      type: L2RpcEndpoint.Sequencer,
      account: l2Account,
      httpConfig: { timeout: 60_000 },
    });
    const testContract = context.l2Contracts.testContract(walletClient);

    // This shall be rejected by sequencer due to traces module limit overflow
    const rejectedTxHash = await testContract.write.testAddmod([13000n, 31n]);
    logger.debug(`Transaction rejected as expected (SEQUENCER). transactionHash=${rejectedTxHash}`);

    const exclusionStatus = await awaitUntil(
      async () => {
        const status = await transactionExclusionClient.getTransactionExclusionStatusV1({ txHash: rejectedTxHash });
        logger.debug(`Polling for transaction exclusion status... response=${serialize(status)}`);
        return status;
      },
      (status) => status !== null && status !== undefined,
      { pollingIntervalMs: 1_000, timeoutMs: 100_000 },
    );

    logger.debug(`Transaction exclusion status received. response=${serialize(exclusionStatus)}`);

    expect(exclusionStatus.txHash).toStrictEqual(rejectedTxHash);
    expect(exclusionStatus.txRejectionStage).toStrictEqual("SEQUENCER");
    expect(exclusionStatus.from.toLowerCase()).toStrictEqual(l2Account.address.toLowerCase());
  }, 120_000);
});
