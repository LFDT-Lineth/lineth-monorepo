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
  cctpTokenMessengerV2Address: z.string().refine((val) => isAddress(val), {
    message: "Invalid Ethereum address",
  }),
  cctpMessageTransmitterV2Address: z.string().refine((val) => isAddress(val), {
    message: "Invalid Ethereum address",
  }),
});

export const configSchema = z
  .object({
    chains: z.record(z.string().regex(/^\d+$/), chainConfigSchema),
    e2eTestMode: z.boolean().default(false),
    walletConnectId: z.string().nonempty(),
    storage: z.object({
      minVersion: z.number().positive().int(),
    }),
    // Feature toggle for CCTPV2 for USDC transfers
    isCctpEnabled: z.boolean(),
    infuraApiKey: z.string().nonempty(),
    alchemyApiKey: z.string().nonempty(),
    quickNodeApiKey: z.string().nonempty(),
    web3AuthClientId: z.string().nonempty(),
    lifiApiKey: z.string().nonempty(),
    lifiIntegrator: z.string().nonempty(),
    onRamperApiKey: z.string().nonempty(),
    layerswapApiKey: z.string().nonempty(),
    tokenListUrls: z.object({
      mainnet: z.string().trim().url(),
      sepolia: z.string().trim().url(),
    }),
  })
  .strict();

export type Config = z.infer<typeof configSchema>;
