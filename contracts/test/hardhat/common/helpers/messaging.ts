import { ethers } from "ethers";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { encodeData } from "./encoding";
import { MESSAGE_FEE, MESSAGE_VALUE_1ETH, DEFAULT_MESSAGE_NONCE } from "../constants";

export async function encodeSendMessageLog(
  sender: SignerWithAddress,
  receiver: SignerWithAddress,
  messageHash: string,
  calldata: string,
) {
  const topic = ethers.id("MessageSent(address,address,uint256,uint256,uint256,bytes,bytes32)");
  const data = encodeData(
    ["address", "address", "uint256", "uint256", "uint256", "bytes", "bytes32"],
    [sender.address, receiver.address, MESSAGE_FEE, MESSAGE_VALUE_1ETH, DEFAULT_MESSAGE_NONCE, calldata, messageHash],
  );

  return {
    topic,
    data,
  };
}

export async function encodeSendMessage(
  sender: string,
  receiver: string,
  fee: bigint,
  amount: bigint,
  salt: bigint,
  calldata: string,
) {
  return encodeData(
    ["address", "address", "uint256", "uint256", "uint256", "bytes"],
    [sender, receiver, fee, amount, salt, calldata],
  );
}
