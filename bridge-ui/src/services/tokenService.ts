import log from "loglevel";
import { Address } from "viem";
import { config } from "@/config";
import { SupportedCurrencies, defaultTokensConfig } from "@/stores";
import { GithubTokenListToken, Token, BridgeProvider, NetworkTokens } from "@/types";
import { PRIORITY_SYMBOLS, USDC_SYMBOL } from "@/constants";
import { isUndefined } from "@/utils";

enum NetworkTypes {
  MAINNET = "MAINNET",
  SEPOLIA = "SEPOLIA",
}

export async function getTokens(networkTypes: NetworkTypes): Promise<GithubTokenListToken[]> {
  try {
    // Fetch the JSON data from the URL.
    let url = config.tokenListUrls.mainnet;
    if (networkTypes === NetworkTypes.SEPOLIA) {
      url = config.tokenListUrls.sepolia;
    }

    const response = await fetch(url, { next: { revalidate: 60 } });
    const data = await response.json();
    const tokens = data.tokens as GithubTokenListToken[];
    const bridgedTokens = tokens.filter(
      (token: GithubTokenListToken) =>
        token.tokenType.includes("canonical-bridge") ||
        (token.tokenType.includes("native") && token.extension?.rootAddress !== undefined) ||
        token.symbol === USDC_SYMBOL,
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
    await fetch(url);
    return url;
  } catch (error) {
    return `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/noTokenLogo.svg`;
  }
}

export async function formatToken(token: GithubTokenListToken): Promise<Token> {
  const bridgeProvider = token.symbol === USDC_SYMBOL ? BridgeProvider.CCTP : BridgeProvider.NATIVE;

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

export async function getTokenConfig(): Promise<NetworkTokens> {
  if (config.e2eTestMode) {
    return {
      MAINNET: [
        ...defaultTokensConfig.MAINNET,
        {
          type: ["canonical-bridge"],
          name: "TestERC20",
          symbol: "TERC20",
          decimals: 18,
          L1: "0x8A791620dd6260079BF849Dc5567aDC3F2FdC318",
          L2: "0xCC1B08B17301e090cbb4c1F5598Cbaa096d591FB",
          image: "",
          isDefault: true,
          bridgeProvider: BridgeProvider.NATIVE,
        },
      ],
      SEPOLIA: [
        ...defaultTokensConfig.SEPOLIA,
        {
          type: ["canonical-bridge"],
          name: "TestERC20",
          symbol: "TERC20",
          decimals: 18,
          L1: "0x8A791620dd6260079BF849Dc5567aDC3F2FdC318",
          L2: "0xCC1B08B17301e090cbb4c1F5598Cbaa096d591FB",
          image: "",
          isDefault: true,
          bridgeProvider: BridgeProvider.NATIVE,
        },
      ],
    };
  }
  const updatedTokensConfig = { ...defaultTokensConfig };

  // Feature toggle, remove when feature toggle no longer needed
  const filterOutUSDCWhenCctpNotEnabled = (token: Token) => config.isCctpEnabled || token.symbol !== USDC_SYMBOL;

  // Sort the tokens to put priority tokens first
  const sortPriorityTokensFirst = (tokens: Token[]): Token[] => {
    const priority: Token[] = [];
    const rest: Token[] = [];

    for (const token of tokens) {
      if (PRIORITY_SYMBOLS.includes(token.symbol)) {
        // Avoid duplicates
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
    // Feature toggle, remove .filter expression when feature toggle no longer needed
    const allTokens = [...defaultList, ...formatted.filter(filterOutUSDCWhenCctpNotEnabled)];
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
}
