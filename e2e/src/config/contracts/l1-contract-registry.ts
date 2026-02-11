import {
  getDummyContract,
  getLineaRollupContract,
  getLineaRollupProxyAdminContract,
  getTestERC20Contract,
  getTokenBridgeContract,
} from "./contracts";

import type { L1Config } from "../schema/config-schema";
import type { Client, Transport, Chain, Account } from "viem";

export function createL1ContractRegistry(cfg: L1Config) {
  return {
    lineaRollup: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getLineaRollupContract(client, cfg.lineaRollupAddress),

    lineaRollupProxyAdmin: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getLineaRollupProxyAdminContract(client, cfg.lineaRollupProxyAdminAddress),

    testERC20: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getTestERC20Contract(client, cfg.l1TokenAddress),

    tokenBridge: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getTokenBridgeContract(client, cfg.tokenBridgeAddress),

    dummyContract: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getDummyContract(client, cfg.dummyContractAddress),
  };
}

export type L1ContractRegistry = ReturnType<typeof createL1ContractRegistry>;
