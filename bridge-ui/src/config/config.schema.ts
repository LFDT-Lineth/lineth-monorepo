import { isAddress } from "viem";
import { z } from "zod";

const chainConfigSchema = z.object({
  iconPath: z.string(),
  messageServiceAddress: z.string().refine((val) => isAddress(val), {
    message: "Invalid Ethereum address",
  }),
  tokenBridgeAddress: z.string().refine((val) => isAddress(val), {
    message: "Invalid Ethereum address",
  }),
  gasLimitSurplus: z.bigint().positive(),
  profitMargin: z.bigint().positive(),
  cctpDomain: z.number().gte(0).int(),
});

export const configSchema = z
  .object({
    chains: z.record(z.string().regex(/^\d+$/), chainConfigSchema),
    walletConnectId: z.string(),
    storage: z.object({
      minVersion: z.number().positive().int(),
    }),
    // Feature toggle for CCTPV2 for USDC transfers
    isCCTPEnabled: z.boolean(),
  })
  .strict();

export type Config = z.infer<typeof configSchema>;
