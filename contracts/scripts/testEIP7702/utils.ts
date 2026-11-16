import { type Address, type Hex, type PublicClient, formatEther, getAddress } from "viem";
import { estimateGas as lineaEstimateGas } from "viem/linea";

import { SupportedChainIds } from "../../common/supportedNetworks.js";

const BASE_FEE_MULTIPLIER_PERCENT = 135n;
const PRIORITY_FEE_MULTIPLIER_PERCENT = 105n;

function isLineaChainId(chainId: number): boolean {
  const lineaChainIds = new Set<number>([
    SupportedChainIds.LINEA_DEVNET,
    SupportedChainIds.LINEA_SEPOLIA,
    SupportedChainIds.LINEA_TESTNET,
    SupportedChainIds.LINEA,
  ]);
  return lineaChainIds.has(chainId);
}

export async function checkDelegation(
  client: PublicClient,
  address: Address,
): Promise<{ isDelegated: boolean; implementationAddress?: Address }> {
  const code = (await client.getCode({ address })) ?? "0x";

  if (code === "0x") {
    console.log(`No delegation found for ${address}`);
    return { isDelegated: false };
  }

  // EIP-7702 delegation code starts with 0xef0100
  if (code.startsWith("0xef0100")) {
    const delegatedAddress = getAddress(`0x${code.slice(8)}`);
    console.log(`Delegation found for ${address} -> ${delegatedAddress}`);
    return { isDelegated: true, implementationAddress: delegatedAddress };
  }

  console.log(`Address has code but not EIP-7702 delegation: ${code}`);
  return { isDelegated: false };
}

export async function getAccountInfo(
  client: PublicClient,
  address: Address,
): Promise<{ address: Address; balance: string; nonce: number }> {
  const balance = await client.getBalance({ address });
  const nonce = await client.getTransactionCount({ address });
  return { address, balance: formatEther(balance), nonce };
}

// Estimates gas fees, branching on whether the chain is Linea (linea_estimateGas) or not (EIP-1559 fee history).
export async function estimateGasFees(
  client: PublicClient,
  from: Address,
  to?: Address,
  data: Hex = "0x",
): Promise<{ maxFeePerGas: bigint; maxPriorityFeePerGas: bigint }> {
  const chainId = await client.getChainId();
  if (isLineaChainId(chainId)) {
    const { baseFeePerGas, priorityFeePerGas } = await lineaEstimateGas(client, { account: from, to, data, value: 0n });
    const maxPriorityFeePerGas = (priorityFeePerGas * PRIORITY_FEE_MULTIPLIER_PERCENT) / 100n;
    return {
      maxFeePerGas: (baseFeePerGas * BASE_FEE_MULTIPLIER_PERCENT) / 100n + maxPriorityFeePerGas,
      maxPriorityFeePerGas,
    };
  }
  const { maxFeePerGas, maxPriorityFeePerGas } = await client.estimateFeesPerGas();
  return { maxFeePerGas, maxPriorityFeePerGas };
}
