import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { ethers } from "ethers";
import { BridgedToken } from "../../../../../typechain-types";

const Permit = [
  { name: "owner", type: "address" },
  { name: "spender", type: "address" },
  { name: "value", type: "uint256" },
  { name: "nonce", type: "uint256" },
  { name: "deadline", type: "uint256" },
];

function buildData(
  owner: string,
  name: string,
  version: string,
  chainId: number,
  verifyingContract: string,
  spender: string,
  value: number,
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
  wallet: SignerWithAddress,
  token: BridgedToken,
  nonce: bigint,
  chainId: number,
  spender: string,
  value: number,
  deadline: bigint,
) {
  const name = await token.name();
  const data = buildData(wallet.address, name, "1", chainId, await token.getAddress(), spender, value, nonce, deadline);
  const signature = await wallet.signTypedData(data.domain, data.types, data.value);
  const { v, r, s } = ethers.Signature.from(signature);
  const permitCall = token.interface.encodeFunctionData("permit", [wallet.address, spender, value, deadline, v, r, s]);
  return permitCall;
}
