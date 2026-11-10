import { type Hex, keccak256 } from "viem";

import { encodeData } from "./encoding.js";

export const generateKeccak256 = (types: string[], values: unknown[], opts: { encodePacked?: boolean }): Hex =>
  keccak256(encodeData(types, values, opts.encodePacked));

export const generateKeccak256ForString = (value: string) =>
  generateKeccak256(["string"], [value], { encodePacked: true });

export const generateFunctionSelector = (functionSignature: string) =>
  generateKeccak256ForString(functionSignature).slice(2, 10);
