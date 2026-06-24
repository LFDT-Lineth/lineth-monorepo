import * as fs from "node:fs";

import { ContractFactory, JsonRpcProvider, Wallet, type InterfaceAbi } from "ethers";

import { resolveL1DeployerConfig } from "./deployer-wallet";
import {
  envValue,
  LOCAL_L2_POLICY_DEFAULTS,
  parseDecimalWei,
  sanitizeExternalError,
  SEPOLIA_POLICY_DEFAULTS,
} from "./sepolia-policy";

type Lane = "l1" | "l2";

type AddressBook = {
  l1?: Record<string, string>;
  l2?: Record<string, string>;
};

const [, , laneArg] = process.argv;
const lane = laneArg as Lane;

if (lane !== "l1" && lane !== "l2") {
  throw new Error("usage: ensure-demo-erc20.ts <l1|l2>");
}

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} must be set`);
  }
  return value;
}

function log(message: string) {
  process.stdout.write(`[demo-erc20] ${sanitizeExternalError(message)}\n`);
}

async function main() {
  const addressesPath = process.env.ADDRESSES_PATH ?? "/deployments/addresses.json";
  let addressBook: AddressBook;
  try {
    addressBook = JSON.parse(fs.readFileSync(addressesPath, "utf8")) as AddressBook;
  } catch (error) {
    if (error && typeof error === "object" && "code" in error && error.code === "ENOENT") {
      throw new Error(`${addressesPath} missing; boot the stack first`);
    }
    throw error;
  }
  addressBook.l1 ??= {};
  addressBook.l2 ??= {};

  const l1Config = lane === "l1" ? await resolveL1DeployerConfig(process.env, "container") : undefined;
  const rpcUrl = lane === "l1" ? l1Config!.rpcUrl : requiredEnv("L2_RPC_URL");
  const privateKey = lane === "l1" ? l1Config!.privateKey : requiredEnv("L2_DEPLOYER_PRIVATE_KEY");
  const provider = new JsonRpcProvider(rpcUrl);
  const wallet = new Wallet(privateKey, provider);
  const artifactPath =
    process.env.TEST_ERC20_ARTIFACT ?? "/workspace/contracts/local-deployments-artifacts/static-artifacts/TestERC20.json";
  const artifact = JSON.parse(fs.readFileSync(artifactPath, "utf8")) as { abi: InterfaceAbi; bytecode: string };

  const existing = addressBook[lane]?.ERC20Example;
  if (existing) {
    const code = await provider.getCode(existing);
    if (code !== "0x") {
      log(`reusing ${lane.toUpperCase()} ERC20Example at ${existing}`);
      provider.destroy();
      return;
    }
    log(`${lane.toUpperCase()} ERC20Example address ${existing} has no code; redeploying`);
  }

  const feeOverrides =
    lane === "l1"
      ? {
          gasPrice: parseDecimalWei(
            "L1_DEPLOY_GAS_PRICE_WEI",
            envValue("L1_DEPLOY_GAS_PRICE_WEI", process.env, SEPOLIA_POLICY_DEFAULTS.L1_DEPLOY_GAS_PRICE_WEI),
          ),
        }
      : {
          gasPrice: parseDecimalWei(
            "L2_GAS_PRICE_WEI",
            envValue("L2_GAS_PRICE_WEI", process.env, LOCAL_L2_POLICY_DEFAULTS.L2_GAS_PRICE_WEI),
          ),
        };

  const factory = new ContractFactory(artifact.abi, artifact.bytecode, wallet);
  const contract = await factory.deploy("ERC20Example", "ERC20", "100000", feeOverrides);
  const tx = contract.deploymentTransaction();
  if (!tx) {
    throw new Error("ERC20Example deployment transaction missing");
  }
  console.log(`contract=TestERC20 pending: transactionHash=${tx.hash} nonce=${tx.nonce}`);
  const receipt = await tx.wait();
  if (!receipt) {
    throw new Error("ERC20Example deployment receipt missing");
  }
  const address = await contract.getAddress();
  const chainId = (await provider.getNetwork()).chainId.toString();
  console.log(`contract=TestERC20 deployed: address=${address} blockNumber=${receipt.blockNumber} chainId=${chainId}`);

  addressBook[lane]!.ERC20Example = address;
  addressBook[lane]!.TestERC20 = address;
  const tmpPath = `${addressesPath}.tmp-${process.pid}`;
  fs.writeFileSync(tmpPath, `${JSON.stringify(addressBook, null, 2)}\n`, { mode: 0o644 });
  fs.renameSync(tmpPath, addressesPath);
  log(`wrote ${lane.toUpperCase()} ERC20Example to ${addressesPath}`);
  provider.destroy();
}

main().catch((error) => {
  process.stderr.write(`[demo-erc20] ERROR: ${sanitizeExternalError(error)}\n`);
  process.exit(1);
});
