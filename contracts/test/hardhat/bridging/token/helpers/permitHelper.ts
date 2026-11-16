import { type Address, type Hex, encodeFunctionData, parseSignature } from "viem";

import type { ContractReturnType, WalletClient } from "@nomicfoundation/hardhat-viem/types";

const Permit = [
  { name: "owner", type: "address" },
  { name: "spender", type: "address" },
  { name: "value", type: "uint256" },
  { name: "nonce", type: "uint256" },
  { name: "deadline", type: "uint256" },
] as const;

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
  const signature = await wallet.signTypedData({
    domain: { name, version: "1", chainId, verifyingContract: token.address },
    types: { Permit },
    primaryType: "Permit",
    message: { owner, spender, value: BigInt(value), nonce, deadline },
  });
  const { v, r, s, yParity } = parseSignature(signature);
  const permitCall = encodeFunctionData({
    abi: token.abi,
    functionName: "permit",
    args: [owner, spender, BigInt(value), deadline, v !== undefined ? Number(v) : yParity + 27, r, s],
  });
  return permitCall;
}
