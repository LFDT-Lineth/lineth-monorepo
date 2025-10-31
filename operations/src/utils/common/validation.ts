import { isAddress, isHex } from "viem";

export function validateEthereumAddress(input: string, argName = "Ethereum address") {
  if (!isAddress(input)) {
    throw new Error(`${argName} is not a valid Ethereum address.`);
  }
  return input;
}

export function isValidProtocolUrl(input: string, allowedProtocols: string[]): boolean {
  try {
    const url = new URL(input);
    return allowedProtocols.includes(url.protocol);
  } catch (e) {
    return false;
  }
}

export function validateUrl(argName: string, input: string, allowedProtocols: string[]) {
  if (!isValidProtocolUrl(input, allowedProtocols)) {
    throw new Error(`${argName}, with value: ${input} is not a valid URL`);
  }
  return input;
}

export function validateHexString(input: string) {
  if (!isHex(input)) {
    throw new Error(`Input must be a hexadecimal string.`);
  }
  return input;
}
