import { keccak256, encodeAbiParameters, Address } from "viem";
import { CCTPV2BridgeMessage, NativeBridgeMessage } from "./history";

const INBOX_L1L2_MESSAGE_STATUS_MAPPING_SLOT = 176n;

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

export function isNativeBridgeMessage(msg: NativeBridgeMessage | CCTPV2BridgeMessage): msg is NativeBridgeMessage {
  return (
    typeof (msg as NativeBridgeMessage).from === "string" &&
    typeof (msg as NativeBridgeMessage).to === "string" &&
    typeof (msg as NativeBridgeMessage).fee === "bigint" &&
    typeof (msg as NativeBridgeMessage).nonce === "bigint" &&
    typeof (msg as NativeBridgeMessage).calldata === "string" &&
    typeof (msg as NativeBridgeMessage).messageHash === "string"
  );
}

export function isCCTPV2BridgeMessage(msg: NativeBridgeMessage | CCTPV2BridgeMessage): msg is CCTPV2BridgeMessage {
  return (
    typeof (msg as CCTPV2BridgeMessage).message === "string" &&
    typeof (msg as CCTPV2BridgeMessage).attestation === "string"
  );
}
