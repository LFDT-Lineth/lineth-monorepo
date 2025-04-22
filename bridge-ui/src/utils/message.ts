import { keccak256, encodeAbiParameters, Address } from "viem";
import { CctpV2BridgeMessage, Chain, ChainLayer, NativeBridgeMessage, Token } from "@/types";
import { INBOX_L1L2_MESSAGE_STATUS_MAPPING_SLOT } from "@/constants";
import { isCctp } from "./tokens";

export function computeMessageHash(
  from: Address,
  to: Address,
  fee: bigint,
  value: bigint,
  nonce: bigint,
  calldata: `0x${string}` = "0x",
) {
  return keccak256(
    encodeAbiParameters(
      [
        { name: "from", type: "address" },
        { name: "to", type: "address" },
        { name: "fee", type: "uint256" },
        { name: "value", type: "uint256" },
        { name: "nonce", type: "uint256" },
        { name: "calldata", type: "bytes" },
      ],
      [from, to, fee, value, nonce, calldata],
    ),
  );
}

export function computeMessageStorageSlot(messageHash: `0x${string}`) {
  return keccak256(
    encodeAbiParameters(
      [
        { name: "messageHash", type: "bytes32" },
        { name: "mappingSlot", type: "uint256" },
      ],
      [messageHash, INBOX_L1L2_MESSAGE_STATUS_MAPPING_SLOT],
    ),
  );
}

export function isNativeBridgeMessage(msg: NativeBridgeMessage | CctpV2BridgeMessage): msg is NativeBridgeMessage {
  return (
    typeof (msg as NativeBridgeMessage).from === "string" &&
    typeof (msg as NativeBridgeMessage).to === "string" &&
    typeof (msg as NativeBridgeMessage).fee === "bigint" &&
    typeof (msg as NativeBridgeMessage).nonce === "bigint" &&
    typeof (msg as NativeBridgeMessage).calldata === "string" &&
    typeof (msg as NativeBridgeMessage).messageHash === "string"
  );
}

export function isCctpV2BridgeMessage(msg: NativeBridgeMessage | CctpV2BridgeMessage): msg is CctpV2BridgeMessage {
  return (
    typeof (msg as CctpV2BridgeMessage).nonce === "string" &&
    typeof (msg as CctpV2BridgeMessage).message === "string" &&
    typeof (msg as CctpV2BridgeMessage).attestation === "string"
  );
}

export type GetEstimatedTimeTextOptions = {
  withSpaceAroundHyphen: boolean;
  isAbbreviatedTimeUnit?: boolean;
};

export const getEstimatedTimeText = (fromChain: Chain, token: Token, opts: GetEstimatedTimeTextOptions) => {
  const { withSpaceAroundHyphen, isAbbreviatedTimeUnit } = opts;
  const spaceChar = withSpaceAroundHyphen ? " " : "";
  const hourUnit = isAbbreviatedTimeUnit ? "hrs" : "hour";
  const minuteUnit = isAbbreviatedTimeUnit ? "mins" : "minute";
  const secondUnit = isAbbreviatedTimeUnit ? "secs" : "second";

  if (isCctp(token)) {
    return `22 ${secondUnit}${spaceChar}-${spaceChar}19 ${minuteUnit}`;
  }
  if (fromChain.layer === ChainLayer.L1) {
    return `20 ${minuteUnit}`;
  }
  return `8${spaceChar}-${spaceChar}32 ${hourUnit}`;
};
