import { cache } from "react";

import log from "loglevel";
import { Address } from "viem";

import { allAdapters } from "@/adapters";
import { config } from "@/config";
import { PRIORITY_SYMBOLS } from "@/constants/tokens";
import { type SupportedCurrencies } from "@/stores/configStore";
import { defaultTokensConfig } from "@/stores/tokenStore";
import { BridgeProvider, GithubTokenListToken, NetworkTokens, Token } from "@/types";
import { isUndefined } from "@/utils/misc";

enum NetworkTypes {
  MAINNET = "MAINNET",
  SEPOLIA = "SEPOLIA",
}

function resolveBridgeProvider(token: GithubTokenListToken): BridgeProvider {
  return allAdapters.find((adapter) => adapter.matchesToken(token))?.provider ?? BridgeProvider.NATIVE;
}

function isBridgeProviderEnabled(token: Token): boolean {
  const adapter = allAdapters.find((registeredAdapter) => registeredAdapter.provider === token.bridgeProvider);
  return adapter?.isEnabled() ?? false;
}

export async function getTokens(networkTypes: NetworkTypes): Promise<GithubTokenListToken[]> {
  try {
    let url = config.tokenListUrls.mainnet;
    if (networkTypes === NetworkTypes.SEPOLIA) {
      url = config.tokenListUrls.sepolia;
    }

    const response = await fetch(url, { next: { revalidate: 60 } });
    const data = await response.json();
    const tokens = data.tokens as GithubTokenListToken[];

    return tokens.filter((token: GithubTokenListToken) => {
      const hasRootAddress = !isUndefined(token.extension?.rootAddress);
      const isBridgeToken = token.tokenType.includes("canonical-bridge") || token.tokenType.includes("native");
      const isAdapterToken = allAdapters.some((adapter) => adapter.matchesToken(token));
      return hasRootAddress && (isBridgeToken || isAdapterToken);
    });
  } catch (error) {
    log.error("Error getTokens", { error });
    return [];
  }
}

export async function fetchTokenPrices(
  tokenAddresses: Address[],
  currency: SupportedCurrencies,
  chainId?: number,
): Promise<Record<string, number>> {
  if (isUndefined(chainId)) {
    return {};
  }

  const response = await fetch(
    `https://price.api.cx.metamask.io/v2/chains/${chainId}/spot-prices?tokenAddresses=${tokenAddresses.join(",")}&vsCurrency=${currency}`,
  );
  if (!response.ok) {
    throw new Error("Error in getTokenPrices");
  }

  const data: Record<string, Record<SupportedCurrencies, number>> = await response.json();

  return Object.fromEntries(Object.entries(data).map(([address, value]) => [address, value[currency]]));
}

export async function validateTokenURI(url: string): Promise<string> {
  try {
    await fetch(url, {
      next: { revalidate: 3600 },
    });
    return url;
  } catch {
    return `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/noTokenLogo.svg`;
  }
}

export async function formatToken(token: GithubTokenListToken): Promise<Token | undefined> {
  const bridgeProvider = resolveBridgeProvider(token);

  const logoURI = await validateTokenURI(token.logoURI);

  return {
    type: token.tokenType,
    name: token.name,
    symbol: token.symbol,
    decimals: token.decimals,
    L1: token.extension.rootAddress,
    L2: token.address,
    image: logoURI,
    isDefault: true,
    bridgeProvider,
  };
}

export const getTokenConfig = cache(async (): Promise<NetworkTokens> => {
  const updatedTokensConfig = { ...defaultTokensConfig };

  const sortPriorityTokensFirst = (tokens: Token[]): Token[] => {
    const priority: Token[] = [];
    const rest: Token[] = [];

    for (const token of tokens) {
      if (PRIORITY_SYMBOLS.includes(token.symbol)) {
        if (!priority.find((t) => t.symbol === token.symbol)) {
          priority.push(token);
        }
      } else {
        rest.push(token);
      }
    }

    return [...priority, ...rest];
  };

  const enrichTokens = async (tokens: GithubTokenListToken[], defaultList: Token[]): Promise<Token[]> => {
    const formatted = await Promise.all(tokens.map(formatToken));
    const nonNullableTokens = formatted.filter((token): token is Token => !isUndefined(token));
    const allTokens = [...defaultList, ...nonNullableTokens.filter(isBridgeProviderEnabled)];
    return sortPriorityTokensFirst(allTokens);
  };

  const [mainnetTokens, sepoliaTokens] = await Promise.all([
    getTokens(NetworkTypes.MAINNET),
    getTokens(NetworkTypes.SEPOLIA),
  ]);

  const [mainnet, sepolia] = await Promise.all([
    enrichTokens(mainnetTokens, defaultTokensConfig.MAINNET),
    enrichTokens(sepoliaTokens, defaultTokensConfig.SEPOLIA),
  ]);

  updatedTokensConfig.MAINNET = mainnet;
  updatedTokensConfig.SEPOLIA = sepolia;

  return updatedTokensConfig;
});
