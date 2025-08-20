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
  web3SignerKeystorePassphrase: string;
  web3SignerTrustedStorePath: string;
  web3SignerTrustedStorePassphrase: string;
};

export function validateConfig(flags: ParserOutput["flags"]): Config {
  const requiredFlags = [
    "sender-address",
    "destination-address",
    "threshold",
    "blockchain-rpc-url",
    "web3-signer-url",
    "web3-signer-public-key",
    ...(flags.tls
      ? [
          "web3-signer-keystore-path",
          "web3-signer-keystore-passphrase",
          "web3-signer-trusted-store-path",
          "web3-signer-trusted-store-passphrase",
        ]
      : []),
  ];

  for (const flagName of requiredFlags) {
    if (!flags[flagName]) {
      throw new Error(`Missing required flag: ${flagName}`);
    }
  }

  let maxFeePerGas: bigint;
  try {
    maxFeePerGas = BigInt(flags["max-fee-per-gas"]);
    if (maxFeePerGas <= 0n) {
      throw new Error();
    }
  } catch {
    throw new Error(
      `Invalid value for --max-fee-per-gas: ${flags["max-fee-per-gas"]}. Must be a positive integer in wei.`,
    );
  }

  if (flags["gas-estimation-percentile"] < 0 || flags["gas-estimation-percentile"] > 100) {
    throw new Error(
      `Invalid value for --gas-estimation-percentile: ${flags["gas-estimation-percentile"]}. Must be an integer between 0 and 100.`,
    );
  }

  return {
    senderAddress: flags["sender-address"],
    destinationAddress: flags["destination-address"],
    threshold: flags["threshold"],
    blockchainRpcUrl: flags["blockchain-rpc-url"],
    web3SignerUrl: flags["web3-signer-url"],
    web3SignerPublicKey: flags["web3-signer-public-key"],
    maxFeePerGas,
    gasEstimationPercentile: flags["gas-estimation-percentile"],
    dryRun: flags["dry-run"],
    tls: flags.tls,
    web3SignerKeystorePath: flags["web3-signer-keystore-path"],
    web3SignerKeystorePassphrase: flags["web3-signer-keystore-passphrase"],
    web3SignerTrustedStorePath: flags["web3-signer-trusted-store-path"],
    web3SignerTrustedStorePassphrase: flags["web3-signer-trusted-store-passphrase"],
  };
}
