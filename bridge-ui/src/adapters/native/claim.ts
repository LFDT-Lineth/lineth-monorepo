import { encodeFunctionData, zeroAddress } from "viem";

import { BridgeMessage, Chain, ChainLayer, NativeBridgeMessage } from "@/types";
import { isUndefined, isUndefinedOrEmptyString } from "@/utils/misc";

import { type DepositWarning } from "../types";
import { LINEA_ROLLUP_YIELD_EXTENSION_ABI, MESSAGE_SERVICE_ABI } from "./abis";
import { buildClaimWithProofParams, isNativeBridgeMessage } from "./message";

export function isValidClaimMessage(message: BridgeMessage, toChain: Chain): message is NativeBridgeMessage {
  return (
    isNativeBridgeMessage(message) &&
    !isUndefinedOrEmptyString(message.from) &&
    !isUndefinedOrEmptyString(message.to) &&
    !isUndefined(message.fee) &&
    !isUndefined(message.value) &&
    !isUndefined(message.nonce) &&
    message.nonce !== 0n &&
    !isUndefinedOrEmptyString(message.calldata) &&
    !isUndefinedOrEmptyString(message.messageHash) &&
    !(isUndefined(message.proof) && toChain.layer === ChainLayer.L1)
  );
}

export function encodeL1ClaimData(
  message: NativeBridgeMessage,
  toChain: Chain,
  options?: Record<string, unknown>,
): `0x${string}` | undefined {
  const claimProofParams = buildClaimWithProofParams(message);
  if (!claimProofParams) return undefined;

  if (options?.useAlternativeClaim && toChain.yieldProviderAddress) {
    return encodeFunctionData({
      abi: LINEA_ROLLUP_YIELD_EXTENSION_ABI,
      functionName: "claimMessageWithProofAndWithdrawLST",
      args: [claimProofParams, toChain.yieldProviderAddress],
    });
  }

  return encodeFunctionData({
    abi: MESSAGE_SERVICE_ABI,
    functionName: "claimMessageWithProof",
    args: [claimProofParams],
  });
}

export function encodeL2ClaimData(message: NativeBridgeMessage): `0x${string}` {
  return encodeFunctionData({
    abi: MESSAGE_SERVICE_ABI,
    functionName: "claimMessage",
    args: [
      message.from,
      message.to,
      message.fee,
      message.value,
      zeroAddress,
      message.calldata as `0x{string}`,
      message.nonce,
    ],
  });
}

export function buildStEthClaimMessages(isRecipient: boolean): DepositWarning[] {
  const messages: DepositWarning[] = [];

  if (isRecipient) {
    messages.push({
      text: "Low ETH liquidity. Claim as stETH now or wait until sufficient ETH balance becomes available.",
    });
    messages.push({
      text: "By claiming, you acknowledge that a liquidity buffer may apply. See",
      link: { url: "https://linea.build/terms-of-service", label: "Terms & Conditions." },
    });
  } else {
    messages.push({
      text: "Please connect the recipient wallet to claim stETH, or wait until sufficient ETH balance becomes available.",
    });
  }

  return messages;
}
