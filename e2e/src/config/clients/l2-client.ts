import { ClientFactory, PublicClientParams, WalletClientParams } from "./client-factory";
import { L2Config, LocalL2Config } from "../schema/config-schema";
import {
  createBesuNodeExtension,
  createSequencerExtension,
  createShomeiExtension,
  createShomeiFrontendExtension,
  createTransactionExclusionExtension,
  type BesuNodeActions,
  type SequencerActions,
  type ShomeiActions,
  type ShomeiFrontendActions,
  type TransactionExclusionActions,
} from "./linea-rpc/extensions";

export enum L2RpcEndpoint {
  Default = "default",
  Sequencer = "sequencer",
  BesuNode = "besuNode",
  BesuFollower = "besuFollower",
  Shomei = "shomei",
  ShomeiFrontend = "shomeiFrontend",
  TransactionExclusion = "transactionExclusion",
}

export type EndpointActions = {
  [L2RpcEndpoint.BesuNode]: BesuNodeActions;
  [L2RpcEndpoint.Sequencer]: SequencerActions;
  [L2RpcEndpoint.Shomei]: ShomeiActions;
  [L2RpcEndpoint.ShomeiFrontend]: ShomeiFrontendActions;
  [L2RpcEndpoint.TransactionExclusion]: TransactionExclusionActions;
  [L2RpcEndpoint.Default]: object;
  [L2RpcEndpoint.BesuFollower]: object;
};

function createBaseL2PublicClient(
  factory: ClientFactory,
  url: URL,
  chainId: number,
  httpConfig?: PublicClientParams["httpConfig"],
) {
  return factory.getPublic({
    chainId,
    rpcUrl: url,
    ...(httpConfig ? { httpConfig } : {}),
  });
}

export type L2BasePublicClient = ReturnType<typeof createBaseL2PublicClient>;

export class L2Client {
  private readonly factory: ClientFactory;

  constructor(
    private readonly config: L2Config,
    private readonly local?: LocalL2Config,
  ) {
    this.factory = new ClientFactory(config.chainId, config.rpcUrl);
  }

  public publicClient<T extends L2RpcEndpoint = L2RpcEndpoint.Default>(params?: {
    type?: T;
    httpConfig?: PublicClientParams["httpConfig"];
  }): L2BasePublicClient & EndpointActions[T] {
    return this.createPublicClient(params?.type ?? L2RpcEndpoint.Default, params?.httpConfig) as L2BasePublicClient &
      EndpointActions[T];
  }

  private createPublicClient(type: L2RpcEndpoint, httpConfig?: PublicClientParams["httpConfig"]) {
    const url = this.resolveRpcUrl(type);
    const base = createBaseL2PublicClient(this.factory, url, this.config.chainId, httpConfig);

    switch (type) {
      case L2RpcEndpoint.BesuNode:
        return base.extend(createBesuNodeExtension());
      case L2RpcEndpoint.Sequencer:
        return base.extend(createSequencerExtension());
      case L2RpcEndpoint.Shomei:
        return base.extend(createShomeiExtension());
      case L2RpcEndpoint.ShomeiFrontend:
        return base.extend(createShomeiFrontendExtension());
      case L2RpcEndpoint.TransactionExclusion:
        return base.extend(createTransactionExclusionExtension());
      default:
        return base;
    }
  }

  public walletClient(params?: {
    type?: L2RpcEndpoint;
    account: WalletClientParams["account"];
    httpConfig?: WalletClientParams["httpConfig"];
  }) {
    const { type = L2RpcEndpoint.Default, account } = params ?? {};
    const url = this.resolveRpcUrl(type);
    return this.factory.getWallet({
      chainId: this.config.chainId,
      rpcUrl: url,
      account,
      ...(params?.httpConfig ? { httpConfig: params.httpConfig } : {}),
    });
  }

  private resolveRpcUrl(type: L2RpcEndpoint): URL {
    if (type === L2RpcEndpoint.Default) {
      return this.config.rpcUrl;
    }

    const endpointMap: Record<string, URL | undefined> = {
      [L2RpcEndpoint.Sequencer]: this.local?.sequencerEndpoint ?? this.config.sequencerEndpoint,
      [L2RpcEndpoint.BesuNode]: this.local?.besuNodeRpcUrl ?? this.config.besuNodeRpcUrl,
      [L2RpcEndpoint.BesuFollower]: this.local?.besuFollowerNodeRpcUrl ?? this.config.besuFollowerNodeRpcUrl,
      [L2RpcEndpoint.Shomei]: this.local?.shomeiEndpoint ?? this.config.shomeiEndpoint,
      [L2RpcEndpoint.ShomeiFrontend]: this.local?.shomeiFrontendEndpoint ?? this.config.shomeiFrontendEndpoint,
      [L2RpcEndpoint.TransactionExclusion]:
        this.local?.transactionExclusionEndpoint ?? this.config.transactionExclusionEndpoint,
    };

    const url = endpointMap[type];
    if (!url) {
      throw new Error(`RPC endpoint "${type}" not configured for this environment.`);
    }
    return url;
  }
}
