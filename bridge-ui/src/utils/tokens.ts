import { BridgeProvider, Token } from "@/types";
import { isAddress, isAddressEqual, zeroAddress } from "viem";

export const isEth = (token: Token) => {
  return (
    isAddress(token.L1) &&
    isAddress(token.L2) &&
    isAddressEqual(token.L1, zeroAddress) &&
    isAddressEqual(token.L2, zeroAddress)
  );
};

export const isCctp = (token: Token) => {
  return (
    !isEth(token) &&
    token.bridgeProvider === BridgeProvider.CCTP &&
    token.symbol === "USDC" &&
    isAddress(token.L1) &&
    isAddress(token.L2)
  );
};
