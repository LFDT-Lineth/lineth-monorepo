import axios from "axios";
import { err, ok, Result } from "neverthrow";

export async function fetchEthereumPrice(
  coingeckoApiBaseUrl: string,
  coinGeckoApiKey: string,
): Promise<Result<{ ethereum: { usd: number } }, Error>> {
  try {
    const { data } = await axios.get(`${coingeckoApiBaseUrl}/simple/price?vs_currencies=usd&ids=ethereum`, {
      headers: buildCoinGeckoApiHeader(coingeckoApiBaseUrl, coinGeckoApiKey),
    });
    return ok(data);
  } catch (error) {
    return err(error as Error);
  }
}

export function buildCoinGeckoApiHeader(coingeckoApiBaseUrl: string, coinGeckoApiKey: string) {
  if (coingeckoApiBaseUrl.includes("pro-api")) {
    return {
      "x-cg-pro-api-key": coinGeckoApiKey,
    };
  }

  return {
    "x-cg-demo-api-key": coinGeckoApiKey,
  };
}
