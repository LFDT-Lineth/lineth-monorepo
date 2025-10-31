import { Address, Hex } from "viem";
import { Flags } from "@oclif/core";
import { validateEthereumAddress, validateHexString } from "./validation.js";

export const address = Flags.custom<Address>({
  parse: async (input, _, opts) => validateEthereumAddress(input, opts.description),
});

export const hexString = Flags.custom<Hex>({
  parse: async (input) => validateHexString(input),
});
