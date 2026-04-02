import type { Hex } from "../types/primitives";

export interface ICalldataDecoder {
  /** Decode function calldata according to the given ABI signature string. */
  decode(calldataFunctionInterface: string, calldata: Hex): Record<string, unknown>;
}
