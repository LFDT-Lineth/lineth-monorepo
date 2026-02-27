import { encodeFunctionCall } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";

import { estimateLineaGas, sendTransactionWithRetry } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { OpcodeTesterAbi } from "./generated";

const context = createTestContext();
const l2AccountManager = context.getL2AccountManager();

describe("Opcodes test suite", () => {
  it.concurrent("Should be able to execute all opcodes", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
    const opcodeTesterRead = context.l2Contracts.opcodeTester(l2PublicClient);
    const walletClient = context.l2WalletClient({ account });
    const opcodeTesterWrite = context.l2Contracts.opcodeTester(walletClient);

    const valueBeforeExecution = await opcodeTesterRead.read.rollingBlockDetailComputations();

    const txEstimationParams = {
      account,
      to: opcodeTesterRead.address,
      data: encodeFunctionCall({
        abi: OpcodeTesterAbi,
        functionName: "executeAllOpcodes",
      }),
    };

    const estimatedGasFees = await estimateLineaGas(l2PublicClient, txEstimationParams);
    const nonce = await l2PublicClient.getTransactionCount({ address: account.address });

    await sendTransactionWithRetry(l2PublicClient, (fees) =>
      opcodeTesterWrite.write.executeAllOpcodes({ nonce, ...estimatedGasFees, ...fees }),
    );

    const valueAfterExecution = await opcodeTesterRead.read.rollingBlockDetailComputations();

    logger.debug(`Value before execution: ${valueBeforeExecution}, value after execution: ${valueAfterExecution}`);
    expect(valueBeforeExecution).not.toEqual(valueAfterExecution);

    logger.debug("All opcodes executed successfully");
  });

  it.concurrent("Should be able to execute precompiles (P256VERIFY, KZG)", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
    const walletClient = context.l2WalletClient({ account });
    const opcodeTester = context.l2Contracts.opcodeTester(walletClient);

    const txEstimationParams = {
      account,
      to: opcodeTester.address,
      data: encodeFunctionCall({
        abi: OpcodeTesterAbi,
        functionName: "executePrecompiles",
      }),
    };

    const estimatedGasFees = await estimateLineaGas(l2PublicClient, txEstimationParams);
    const nonce = await l2PublicClient.getTransactionCount({ address: account.address });

    const { receipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      opcodeTester.write.executePrecompiles({ nonce, ...estimatedGasFees, ...fees }),
    );

    expect(receipt.status).toBe("success");
    logger.debug("Precompiles executed successfully");
  });

  it.concurrent("Should be able to execute G1 BLS precompiles", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
    const walletClient = context.l2WalletClient({ account });
    const opcodeTester = context.l2Contracts.opcodeTester(walletClient);

    const txEstimationParams = {
      account,
      to: opcodeTester.address,
      data: encodeFunctionCall({
        abi: OpcodeTesterAbi,
        functionName: "executeG1BLSPrecompiles",
      }),
    };

    const estimatedGasFees = await estimateLineaGas(l2PublicClient, txEstimationParams);
    const nonce = await l2PublicClient.getTransactionCount({ address: account.address });

    const { receipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      opcodeTester.write.executeG1BLSPrecompiles({ nonce, ...estimatedGasFees, ...fees }),
    );

    expect(receipt.status).toBe("success");
    logger.debug("G1 BLS precompiles executed successfully");
  });

  it.concurrent("Should be able to execute G2 BLS precompiles", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
    const walletClient = context.l2WalletClient({ account });
    const opcodeTester = context.l2Contracts.opcodeTester(walletClient);

    const txEstimationParams = {
      account,
      to: opcodeTester.address,
      data: encodeFunctionCall({
        abi: OpcodeTesterAbi,
        functionName: "executeG2BLSPrecompiles",
      }),
    };

    const estimatedGasFees = await estimateLineaGas(l2PublicClient, txEstimationParams);
    const nonce = await l2PublicClient.getTransactionCount({ address: account.address });

    const { receipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      opcodeTester.write.executeG2BLSPrecompiles({ nonce, ...estimatedGasFees, ...fees }),
    );

    expect(receipt.status).toBe("success");
    logger.debug("G2 BLS precompiles executed successfully");
  });

  it.concurrent("Should be able to execute BLS pairing and map precompiles", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
    const walletClient = context.l2WalletClient({ account });
    const opcodeTester = context.l2Contracts.opcodeTester(walletClient);

    const txEstimationParams = {
      account,
      to: opcodeTester.address,
      data: encodeFunctionCall({
        abi: OpcodeTesterAbi,
        functionName: "executeBLSPairingAndMapPrecompiles",
      }),
    };

    const estimatedGasFees = await estimateLineaGas(l2PublicClient, txEstimationParams);
    const nonce = await l2PublicClient.getTransactionCount({ address: account.address });

    const { receipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      opcodeTester.write.executeBLSPairingAndMapPrecompiles({ nonce, ...estimatedGasFees, ...fees }),
    );

    expect(receipt.status).toBe("success");
    logger.debug("BLS pairing and map precompiles executed successfully");
  });
});
