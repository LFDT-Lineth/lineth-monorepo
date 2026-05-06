import { Hex } from "../types/misc";

export function slice(hex: Hex, start: number, end: number): Hex {
  return ("0x" + hex.slice(2 + start * 2, 2 + end * 2)) as Hex;
}

export function hexToNumber(hex: Hex): number {
  return parseInt(hex, 16);
}
