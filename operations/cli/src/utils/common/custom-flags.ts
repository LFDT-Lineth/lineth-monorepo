import { Flags } from "@oclif/core";
import { Address, Hex } from "viem";

import { validateEthereumAddress, validateHexString, validateIsoDate } from "./validation.js";

export const address = Flags.custom<Address>({
  parse: async (input, _, opts) => validateEthereumAddress(input, opts.description),
});

export const hexString = Flags.custom<Hex>({
  parse: async (input) => validateHexString(input),
});

export const isoDate = Flags.custom<Date>({
  parse: async (input, _, opts) => validateIsoDate(input, opts.name),
});
