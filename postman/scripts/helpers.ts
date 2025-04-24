import { ethers } from "ethers";

export function encodeSendMessage(
  sender: string,
  receiver: string,
  fee: bigint,
  amount: bigint,
  messageNonce: bigint,
  calldata: string,
) {
  const abiCoder = new ethers.AbiCoder();
  const data = abiCoder.encode(
    ["address", "address", "uint256", "uint256", "uint256", "bytes"],
    [sender, receiver, fee, amount, messageNonce, calldata],
  );

  return ethers.keccak256(data);
}
