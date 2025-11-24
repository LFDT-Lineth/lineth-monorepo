/* Run against anvil node

RPC_URL=http://127.0.0.1:8545 \
PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
pnpm --filter @consensys/linea-shared-utils exec tsx scripts/test-ethereum-mainnet-client-library-viem-signer.ts

 */
import { ViemBlockchainClientAdapter } from "../src/clients/ViemBlockchainClientAdapter";
import { ViemWalletSignerClientAdapter } from "../src/clients/ViemWalletSignerClientAdapter";
import { WinstonLogger } from "../src/logging/WinstonLogger";
import { Hex } from "viem";
import { anvil } from "viem/chains";

async function main() {
  const requiredEnvVars = ["RPC_URL", "PRIVATE_KEY"];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }
  const rpcUrl = process.env.RPC_URL as string;
  const privateKey = process.env.PRIVATE_KEY as Hex;

  const signer = new ViemWalletSignerClientAdapter(
    new WinstonLogger("ViemWalletSignerClientAdapter.integration"),
    rpcUrl,
    privateKey,
    anvil,
  );
  const clientLibrary = new ViemBlockchainClientAdapter(
    new WinstonLogger("ViemBlockchainClientAdapter.integration"),
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
