import log from "loglevel";
import { Address } from "viem";
import { config } from "@/config";
import { SupportedCurrencies, defaultTokensConfig } from "@/stores";
import { GithubTokenListToken, Token, BridgeProvider, NetworkTokens } from "@/types";

enum NetworkTypes {
  MAINNET = "MAINNET",
  SEPOLIA = "SEPOLIA",
}

export async function getTokens(networkTypes: NetworkTypes): Promise<GithubTokenListToken[]> {
  try {
    // Fetch the JSON data from the URL.
    let url = process.env.MAINNET_TOKEN_LIST ? (process.env.MAINNET_TOKEN_LIST as string) : "";
    if (networkTypes === NetworkTypes.SEPOLIA) {
      url = process.env.SEPOLIA_TOKEN_LIST ? (process.env.SEPOLIA_TOKEN_LIST as string) : "";
    }

    const response = await fetch(url);
    const data = await response.json();
    const tokens = data.tokens as GithubTokenListToken[];
    const bridgedTokens = tokens.filter(
      (token: GithubTokenListToken) =>
        token.tokenType.includes("canonical-bridge") || token.tokenType.includes("native") || token.symbol === "USDC",
    );
    return bridgedTokens;
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
  if (!chainId) {
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
    await fetch(url);
    return url;
  } catch (error) {
    return "/images/logo/noTokenLogo.svg";
  }
}

export async function formatToken(token: GithubTokenListToken): Promise<Token> {
  const bridgeProvider = token.symbol === "USDC" ? BridgeProvider.CCTP : BridgeProvider.NATIVE;

  const logoURI = await validateTokenURI(token.logoURI);

  return {
    type: token.tokenType,
    name: token.name,
    symbol: token.symbol,
    decimals: token.decimals,
    L1: token?.extension?.rootAddress ?? null,
    L2: token.address,
    image: logoURI,
    isDefault: true,
    bridgeProvider,
  };
}

export async function getTokenConfig(): Promise<NetworkTokens> {
  const [mainnetTokens, sepoliaTokens] = await Promise.all([
    getTokens(NetworkTypes.MAINNET),
    getTokens(NetworkTypes.SEPOLIA),
  ]);

  const updatedTokensConfig = { ...defaultTokensConfig };

  // Feature toggle, remove when feature toggle no longer needed
  const filterOutUSDCWhenCCTPNotEnabled = (token: Token) => config.isCCTPEnabled || token.symbol !== "USDC";

  updatedTokensConfig.MAINNET = [
    ...defaultTokensConfig.MAINNET,
    ...(await Promise.all(mainnetTokens.map(async (token: GithubTokenListToken): Promise<Token> => formatToken(token))))
      // Feature toggle, remove .filter expression when feature toggle no longer needed
      .filter(filterOutUSDCWhenCCTPNotEnabled),
  ];

  updatedTokensConfig.SEPOLIA = [
    ...defaultTokensConfig.SEPOLIA,
    ...(await Promise.all(sepoliaTokens.map((token: GithubTokenListToken): Promise<Token> => formatToken(token))))
      // Feature toggle, remove .filter expression when feature toggle no longer needed
      .filter(filterOutUSDCWhenCCTPNotEnabled),
  ];

  return updatedTokensConfig;
}
