import {
  AwsKmsSignerClientAdapter,
  IContractSignerClient,
  ILogger,
  ViemWalletSignerClientAdapter,
  Web3SignerClientAdapter,
} from "@consensys/linea-shared-utils";
import { type Chain } from "viem";

import { SignerConfig } from "./SignerConfig";

/**
 * Factory that creates an IContractSignerClient from a SignerConfig.
 * Dispatches to:
 *   - ViemWalletSignerClientAdapter (private-key)
 *   - Web3SignerClientAdapter (web3signer)
 *   - AwsKmsSignerClientAdapter (aws-kms, initialised asynchronously)
 */
export async function createSignerClient(
  config: SignerConfig,
  logger: ILogger,
  rpcUrl: string,
  chain: Chain,
): Promise<IContractSignerClient> {
  switch (config.type) {
    case "private-key":
      return new ViemWalletSignerClientAdapter(logger, rpcUrl, config.privateKey, chain);

    case "web3signer":
      return new Web3SignerClientAdapter(
        logger,
        config.endpoint,
        config.publicKey,
        config.tls?.keyStorePath ?? "",
        config.tls?.keyStorePassword ?? "",
        config.tls?.trustStorePath ?? "",
        config.tls?.trustStorePassword ?? "",
      );

    case "aws-kms":
      return AwsKmsSignerClientAdapter.create(
        logger,
        config.kmsKeyId,
        config.region ? { region: config.region } : undefined,
      );

    default: {
      const exhaustiveCheck: never = config;
      throw new Error(`Unsupported signer type: ${(exhaustiveCheck as { type: string }).type}`);
    }
  }
}
