import { AccountManager } from "../accounts/account-manager";
import { PublicClientParams, WalletClientParams } from "../clients/client-factory";
import { L1Client } from "../clients/l1-client";
import { L2Client, L2RpcEndpoint, type L2BasePublicClient, type EndpointActions } from "../clients/l2-client";
import { createL1ContractRegistry, type L1ContractRegistry } from "../contracts/l1-contract-registry";
import { createL2ContractRegistry, type L2ContractRegistry } from "../contracts/l2-contract-registry";
import { Config, LocalL2Config, L2Config } from "../schema/config-schema";

import type { TestEnv } from "./test-setup-factory";

function isLocalL2Config(config: L2Config): config is LocalL2Config {
  return (
    config.besuNodeRpcUrl !== undefined &&
    config.shomeiEndpoint !== undefined &&
    config.shomeiFrontendEndpoint !== undefined &&
    config.sequencerEndpoint !== undefined &&
    config.transactionExclusionEndpoint !== undefined
  );
}

export default class TestSetupCore {
  protected L1: {
    client: L1Client;
  };

  protected L2: {
    client: L2Client;
  };

  public readonly l1Contracts: L1ContractRegistry;
  public readonly l2Contracts: L2ContractRegistry;

  constructor(
    protected readonly config: Config,
    private readonly env: TestEnv,
  ) {
    const l1Client = new L1Client(config);
    const localL2Config = isLocalL2Config(config.L2) ? config.L2 : undefined;
    const l2Client = new L2Client(config.L2, localL2Config);

    this.L1 = {
      client: l1Client,
    };

    this.L2 = {
      client: l2Client,
    };

    this.l1Contracts = createL1ContractRegistry(config.L1);
    this.l2Contracts = createL2ContractRegistry(config.L2);
  }

  public getL1AccountManager(): AccountManager {
    return this.config.L1.accountManager;
  }

  public getL2AccountManager(): AccountManager {
    return this.config.L2.accountManager;
  }

  public getL2ChainId(): number {
    return this.config.L2.chainId;
  }

  public l1PublicClient(params?: PublicClientParams) {
    return this.L1.client.publicClient(params);
  }

  public l1WalletClient(params?: WalletClientParams) {
    return this.L1.client.walletClient(params);
  }

  public l2PublicClient<T extends L2RpcEndpoint = L2RpcEndpoint.Default>(params?: {
    type?: T;
    httpConfig?: PublicClientParams["httpConfig"];
  }): L2BasePublicClient & EndpointActions[T] {
    return this.L2.client.publicClient(params);
  }

  public l2WalletClient(params?: {
    type?: L2RpcEndpoint;
    account: WalletClientParams["account"];
    httpConfig?: WalletClientParams["httpConfig"];
  }) {
    return this.L2.client.walletClient(params);
  }

  isLocal(): boolean {
    return this.env === "local";
  }
}
