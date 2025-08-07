import { ParserOutput } from "@oclif/core/interfaces";

export type Config = {
  senderAddress: string;
  destinationAddress: string;
  threshold: string;
  blockchainRpcUrl: string;
  web3SignerUrl: string;
  web3SignerPublicKey: string;
  maxFeePerGas: bigint;
  gasEstimationPercentile: number;
  dryRun: boolean;
  tls: boolean;
  web3SignerKeystorePath: string;
  web3SignerPassphrase: string;
  web3SignerTrustedStorePath: string;
};

export function validateConfig(flags: ParserOutput["flags"]): Config {
  const {
    senderAddress,
    destinationAddress,
    threshold,
    blockchainRpcUrl,
    web3SignerUrl,
    web3SignerPublicKey,
    dryRun,
    maxFeePerGas: maxFeePerGasArg,
    gasEstimationPercentile,
    tls,
    web3SignerKeystorePath,
    web3SignerPassphrase,
    web3SignerTrustedStorePath,
  } = flags;

  const requiredFlags = [
    "senderAddress",
    "destinationAddress",
    "threshold",
    "blockchainRpcUrl",
    "web3SignerUrl",
    "web3SignerPublicKey",
    ...(tls ? ["web3SignerKeystorePath", "web3SignerPassphrase", "web3SignerTrustedStorePath"] : []),
  ];

  for (const flagName of requiredFlags) {
    if (!flags[flagName]) {
      throw new Error(`Missing required flag: ${flagName}`);
    }
  }

  let maxFeePerGas: bigint;
  try {
    maxFeePerGas = BigInt(maxFeePerGasArg);
    if (maxFeePerGas <= 0n) {
      throw new Error();
    }
  } catch {
    throw new Error(`Invalid value for --max-fee-per-gas: ${maxFeePerGasArg}. Must be a positive integer in wei.`);
  }

  if (gasEstimationPercentile < 0 || gasEstimationPercentile > 100) {
    throw new Error(
      `Invalid value for --gas-estimation-percentile: ${gasEstimationPercentile}. Must be an integer between 0 and 100.`,
    );
  }

  return {
    senderAddress,
    destinationAddress,
    threshold,
    blockchainRpcUrl,
    web3SignerUrl,
    web3SignerPublicKey,
    maxFeePerGas,
    gasEstimationPercentile,
    dryRun,
    tls,
    web3SignerKeystorePath,
    web3SignerPassphrase,
    web3SignerTrustedStorePath,
  };
}
