import { getBalance } from "@wagmi/core";
import { Config } from "wagmi";

import { Chain, ChainLayer, Token } from "@/types";
import { isEth } from "@/utils/tokens";

export function isL2ToL1EthWithYieldProvider(token: Token, fromChain: Chain, toChain: Chain): boolean {
  return (
    fromChain.layer === ChainLayer.L2 &&
    toChain.layer === ChainLayer.L1 &&
    isEth(token) &&
    !!toChain.yieldProviderAddress
  );
}

export async function fetchMessageServiceBalance(wagmiConfig: Config, toChain: Chain): Promise<bigint> {
  const { value } = await getBalance(wagmiConfig, {
    address: toChain.messageServiceAddress,
    chainId: toChain.id,
  });
  return value;
}
