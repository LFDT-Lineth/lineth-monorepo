import { IContractSignerClient } from "@consensys/linea-shared-utils";
import { Chain, LocalAccount, PublicClient, WalletClient } from "viem";

export type ChainContext = {
  chainId: number;
  chain: Chain;
  publicClient: PublicClient;
  walletClient: WalletClient;
  account: LocalAccount;
  signer: IContractSignerClient;
};
