import { ClientFactory, PublicClientParams, WalletClientParams } from "./client-factory";
import { Config } from "../schema/config-schema";

export class L1Client {
  private readonly factory: ClientFactory;

  constructor(private readonly config: Config) {
    this.factory = new ClientFactory(this.config.L1.chainId, this.config.L1.rpcUrl);
  }

  public publicClient(params?: PublicClientParams) {
    return this.factory.getPublic(params);
  }

  public walletClient(params?: WalletClientParams) {
    return this.factory.getWallet(params);
  }
}
