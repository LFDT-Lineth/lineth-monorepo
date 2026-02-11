import { createPublicClient, createWalletClient, http, HttpTransportConfig, PrivateKeyAccount } from "viem";

import { resolveChain } from "../chains/chain-registry";

type ClientParamsBase = {
  chainId?: number;
  rpcUrl?: URL;
  httpConfig?: HttpTransportConfig;
};

export type PublicClientParams = ClientParamsBase;

export type WalletClientParams = ClientParamsBase & {
  account: PrivateKeyAccount | undefined;
};

export class ClientFactory {
  constructor(
    private readonly chainId: number,
    private readonly rpcUrl: URL,
  ) {}

  getPublic(params?: PublicClientParams) {
    const { chainId = this.chainId, rpcUrl = this.rpcUrl } = params ?? {};
    return createPublicClient({
      chain: resolveChain(chainId),
      transport: http(rpcUrl.toString(), params?.httpConfig),
    });
  }

  getWallet(params?: WalletClientParams) {
    const { chainId = this.chainId, rpcUrl = this.rpcUrl, account } = params ?? {};
    return createWalletClient({
      chain: resolveChain(chainId),
      transport: http(rpcUrl.toString(), params?.httpConfig),
      ...(account ? { account } : {}),
    });
  }
}
