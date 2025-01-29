import { ethers, AbiCoder } from "ethers";

export const encodeData = (types: string[], values: unknown[], packed?: boolean) => {
  if (packed) {
    return ethers.solidityPacked(types, values);
  }
  return AbiCoder.defaultAbiCoder().encode(types, values);
};

export function convertStringToPaddedHexBytes(strVal: string, paddedSize: number): string {
  if (strVal.length > paddedSize) {
    throw "Length is longer than padded size!";
  }

  const strBytes = ethers.toUtf8Bytes(strVal);
  const bytes = ethers.zeroPadBytes(strBytes, paddedSize);
  const bytes8Hex = ethers.hexlify(bytes);

  return bytes8Hex;
}
