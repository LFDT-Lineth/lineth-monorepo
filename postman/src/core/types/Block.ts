import type { Hash } from "./primitives";

export type Block = {
  number: number;
  timestamp: number;
  hash: Hash;
};
