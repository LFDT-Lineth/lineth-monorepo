import { describe, expect, it } from "@jest/globals";
import { encodeFunctionData, getAddress } from "viem";

import { estimateLineaGas, sendTransactionWithRetry } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { TestEIP7702DelegationAbi, TestEIP7702DelegationAbiBytecode } from "./generated";

const context = createTestContext();
const l2AccountManager = context.getL2AccountManager();

describe("EIP-7702 test suite", () => {
  const lineaEstimateGasClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });

  it.concurrent("should execute EIP-7702 (Set Code) transactions", async () => {
    const [deployer, eoa] = await l2AccountManager.generateAccounts(2);
    const l2PublicClient = context.l2PublicClient();

    // Deploy the TestEIP7702Delegation target contract
    const deployerWalletClient = context.l2WalletClient({ account: deployer });
    const deployNonce = await l2PublicClient.getTransactionCount({ address: deployer.address });

    const deployEstimate = await estimateLineaGas(lineaEstimateGasClient, {
      account: deployer,
      data: TestEIP7702DelegationAbiBytecode,
    });

    const { receipt: deployReceipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
      deployerWalletClient.sendTransaction({
        data: TestEIP7702DelegationAbiBytecode,
        nonce: deployNonce,
        ...deployEstimate,
        ...fees,
      }),
    );

    expect(deployReceipt.status).toEqual("success");
    expect(deployReceipt.contractAddress).toBeTruthy();
    const targetContractAddress = getAddress(deployReceipt.contractAddress!);

    logger.debug(`TestEIP7702Delegation deployed. address=${targetContractAddress}`);

    // Sign EIP-7702 authorization: EOA delegates to the target contract.
    const eoaWalletClient = context.l2WalletClient({ account: eoa });

    const authorization = await eoaWalletClient.signAuthorization({
      contractAddress: targetContractAddress,
      executor: "self",
    });

    logger.debug(`EIP-7702 authorization signed. eoaAddress=${eoa.address} target=${targetContractAddress}`);

    const initializeData = encodeFunctionData({
      abi: TestEIP7702DelegationAbi,
      functionName: "initialize",
    });

    const eoaNonce = await l2PublicClient.getTransactionCount({ address: eoa.address });

    const { maxFeePerGas, maxPriorityFeePerGas } = await estimateLineaGas(lineaEstimateGasClient, {
      account: deployer,
      to: targetContractAddress,
      data: initializeData,
    });

    const txHash = await eoaWalletClient.sendTransaction({
      authorizationList: [authorization],
      to: eoa.address,
      data: initializeData,
      nonce: eoaNonce,
      gas: 100_000n,
      maxFeePerGas,
      maxPriorityFeePerGas,
    });

    logger.debug(`EIP-7702 transaction sent. transactionHash=${txHash}`);

    const receipt = await l2PublicClient.waitForTransactionReceipt({ hash: txHash, timeout: 60_000 });
    logger.debug(`EIP-7702 transaction receipt received. transactionHash=${txHash} status=${receipt.status}`);

    expect(receipt.status).toEqual("success");
  });
});
