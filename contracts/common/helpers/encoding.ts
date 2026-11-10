import { randomBytes } from "node:crypto";
import { type Hex, bytesToHex, encodeAbiParameters, encodePacked, pad, parseAbiParameters, stringToBytes } from "viem";

/**
 * ABI-encodes `values` for the given Solidity `types`, or tightly packs them when `packed` is set.
 * Accepts ethers-style type strings, including `tuple(...)` (viem's parser expects a bare `(...)`).
 */
export const encodeData = (types: string[], values: unknown[], packed?: boolean): Hex => {
  if (packed) {
    return encodePacked(types, values);
  }
  return encodeAbiParameters(parseAbiParameters(types.map((type) => type.replace(/tuple\(/g, "(")).join(", ")), values);
};

/** UTF-8 encodes `strVal` and right-pads it with zero bytes to `paddedSize` bytes (ethers' `zeroPadBytes`). */
export function convertStringToPaddedHexBytes(strVal: string, paddedSize: number): Hex {
  if (strVal.length > paddedSize) {
    throw "Length is longer than padded size!";
  }
  return bytesToHex(pad(stringToBytes(strVal), { dir: "right", size: paddedSize }));
}

export function encodeSendMessage(
  sender: string,
  receiver: string,
  fee: bigint,
  amount: bigint,
  salt: bigint,
  calldata: string,
): Hex {
  return encodeData(
    ["address", "address", "uint256", "uint256", "uint256", "bytes"],
    [sender, receiver, fee, amount, salt, calldata],
  );
}

export const randomBytes32 = (): Hex => bytesToHex(randomBytes(32));
