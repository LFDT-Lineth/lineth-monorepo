/* Run against anvil node

Terminal 1 - Run anvil

Prefund the Signer
cast rpc anvil_setBalance 0xD42E308FC964b71E18126dF469c21B0d7bcb86cC 0x8AC7230489E80000 --rpc-url http://localhost:8545

Terminal 2 - Run Web3Signer

docker run --rm \
  --platform=linux/amd64 \
  --name web3signer \
  -p 9000:9000 \
  -v "$(pwd)/docker/web3signer/key-files:/key-files" \
  -v "$(pwd)/docker/web3signer/tls-files:/tls-files" \
  consensys/web3signer:25.12.0 \
  --key-store-path=/key-files/ \
  --tls-keystore-file=/tls-files/web3signer-keystore.p12 \
  --tls-keystore-password-file=/tls-files/web3signer-keystore-password.txt \
  --tls-known-clients-file=/tls-files/known-clients.txt \
  --http-host-allowlist='*' \
  eth1 \
  --chain-id=31337

Terminal 3 - Run script

RPC_URL=http://127.0.0.1:8545 \
WEB3_SIGNER_URL=https://127.0.0.1:9000 \
WEB3_SIGNER_PUBLIC_KEY=0x4a788ad6fa008beed58de6418369717d7492f37d173d70e2c26d9737e2c6eeae929452ef8602a19410844db3e200a0e73f5208fd76259a8766b73953fc3e7023 \
WEB3_SIGNER_KEYSTORE_PATH="$(pwd)/docker/config/linea-besu-sequencer/tls-files/sequencer_client_keystore.p12" \
WEB3_SIGNER_KEYSTORE_PASSPHRASE=changeit \
WEB3_SIGNER_TRUST_STORE_PATH="$(pwd)/docker/config/linea-besu-sequencer/tls-files/web3signer_truststore.p12" \
WEB3_SIGNER_TRUST_STORE_PASSPHRASE=changeit \
pnpm --filter @consensys/linea-shared-utils exec tsx scripts/test-ethereum-mainnet-client-library-web3-signer.ts

 */
import { Hex } from "viem";
import { anvil } from "viem/chains";

import { Web3SignerClientAdapter } from "../src";
import { ViemBlockchainClientAdapter } from "../src/clients/ViemBlockchainClientAdapter";
import { WinstonLogger } from "../src/logging/WinstonLogger";

async function main() {
  const requiredEnvVars = [
    "RPC_URL",
    "WEB3_SIGNER_URL",
    "WEB3_SIGNER_PUBLIC_KEY",
    "WEB3_SIGNER_KEYSTORE_PATH",
    "WEB3_SIGNER_KEYSTORE_PASSPHRASE",
    "WEB3_SIGNER_TRUST_STORE_PATH",
    "WEB3_SIGNER_TRUST_STORE_PASSPHRASE",
  ];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }
  const rpcUrl = process.env.RPC_URL as string;
  const web3SignerUrl = process.env.WEB3_SIGNER_URL as string;
  const web3SignerPublicKey = process.env.WEB3_SIGNER_PUBLIC_KEY as Hex;
  const web3SignerKeystorePath = process.env.WEB3_SIGNER_KEYSTORE_PATH as string;
  const web3SignerKeystorePassphrase = process.env.WEB3_SIGNER_KEYSTORE_PASSPHRASE as string;
  const web3SignerTrustedStorePath = process.env.WEB3_SIGNER_TRUST_STORE_PATH as string;
  const web3SignerTrustedStorePassphrase = process.env.WEB3_SIGNER_TRUST_STORE_PASSPHRASE as string;

  const logger = new WinstonLogger("Web3SignerClientAdapter.integration", { level: "debug" });
  const signer = new Web3SignerClientAdapter(
    logger,
    web3SignerUrl,
    web3SignerPublicKey,
    web3SignerKeystorePath,
    web3SignerKeystorePassphrase,
    web3SignerTrustedStorePath,
    web3SignerTrustedStorePassphrase,
  );
  const clientLibrary = new ViemBlockchainClientAdapter(
    new WinstonLogger("ViemBlockchainClientAdapter.integration", { level: "debug" }),
    rpcUrl,
    anvil,
    signer,
  );

  try {
    const address = signer.getAddress();
    console.log("Address:", address);

    const chainId = await clientLibrary.getChainId();
    console.log("Chain ID:", chainId);

    const balance = await clientLibrary.getBalance(address);
    console.log(`Balance for ${address}:`, balance.toString());

    const fees = await clientLibrary.estimateGasFees();
    console.log("Estimated fees:", fees);

    const receipt = await clientLibrary.sendSignedTransaction(address, "0x");
    console.log("Receipt:", receipt);
  } catch (err) {
    console.error("ViemBlockchainClientAdapter integration script failed:", err);
    process.exitCode = 1;
  }
}

main();
