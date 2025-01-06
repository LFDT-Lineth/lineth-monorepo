import { ethers } from "ethers";
import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { RollupGetZkEVMBlockNumberClient, etherToWei } from "./common/utils";
import { TRANSACTION_CALLDATA_LIMIT } from "./common/constants";

const l2AccountManager = config.getL2AccountManager();

describe("Layer 2 test suite", () => {
  const l2Provider = config.getL2Provider();

  it.concurrent("Should revert if transaction data size is above the limit", async () => {
    const account = await l2AccountManager.generateAccount();
    const dummyContract = config.getL2DummyContract(account);

    await expect(
      dummyContract.connect(account).setPayload(ethers.randomBytes(TRANSACTION_CALLDATA_LIMIT)),
    ).rejects.toThrow("missing revert data");
  });

  it.concurrent("Should succeed if transaction data size is below the limit", async () => {
    const account = await l2AccountManager.generateAccount();
    const dummyContract = config.getL2DummyContract(account);
    const nonce = await l2Provider.getTransactionCount(account.address, "pending");
    const { maxPriorityFeePerGas, maxFeePerGas } = await l2Provider.getFeeData();

    const tx = await dummyContract.connect(account).setPayload(ethers.randomBytes(1000), {
      nonce: nonce,
      maxPriorityFeePerGas: maxPriorityFeePerGas,
      maxFeePerGas: maxFeePerGas,
    });

    const receipt = await tx.wait();
    expect(receipt?.status).toEqual(1);
  });

  it.concurrent("Should successfully send a legacy transaction", async () => {
    const account = await l2AccountManager.generateAccount();

    const { gasPrice } = await config.getL2Provider().getFeeData();

    const receipt = await (
      await account.sendTransaction({
        type: 0,
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
        gasPrice,
        value: etherToWei("0.01"),
        gasLimit: "0x466124",
        chainId: config.getL2ChainId(),
      })
    ).wait();

    expect(receipt).not.toBeNull();
  });

  it.concurrent("Should successfully send an EIP1559 transaction", async () => {
    const account = await l2AccountManager.generateAccount();

    const { maxPriorityFeePerGas, maxFeePerGas } = await config.getL2Provider().getFeeData();

    const receipt = await (
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

    expect(receipt).not.toBeNull();
  });

  it.concurrent("Should successfully send an access list transaction with empty access list", async () => {
    const account = await l2AccountManager.generateAccount();

    const { gasPrice } = await config.getL2Provider().getFeeData();

    const receipt = await (
      await account.sendTransaction({
        type: 1,
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
        gasPrice,
        value: etherToWei("0.01"),
        gasLimit: "21000",
        chainId: config.getL2ChainId(),
      })
    ).wait();

    expect(receipt).not.toBeNull();
  });

  it.concurrent("Should successfully send an access list transaction with access list", async () => {
    const account = await l2AccountManager.generateAccount();

    const { gasPrice } = await config.getL2Provider().getFeeData();
    const accessList = {
      "0x8D97689C9818892B700e27F316cc3E41e17fBeb9": [
        "0x0000000000000000000000000000000000000000000000000000000000000000",
        "0x0000000000000000000000000000000000000000000000000000000000000001",
      ],
    };

    const receipt = await (
      await account.sendTransaction({
        type: 1,
        to: "0x8D97689C9818892B700e27F316cc3E41e17fBeb9",
        gasPrice,
        value: etherToWei("0.01"),
        gasLimit: "200000",
        chainId: config.getL2ChainId(),
        accessList: ethers.accessListify(accessList),
      })
    ).wait();

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
    }
  }, 150_000);
});
