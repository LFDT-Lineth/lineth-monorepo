import {
  IContractSignerClient,
  ILogger,
  ViemWalletSignerClientAdapter,
  Web3SignerClientAdapter,
} from "@consensys/linea-shared-utils";
import { type Chain } from "viem";

import { SignerConfig } from "./SignerConfig";

/**
 * Factory that creates an IContractSignerClient from a SignerConfig.
 * Dispatches to ViemWalletSignerClientAdapter (private-key) or
 * Web3SignerClientAdapter (web3signer) transparently.
 */
export function createSignerClient(
  config: SignerConfig,
  logger: ILogger,
  rpcUrl: string,
  chain: Chain,
): IContractSignerClient {
  if (config.type === "private-key") {
    return new ViemWalletSignerClientAdapter(logger, rpcUrl, config.privateKey, chain);
  }

  return new Web3SignerClientAdapter(
    logger,
    config.endpoint,
    config.publicKey,
    config.tls?.keyStorePath ?? "",
    config.tls?.keyStorePassword ?? "",
    config.tls?.trustStorePath ?? "",
    config.tls?.trustStorePassword ?? "",
  );
}
