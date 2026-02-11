import {
  getDummyContract,
  getL2MessageServiceContract,
  getLineaSequencerUpTimeFeedContract,
  getOpcodeTesterContract,
  getSparseMerkleProofContract,
  getTestContract,
  getTestERC20Contract,
  getTokenBridgeContract,
} from "./contracts";

import type { L2Config } from "../schema/config-schema";
import type { Client, Transport, Chain, Account } from "viem";

export function createL2ContractRegistry(cfg: L2Config) {
  return {
    dummyContract: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getDummyContract(client, cfg.dummyContractAddress),

    testERC20: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getTestERC20Contract(client, cfg.l2TokenAddress),

    l2MessageService: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getL2MessageServiceContract(client, cfg.l2MessageServiceAddress),

    tokenBridge: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getTokenBridgeContract(client, cfg.tokenBridgeAddress),

    testContract: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getTestContract(client, cfg.l2TestContractAddress ?? "0x"),

    sparseMerkleProof: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getSparseMerkleProofContract(client, cfg.l2SparseMerkleProofAddress),

    lineaSequencerUptimeFeed: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getLineaSequencerUpTimeFeedContract(client, cfg.l2LineaSequencerUptimeFeedAddress),

    opcodeTester: <T extends Transport, C extends Chain | undefined, A extends Account | undefined>(
      client: Client<T, C, A>,
    ) => getOpcodeTesterContract(client, cfg.opcodeTesterAddress),
  };
}

export type L2ContractRegistry = ReturnType<typeof createL2ContractRegistry>;
