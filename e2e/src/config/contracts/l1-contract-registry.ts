import {
  getDummyContract,
  getForcedTransactionGatewayContract,
  getLinethRollupContract,
  getLinethRollupProxyAdminContract,
  getTestERC20Contract,
  getTokenBridgeContract,
} from "./contracts";

import type { L1Config } from "../schema/config-schema";
import type { Client, Transport, Chain, Account } from "viem";

export function createL1ContractRegistry(cfg: L1Config) {
  return {
    linethRollup: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getLinethRollupContract(client, cfg.linethRollupAddress),

    linethRollupProxyAdmin: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getLinethRollupProxyAdminContract(client, cfg.linethRollupProxyAdminAddress),

    testERC20: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getTestERC20Contract(client, cfg.l1TokenAddress),

    tokenBridge: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getTokenBridgeContract(client, cfg.tokenBridgeAddress),

    dummyContract: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getDummyContract(client, cfg.dummyContractAddress),

    forcedTransactionGateway: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getForcedTransactionGatewayContract(client, cfg.forcedTransactionGatewayAddress),
  };
}

export type L1ContractRegistry = ReturnType<typeof createL1ContractRegistry>;
