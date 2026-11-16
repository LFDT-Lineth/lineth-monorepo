import { type Hex, keccak256 } from "viem";

import { encodeData } from "./encoding.js";

export const generateKeccak256 = (types: string[], values: unknown[], opts: { encodePacked?: boolean }): Hex =>
  keccak256(encodeData(types, values, opts.encodePacked));
