import { ethers, Transaction } from "ethers";
import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { LineaEstimateGasClient, RollupGetZkEVMBlockNumberClient, etherToWei } from "./common/utils";
import { TRANSACTION_CALLDATA_LIMIT } from "./common/constants";

const l2AccountManager = config.getL2AccountManager();

describe("Layer 2 test suite", () => {
  const lineaEstimateGasClient = new LineaEstimateGasClient(config.getL2BesuNodeEndpoint()!);

  it.concurrent("Should revert if transaction data size is above the limit", async () => {
    const account = await l2AccountManager.generateAccount();
    const dummyContract = config.getL2DummyContract(account);

    const oversizedData = ethers.randomBytes(TRANSACTION_CALLDATA_LIMIT);
    logger.debug(`Generated oversized transaction data. dataLength=${oversizedData.length}`);

    // Provide explicit gasLimit to bypass estimateGas and ensure the transaction
    // is submitted directly to the txpool where CalldataValidator rejects it.
    await expect(dummyContract.connect(account).setPayload(oversizedData, { gasLimit: 5_000_000 })).rejects.toThrow(
      "Calldata of transaction is greater than the allowed max of 30000",
    );
    logger.debug("Transaction correctly reverted due to oversized data.");
  });

  it.concurrent("Should succeed if transaction data size is below the limit", async () => {
    const account = await l2AccountManager.generateAccount();
    const dummyContract = config.getL2DummyContract(account);

    const { maxPriorityFeePerGas, maxFeePerGas } = await lineaEstimateGasClient.lineaEstimateGas(
      account.address,
      await dummyContract.getAddress(),
      dummyContract.interface.encodeFunctionData("setPayload", [ethers.randomBytes(1000)]),
    );
    logger.debug(`Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`);

    const tx = await dummyContract.connect(account).setPayload(ethers.randomBytes(1000), {
      maxPriorityFeePerGas: maxPriorityFeePerGas,
      maxFeePerGas: maxFeePerGas,
    });
    logger.debug(`setPayload transaction sent. transactionHash=${tx.hash}`);

    const receipt = await tx.wait();
    logger.debug(`Transaction receipt received. transactionHash=${tx.hash} status=${receipt?.status}`);

    expect(receipt?.status).toEqual(1);
  });

  it.concurrent("Should successfully send a legacy transaction", async () => {
    const account = await l2AccountManager.generateAccount();

    const { gasPrice } = await config.getL2Provider().getFeeData();
    logger.debug(`Fetched gasPrice=${gasPrice}`);

    const tx = await account.sendTransaction({
      type: 0,
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      gasPrice,
      value: etherToWei("0.01"),
      gasLimit: "0x466124",
      chainId: config.getL2ChainId(),
    });

    logger.debug(`Legacy transaction sent. transactionHash=${tx.hash}`);

    const receipt = await tx.wait();
    logger.debug(`Legacy transaction receipt received. transactionHash=${tx.hash} status=${receipt?.status}`);

    expect(receipt).not.toBeNull();
  });

  it.concurrent("Should successfully send an EIP1559 transaction", async () => {
    const account = await l2AccountManager.generateAccount();
    const transaction = Transaction.from({
      type: 2,
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      value: etherToWei("0.01"),
      chainId: config.getL2ChainId(),
    });

    const { maxPriorityFeePerGas, maxFeePerGas, gasLimit } = await lineaEstimateGasClient.lineaEstimateGas(
      account.address,
      "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      transaction.unsignedSerialized,
    );

    logger.debug(`Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`);

    const tx = await account.sendTransaction({
      ...transaction.toJSON(),
      gasLimit,
      maxPriorityFeePerGas,
      maxFeePerGas,
    });

    logger.debug(`EIP1559 transaction sent. transactionHash=${tx.hash}`);

    const receipt = await tx.wait();
    logger.debug(`EIP1559 transaction receipt received. transactionHash=${tx.hash} status=${receipt?.status}`);

    expect(receipt).not.toBeNull();
  });

  it.concurrent("Should successfully send an access list transaction with empty access list", async () => {
    const account = await l2AccountManager.generateAccount();

    const { gasPrice } = await config.getL2Provider().getFeeData();
    logger.debug(`Fetched gasPrice=${gasPrice}`);

    const tx = await account.sendTransaction({
      type: 1,
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      gasPrice,
      value: etherToWei("0.01"),
      gasLimit: "21000",
      chainId: config.getL2ChainId(),
    });

    logger.debug(`Empty access list transaction sent. transactionHash=${tx.hash}`);

    const receipt = await tx.wait();
    logger.debug(
      `Empty access list transaction receipt received. transactionHash=${tx.hash} status=${receipt?.status}`,
    );

    expect(receipt).not.toBeNull();
  });

  it.concurrent("Should successfully send an access list transaction with access list", async () => {
    const account = await l2AccountManager.generateAccount();

    const { gasPrice } = await config.getL2Provider().getFeeData();
    logger.debug(`Fetched gasPrice=${gasPrice}`);

    const accessList = {
      "0x8D97689C9818892B700e27F316cc3E41e17fBeb9": [
        "0x0000000000000000000000000000000000000000000000000000000000000000",
        "0x0000000000000000000000000000000000000000000000000000000000000001",
      ],
    };

    const tx = await account.sendTransaction({
      type: 1,
      to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
      gasPrice,
      value: etherToWei("0.01"),
      gasLimit: "200000",
      chainId: config.getL2ChainId(),
      accessList: ethers.accessListify(accessList),
    });
    logger.debug(`Access list transaction sent. transactionHash=${tx.hash}`);

    const receipt = await tx.wait();
    logger.debug(`Access list transaction receipt received. transactionHash=${tx.hash} status=${receipt?.status}`);

    expect(receipt).not.toBeNull();
  });

  // TODO: discuss new frontend
  it.skip("Shomei frontend always behind while conflating multiple blocks and proving on L1", async () => {
    const account = await l2AccountManager.generateAccount();

    const shomeiEndpoint = config.getShomeiEndpoint();
    const shomeiFrontendEndpoint = config.getShomeiFrontendEndpoint();

    if (!shomeiEndpoint || !shomeiFrontendEndpoint) {
      // Skip this test for dev and uat environments
      return;
    }
    const shomeiClient = new RollupGetZkEVMBlockNumberClient(shomeiEndpoint);
    const shomeiFrontendClient = new RollupGetZkEVMBlockNumberClient(shomeiFrontendEndpoint);

    for (let i = 0; i < 5; i++) {
      const { maxPriorityFeePerGas, maxFeePerGas } = await config.getL2Provider().getFeeData();
      logger.debug(
        `Fetched fee data. transactionNumber=${i + 1} maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`,
      );

      await (
        await account.sendTransaction({
          type: 2,
          to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
          maxPriorityFeePerGas,
          maxFeePerGas,
          value: etherToWei("0.01"),
          gasLimit: "21000",
          chainId: config.getL2ChainId(),
        })
      ).wait();

      const [shomeiBlock, shomeiFrontendBlock] = await Promise.all([
        shomeiClient.rollupGetZkEVMBlockNumber(),
        shomeiFrontendClient.rollupGetZkEVMBlockNumber(),
      ]);
      logger.debug(`shomeiBlock=${shomeiBlock}, shomeiFrontendBlock=${shomeiFrontendBlock}`);

      expect(shomeiBlock).toBeGreaterThan(shomeiFrontendBlock);
      logger.debug(
        `shomeiBlock is greater than shomeiFrontendBlock. shomeiBlock=${shomeiBlock} shomeiFrontendBlock=${shomeiFrontendBlock}`,
      );
    }
  }, 150_000);
});
