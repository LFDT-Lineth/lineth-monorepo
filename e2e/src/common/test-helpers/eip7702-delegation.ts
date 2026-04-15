import {
  encodeFunctionData,
  getAddress,
  isAddress,
  type PrivateKeyAccount,
  type PublicClient,
  type WalletClient,
} from "viem";

import { TestEIP7702DelegationAbi, TestEIP7702DelegationAbiBytecode } from "../../generated";
import { estimateLineaGas, sendTransactionWithRetry } from "../utils";

export function encodeEip7702InitializeData(): `0x${string}` {
  return encodeFunctionData({
    abi: TestEIP7702DelegationAbi,
    functionName: "initialize",
  });
}

export async function deployTestEip7702Delegation(
  l2PublicClient: PublicClient,
  deployerWalletClient: WalletClient,
  deployer: PrivateKeyAccount,
): Promise<`0x${string}`> {
  const deployNonce = await l2PublicClient.getTransactionCount({ address: deployer.address });

  const deployEstimate = await estimateLineaGas(l2PublicClient, {
    account: deployer,
    data: TestEIP7702DelegationAbiBytecode,
  });

  const { receipt: deployReceipt } = await sendTransactionWithRetry(l2PublicClient, (fees) =>
    deployerWalletClient.sendTransaction({
      account: deployer,
      chain: deployerWalletClient.chain,
      data: TestEIP7702DelegationAbiBytecode,
      nonce: deployNonce,
      ...deployEstimate,
      ...fees,
    }),
  );

  if (deployReceipt.status !== "success") {
    throw new Error(`deployTestEip7702Delegation: deploy failed status=${deployReceipt.status}`);
  }
  if (!deployReceipt.contractAddress || !isAddress(deployReceipt.contractAddress)) {
    throw new Error("deployTestEip7702Delegation: missing contract address");
  }

  return getAddress(deployReceipt.contractAddress);
}
