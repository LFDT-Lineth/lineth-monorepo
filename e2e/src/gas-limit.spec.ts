import { describe, expect, it } from "@jest/globals";
import { pollForContractMethodReturnValueExceedTarget, wait } from "./common/utils";
import { config } from "./config/tests-config";
import { ContractTransactionReceipt, Wallet } from "ethers";

const l2AccountManager = config.getL2AccountManager();
const l2Provider = config.getL2Provider();

describe("Gas limit test suite", () => {
  const setGasLimit = async (account: Wallet): Promise<ContractTransactionReceipt | null> => {
    logger.debug(`setGasLimit called with account=${account.address}`);

    const opcodeTestContract = config.getOpcodeTestContract(account);
    const nonce = await l2Provider.getTransactionCount(account.address, "pending");
    logger.debug(`Fetched nonce. nonce=${nonce} account=${account.address}`);

    const { maxPriorityFeePerGas, maxFeePerGas } = await l2Provider.getFeeData();
    logger.debug(`Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`);

    const tx = await opcodeTestContract.connect(account).setGasLimit({
      nonce: nonce,
      maxPriorityFeePerGas: maxPriorityFeePerGas,
      maxFeePerGas: maxFeePerGas,
    });
    logger.debug(`setGasLimit transaction sent. transactionHash=${tx.hash}`);

    const receipt = await tx.wait();
    logger.debug(`Transaction receipt received. transactionHash=${tx.hash} status=${receipt?.status}`);

    return receipt;
  };

  const getGasLimit = async (): Promise<bigint> => {
    const opcodeTestContract = config.getOpcodeTestContract();
    const gasLimit = await opcodeTestContract.getGasLimit();

    logger.debug(`Current gas limit retrieved. gasLimit=${gasLimit}`);

    return gasLimit;
  };

  it.concurrent("Should successfully invoke OpcodeTestContract.setGasLimit()", async () => {
    const account = await l2AccountManager.generateAccount();
    const receipt = await setGasLimit(account);
    expect(receipt?.status).toEqual(1);
  });

  it.concurrent("Should successfully finalize OpcodeTestContract.setGasLimit()", async () => {
    const account = await l2AccountManager.generateAccount();
    const lineaRollupV6 = config.getLineaRollupContract();

    const txReceipt = await setGasLimit(account);
    expect(txReceipt?.status).toEqual(1);
    // Ok to type assertion here, because txReceipt won't be null if it passed above assertion.
    const txBlockNumber = <number>txReceipt?.blockNumber;

    logger.debug(`Waiting for block to be finalized... blockNumber=${txBlockNumber}`);

    const isBlockFinalized = await pollForContractMethodReturnValueExceedTarget(
      lineaRollupV6.currentL2BlockNumber,
      BigInt(txBlockNumber),
    );

    logger.debug(`Block finalized. blockNumber=${txBlockNumber}`);

    expect(isBlockFinalized).toEqual(true);
  });

  // One-time test to test block gas limit increase from 61M -> 2B
  it.skip("Should successfully reach the target gas limit, and finalize the corresponding transaction", async () => {
    const targetBlockGasLimit = 2_000_000_000n;
    let isTargetBlockGasLimitReached = false;
    let blockNumberToCheckFinalization = 0;
    const account = await l2AccountManager.generateAccount();
    const lineaRollupV6 = config.getLineaRollupContract();

    logger.debug(`Target block gasLimit=${targetBlockGasLimit}`);

    while (!isTargetBlockGasLimitReached) {
      const txReceipt = await setGasLimit(account);
      expect(txReceipt?.status).toEqual(1);
      const blockGasLimit = await getGasLimit();

      if (blockGasLimit === targetBlockGasLimit) {
        isTargetBlockGasLimitReached = true;
        // Ok to type assertion here, because txReceipt won't be null if it passed above assertion.
        blockNumberToCheckFinalization = <number>txReceipt?.blockNumber;
      }
      await wait(1000);
    }

    logger.debug(`Waiting for block to be finalized... blockNumber=${blockNumberToCheckFinalization}`);

    const isBlockFinalized = await pollForContractMethodReturnValueExceedTarget(
      lineaRollupV6.currentL2BlockNumber,
      BigInt(blockNumberToCheckFinalization),
    );

    logger.debug(`Block finalized. blockNumber=${blockNumberToCheckFinalization}`);

    expect(isBlockFinalized).toEqual(true);
    // Timeout of 6 hrs
  }, 21_600_000);
});
