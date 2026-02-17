import { etherToWei } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";
import { randomBytes } from "crypto";
import { encodeFunctionData, serializeTransaction, toHex } from "viem";

import { TRANSACTION_CALLDATA_LIMIT } from "./common/constants";
import { estimateLineaGas, sendTransactionWithRetry } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { DummyContractAbi } from "./generated";

const context = createTestContext();
const l2AccountManager = context.getL2AccountManager();

describe("Layer 2 test suite", () => {
  const lineaEstimateGasClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });

  it.concurrent("Should revert if transaction data size is above the limit", async () => {
    const account = await l2AccountManager.generateAccount();
    const walletClient = context.l2WalletClient({ account });
    const dummyContract = context.l2Contracts.dummyContract(walletClient);

    const oversizedData = toHex(randomBytes(TRANSACTION_CALLDATA_LIMIT).toString("hex"));
    logger.debug(`Generated oversized transaction data. dataLength=${oversizedData.length}`);

    await expect(dummyContract.write.setPayload([oversizedData], { gas: 5_000_000n })).rejects.toThrow(
      "Calldata of transaction is greater than the allowed max of 30000",
    );
    logger.debug("Transaction correctly reverted due to oversized data.");
  });

  it.concurrent("Should succeed if transaction data size is below the limit", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient();
    const walletClient = context.l2WalletClient({ account });
    const dummyContract = context.l2Contracts.dummyContract(walletClient);

    const payload = toHex(randomBytes(1000).toString("hex"));
    const nonce = await l2PublicClient.getTransactionCount({ address: account.address });

    const estimatedGasFees = await estimateLineaGas(lineaEstimateGasClient, {
      account,
      to: dummyContract.address,
      data: encodeFunctionData({
        abi: DummyContractAbi,
        functionName: "setPayload",
        args: [payload],
      }),
    });

    const { hash, receipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      dummyContract.write.setPayload([payload], { nonce, ...estimatedGasFees, ...fees }),
    );

    logger.debug(`Transaction receipt received. transactionHash=${hash} status=${receipt.status}`);
    expect(receipt.status).toEqual("success");
  });

  it.concurrent("Should successfully send a legacy transaction", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient();

    const { gasPrice } = await l2PublicClient.estimateFeesPerGas();
    logger.debug(`Fetched gasPrice=${gasPrice}`);

    const txHash = await context.l2WalletClient({ account }).sendTransaction({
      type: "legacy",
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      gasPrice,
      value: etherToWei("0.01"),
      gas: 4612388n,
    });

    logger.debug(`Legacy transaction sent. transactionHash=${txHash}`);

    const receipt = await l2PublicClient.waitForTransactionReceipt({ hash: txHash, timeout: 60_000 });
    logger.debug(`Legacy transaction receipt received. transactionHash=${txHash} status=${receipt.status}`);

    expect(receipt.status).toEqual("success");
  });

  it.concurrent("Should successfully send an EIP1559 transaction", async () => {
    const account = await l2AccountManager.generateAccount();
    const l2PublicClient = context.l2PublicClient();
    const walletClient = context.l2WalletClient({ account });
    const nonce = await l2PublicClient.getTransactionCount({ address: account.address });

    const estimatedGasFees = await estimateLineaGas(lineaEstimateGasClient, {
      account: account.address,
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9" as `0x${string}`,
      data: serializeTransaction({
        type: "eip1559",
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
        value: etherToWei("0.01"),
        chainId: context.getL2ChainId(),
      }),
    });

    const { hash, receipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      walletClient.sendTransaction({
        type: "eip1559",
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
        value: etherToWei("0.01"),
        nonce,
        ...estimatedGasFees,
        ...fees,
      }),
    );

    logger.debug(`EIP1559 transaction receipt received. transactionHash=${hash} status=${receipt.status}`);
    expect(receipt.status).toEqual("success");
  });

  it.concurrent("Should successfully send an access list transaction with empty access list", async () => {
    const account = await l2AccountManager.generateAccount();

    const l2PublicClient = context.l2PublicClient();
    const { gasPrice } = await l2PublicClient.estimateFeesPerGas();
    logger.debug(`Fetched gasPrice=${gasPrice}`);

    const txHash = await context.l2WalletClient({ account }).sendTransaction({
      type: "eip2930",
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      gasPrice,
      value: etherToWei("0.01"),
      gas: 21000n,
      chainId: context.getL2ChainId(),
    });

    logger.debug(`Empty access list transaction sent. transactionHash=${txHash}`);

    const receipt = await l2PublicClient.waitForTransactionReceipt({ hash: txHash, timeout: 60_000 });
    logger.debug(`Empty access list transaction receipt received. transactionHash=${txHash} status=${receipt.status}`);

    expect(receipt.status).toEqual("success");
  });

  it.concurrent("Should successfully send an access list transaction with access list", async () => {
    const account = await l2AccountManager.generateAccount();

    const l2PublicClient = context.l2PublicClient();
    const { gasPrice } = await l2PublicClient.estimateFeesPerGas();
    logger.debug(`Fetched gasPrice=${gasPrice}`);

    const accessList = [
      {
        address: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
        storageKeys: [
          "0x0000000000000000000000000000000000000000000000000000000000000000",
          "0x0000000000000000000000000000000000000000000000000000000000000001",
        ],
      },
    ] as const;

    const txHash = await context.l2WalletClient({ account }).sendTransaction({
      type: "eip2930",
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      gasPrice,
      value: etherToWei("0.01"),
      gas: 200000n,
      chainId: context.getL2ChainId(),
      accessList,
    });
    logger.debug(`Access list transaction sent. transactionHash=${txHash}`);

    const receipt = await context.l2PublicClient().waitForTransactionReceipt({ hash: txHash, timeout: 60_000 });
    logger.debug(`Access list transaction receipt received. transactionHash=${txHash} status=${receipt.status}`);

    expect(receipt.status).toEqual("success");
  });

  // TODO: discuss new frontend
  it.skip("Shomei frontend always behind while conflating multiple blocks and proving on L1", async () => {
    const account = await l2AccountManager.generateAccount();

    if (!context.isLocal()) {
      // Skip this test for dev and uat environments
      return;
    }
    const shomeiClient = context.l2PublicClient({ type: L2RpcEndpoint.Shomei });
    const shomeiFrontendClient = context.l2PublicClient({ type: L2RpcEndpoint.ShomeiFrontend });

    const l2PublicClient = context.l2PublicClient();
    const l2WalletClient = context.l2WalletClient({ account });
    for (let i = 0; i < 5; i++) {
      const { maxPriorityFeePerGas, maxFeePerGas } = await l2PublicClient.estimateFeesPerGas();
      logger.debug(
        `Fetched fee data. transactionNumber=${i + 1} maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`,
      );

      const txHash = await l2WalletClient.sendTransaction({
        type: "eip1559",
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
        maxPriorityFeePerGas,
        maxFeePerGas,
        value: etherToWei("0.01"),
        gas: 21000n,
      });

      logger.debug(`EIP1559 transaction sent. transactionHash=${txHash}`);

      const receipt = await l2PublicClient.waitForTransactionReceipt({ hash: txHash, timeout: 60_000 });
      logger.debug(`EIP1559 transaction receipt received. transactionHash=${txHash} status=${receipt?.status}`);

      const [shomeiBlock, shomeiFrontendBlock] = await Promise.all([
        shomeiClient.getZkEVMBlockNumber(),
        shomeiFrontendClient.getZkEVMBlockNumber(),
      ]);
      logger.debug(`shomeiBlock=${shomeiBlock}, shomeiFrontendBlock=${shomeiFrontendBlock}`);

      expect(shomeiBlock).toBeGreaterThan(shomeiFrontendBlock);
      logger.debug(
        `shomeiBlock is greater than shomeiFrontendBlock. shomeiBlock=${shomeiBlock} shomeiFrontendBlock=${shomeiFrontendBlock}`,
      );
    }
  }, 150_000);
});
