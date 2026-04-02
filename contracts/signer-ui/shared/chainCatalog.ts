import chainCatalogJson from "./chainCatalog.json" with { type: "json" };

export type SignerUiChainCatalogEntry = {
  chainId: number;
  chainName: string;
  rpcUrls: string[];
  blockExplorerUrls: string[];
  nativeCurrency: {
    name: string;
    symbol: string;
    decimals: number;
  };
};

export const SIGNER_UI_CHAIN_CATALOG = chainCatalogJson as readonly SignerUiChainCatalogEntry[];

const catalogByChainId = new Map<number, SignerUiChainCatalogEntry>(
  SIGNER_UI_CHAIN_CATALOG.map((entry) => [entry.chainId, entry]),
);

export function getSignerUiChainCatalogEntry(chainId: number): SignerUiChainCatalogEntry | undefined {
  return catalogByChainId.get(chainId);
}
