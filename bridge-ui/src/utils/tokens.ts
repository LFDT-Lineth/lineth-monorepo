import { isAddress, isAddressEqual, zeroAddress } from "viem";
import { USDC_SYMBOL } from "@/constants";
import { BridgeProvider, Token } from "@/types";

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
    token.symbol === USDC_SYMBOL &&
    isAddress(token.L1) &&
    isAddress(token.L2)
  );
};
