import { type Address, type Hex, encodeFunctionData, parseSignature } from "viem";

import type { ContractReturnType, WalletClient } from "@nomicfoundation/hardhat-viem/types";

const Permit = [
  { name: "owner", type: "address" },
  { name: "spender", type: "address" },
  { name: "value", type: "uint256" },
  { name: "nonce", type: "uint256" },
  { name: "deadline", type: "uint256" },
] as const;

function buildData(
  owner: Address,
  name: string,
  version: string,
  chainId: number,
  verifyingContract: Address,
  spender: Address,
  value: bigint,
  nonce: bigint,
  deadline: bigint,
) {
  return {
    domain: { name, version, chainId, verifyingContract },
    types: { Permit },
    value: { owner, spender, value, nonce, deadline },
  };
}

export async function getPermitData(
  wallet: WalletClient,
  token: ContractReturnType<"BridgedToken">,
  nonce: bigint,
  chainId: number,
  spender: Address,
  value: number | bigint,
  deadline: bigint,
): Promise<Hex> {
  const owner = wallet.account.address;
  const name = await token.read.name();
  const data = buildData(owner, name, "1", chainId, token.address, spender, BigInt(value), nonce, deadline);
  const signature = await wallet.signTypedData({
    domain: data.domain,
    types: data.types,
    primaryType: "Permit",
    message: data.value,
  });
  const { v, r, s, yParity } = parseSignature(signature);
  const permitCall = encodeFunctionData({
    abi: token.abi,
    functionName: "permit",
    args: [owner, spender, BigInt(value), deadline, v !== undefined ? Number(v) : yParity + 27, r, s],
  });
  return permitCall;
}
