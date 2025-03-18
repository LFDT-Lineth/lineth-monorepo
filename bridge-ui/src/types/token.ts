import { Address } from "viem";
import { BridgeProvider } from "./providers";

export interface GithubTokenListTokenExtension {
  rootChainId: number;
  rootChainURI: string;
  rootAddress: Address;
}

export type TokenType = "eth" | "canonical-bridge" | "native" | "external-bridge" | "bridge-reserved";

export interface GithubTokenListToken {
  chainId: number;
  chainURI: string;
  tokenId: string;
  tokenType: TokenType[];
  address: Address;
  name: string;
  symbol: string;
  decimals: number;
  createdAt: string;
  updatedAt: string;
  logoURI: string;
  extension: GithubTokenListTokenExtension;
}

export type Token = {
  type: TokenType[];
  name: string;
  symbol: string;
  decimals: number;
  L1: Address;
  L2: Address;
  image: string;
  isDefault: boolean;
  bridgeProvider: BridgeProvider;
};

export type NetworkTokens = {
  MAINNET: Token[];
  SEPOLIA: Token[];
};
