import { toHex } from "viem";

export const DEFAULT_ADDRESS_FOR_NON_CONNECTED_USER = toHex("not connected", { size: 20 });
