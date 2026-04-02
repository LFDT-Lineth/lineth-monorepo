import { ILogger } from "@consensys/linea-shared-utils";
import { createPublicClient, createWalletClient, defineChain, http, PublicClient } from "viem";

import { ChainContext } from "./ChainContext";
import { contractSignerToViemAccount } from "./contractSignerToViemAccount";
import { createSignerClient } from "./createSignerClient";
import { SignerConfig } from "./SignerConfig";

const DEFAULT_CHAIN_ID_DEADLINE_MS = 60_000;

async function getChainId(client: PublicClient, logger: ILogger, deadlineMs: number): Promise<number> {
  const baseDelayMs = 1_000;
  const startTime = Date.now();
  let attempt = 0;
  let lastError: unknown;

  while (Date.now() - startTime < deadlineMs) {
    try {
      return await client.getChainId();
    } catch (error) {
      lastError = error;
      attempt++;
      const elapsed = Date.now() - startTime;
      if (elapsed >= deadlineMs) break;
      const delay = Math.min(baseDelayMs * 2 ** (attempt - 1), deadlineMs - elapsed);
      logger.warn(`Failed to fetch chainId, retrying in ${delay}ms (attempt ${attempt})`, { error });
      await new Promise((resolve) => setTimeout(resolve, delay));
    }
  }

  const elapsed = Date.now() - startTime;
  logger.error(`Failed to fetch chainId after ${elapsed}ms (${attempt} attempts)`, { error: lastError });
  throw lastError ?? new Error(`Failed to fetch chainId within ${deadlineMs}ms deadline`);
}

export type ChainContextOptions = {
  chainIdFetchDeadlineMs?: number;
};

export async function createChainContext(
  rpcUrl: string,
  signerConfig: SignerConfig,
  logger: ILogger,
  options?: ChainContextOptions,
): Promise<ChainContext> {
  const deadlineMs = options?.chainIdFetchDeadlineMs ?? DEFAULT_CHAIN_ID_DEADLINE_MS;

  const tempClient = createPublicClient({ transport: http(rpcUrl) });
  const chainId = await getChainId(tempClient, logger, deadlineMs);

  const chain = defineChain({
    id: chainId,
    name: "custom",
    nativeCurrency: { name: "Ether", symbol: "ETH", decimals: 18 },
    rpcUrls: { default: { http: [rpcUrl] } },
  });

  const publicClient = createPublicClient({ chain, transport: http(rpcUrl) });

  const signer = createSignerClient(signerConfig, logger, rpcUrl, chain);
  const account = contractSignerToViemAccount(signer);

  const walletClient = createWalletClient({
    account,
    transport: http(rpcUrl),
    chain,
  });

  return { chainId, chain, publicClient, walletClient, account, signer };
}
