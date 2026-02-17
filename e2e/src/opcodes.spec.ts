import { encodeFunctionCall } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";

import { estimateLineaGas, sendTransactionWithRetry } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { OpcodeTesterAbi } from "./generated";

const context = createTestContext();
const l2AccountManager = context.getL2AccountManager();

describe("Opcodes test suite", () => {
  it.concurrent("Should be able to estimate the opcode execution gas using linea_estimateGas endpoint", async () => {
    const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
    const account = await l2AccountManager.generateAccount();
    const opcodeTester = context.l2Contracts.opcodeTester(l2PublicClient);

    const { maxPriorityFeePerGas, maxFeePerGas, gas } = await estimateLineaGas(l2PublicClient, {
      account,
      to: opcodeTester.address,
      data: encodeFunctionCall({
        abi: OpcodeTesterAbi,
        functionName: "executeAllOpcodes",
      }),
    });
    logger.debug(
      `Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas} gasLimit=${gas}`,
    );

    expect(maxPriorityFeePerGas).toBeGreaterThan(0n);
    expect(maxFeePerGas).toBeGreaterThan(0n);
    expect(gas).toBeGreaterThan(0n);
  });

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
});
