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
  } catch {
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

export function validateIsoDate(input: string, argName = "date"): Date {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(input)) {
    throw new Error(`${argName}, with value: ${input} must be a date in the yyyy-MM-dd format`);
  }
  const date = new Date(`${input}T00:00:00.000Z`);
  if (Number.isNaN(date.getTime())) {
    throw new Error(`${argName}, with value: ${input} is not a valid date`);
  }
  return date;
}
