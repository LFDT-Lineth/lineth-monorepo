import { JsonRpcProvider, Wallet } from "ethers";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";

import { sanitizePrivKey } from "../scripts/cli";

const argv = yargs(hideBin(process.argv))
  .option("rpc-url", {
    describe: "RPC url",
    type: "string",
    demandOption: true,
  })
  .option("wallet-priv-key", {
    describe: "deployer private key",
    type: "string",
    demandOption: true,
    coerce: sanitizePrivKey("priv-key"),
  })
  .parseSync();

async function main(args: typeof argv) {
  const provider = new JsonRpcProvider(args.rpcUrl);
  const wallet = new Wallet(args.walletPrivKey, provider);

  const nonce = await wallet.getNonce();

  process.env.L1_NONCE = nonce.toString();
  process.stdout.write(process.env.L1_NONCE);
}

main(argv).catch((error) => {
  console.error(error);
  process.exit(1);
});
