// Executes the custom bootstrap phase: operator-supplied transactions that run
// after the 5 core deployment steps. Three item kinds share one ordered pass:
//   - sign:      deployer signs & submits (value send or contract create) using
//                the runner-supplied starting nonce for continuity.
//   - presigned: broadcast a keyless/external raw transaction as-is.
//   - script:    spawn an operator child script that may create/sign/submit.
// Items arrive via BOOTSTRAP_ITEMS (JSON array) filtered to this chain by the
// runner. Each completed item emits a `lineth-bootstrap-record` IPC message so
// the parent checkpoints it; completed items are skipped on rerun.
import { ethers } from "ethers";
import { spawn } from "node:child_process";

import { sendAndAwaitAck } from "../../../common/helpers/deploymentRecord";
import { getRequiredEnvVar } from "../../../common/helpers/environment";
import { resolveL2DeployFeeOverrides, resolveOneModelFeeOverrides } from "../../../common/helpers/feeOverrides";
import { BootstrapItem, PresignedBootstrapItem, ScriptBootstrapItem, SignBootstrapItem } from "../bootstrap-manifest";
import { orderBootstrapItems, resolveBootstrapScriptPath } from "./bootstrap-helpers";

interface BootstrapRunContext {
  provider: ethers.JsonRpcProvider;
  wallet: ethers.Wallet;
  chain: "l1" | "l2";
  chainId: string;
  scriptsDir?: string | undefined;
}

// Placeholder transactionHash for script items, which perform arbitrary actions
// with no single hash to record. Signals "completed; rerun must skip".
const SCRIPT_ITEM_SENTINEL = "0x" + "00".repeat(32);

let recordSequence = 0;

function requireItems(): BootstrapItem[] {
  const raw = getRequiredEnvVar("BOOTSTRAP_ITEMS");
  const parsed: unknown = JSON.parse(raw);
  if (!Array.isArray(parsed)) throw new Error("BOOTSTRAP_ITEMS must be a JSON array");
  // Items were fully validated by the runner before serialization; trust them here.
  return parsed as BootstrapItem[];
}

// Persist one completed bootstrap item with the checkpoint parent and wait for
// its acknowledgement, via the same send/ack/disconnect handling as
// awaitParentCheckpoint/awaitParentDeploymentIntent.
async function awaitBootstrapRecord(record: {
  itemId: string;
  kind: BootstrapItem["kind"];
  chain: "l1" | "l2";
  transactionHash: string;
  blockNumber: number;
  chainId: string;
  address?: string | undefined;
}): Promise<void> {
  console.log(
    `bootstrap=${record.itemId} complete: tx=${record.transactionHash} block=${record.blockNumber}` +
      (record.address ? ` address=${record.address}` : ""),
  );
  await sendAndAwaitAck(
    (id) => ({ type: "lineth-bootstrap-record", id, record }),
    "lineth-bootstrap-record-ack",
    () => `${process.pid}-bootstrap-${recordSequence++}`,
    "checkpoint parent disconnected before acknowledging bootstrap record",
  );
}

async function runSignItem(item: SignBootstrapItem, nonce: number, context: BootstrapRunContext): Promise<void> {
  const fees =
    context.chain === "l2"
      ? resolveL2DeployFeeOverrides()
      : await resolveOneModelFeeOverrides(context.provider, "L1_DEPLOY_GAS_PRICE_WEI");
  const request: ethers.TransactionRequest = {
    value: BigInt(item.valueWei),
    nonce,
    ...(item.to !== undefined ? { to: item.to } : {}),
    ...(item.data !== undefined ? { data: item.data } : {}),
    ...(item.gasLimit !== undefined ? { gasLimit: BigInt(item.gasLimit) } : {}),
    ...fees,
  };
  const tx = await context.wallet.sendTransaction(request);
  const receipt = await tx.wait();
  if (!receipt || receipt.status !== 1) throw new Error(`bootstrap sign item ${item.id} tx ${tx.hash} failed`);

  let address = item.expectAddress;
  if (item.to === undefined) {
    // Bare CREATE: the deployed address is receipt.contractAddress. Assert it
    // matches the pinned expectAddress when one is supplied.
    const deployed = receipt.contractAddress;
    if (!deployed) throw new Error(`bootstrap sign create ${item.id} tx ${tx.hash} returned no contract address`);
    if (address && deployed.toLowerCase() !== address.toLowerCase()) {
      throw new Error(`bootstrap sign create ${item.id} deployed at ${deployed}; expected ${address}`);
    }
    address = deployed;
  } else if (address) {
    // Contract call (e.g. a CREATE2 deploy through the deterministic proxy):
    // receipt.contractAddress is null, so verify code landed at expectAddress.
    const code = await context.provider.getCode(address);
    if (code === "0x") {
      throw new Error(`bootstrap sign item ${item.id} produced no bytecode at expectAddress ${address}`);
    }
  }
  await awaitBootstrapRecord({
    itemId: item.id,
    kind: item.kind,
    chain: context.chain,
    transactionHash: tx.hash,
    blockNumber: receipt.blockNumber,
    chainId: context.chainId,
    address,
  });
}

async function runPresignedItem(item: PresignedBootstrapItem, context: BootstrapRunContext): Promise<void> {
  if (item.expectAddress) {
    const existing = await context.provider.getCode(item.expectAddress);
    if (existing !== "0x") {
      console.log(`bootstrap=${item.id} already deployed at ${item.expectAddress}; skipping`);
      // A skip still needs a checkpoint record, otherwise a rerun would keep
      // this item pending and re-check it every time. No real tx hash exists
      // for a skip, so reuse the script item's sentinel to signal "done".
      const blockNow = await context.provider.getBlockNumber();
      await awaitBootstrapRecord({
        itemId: item.id,
        kind: item.kind,
        chain: context.chain,
        transactionHash: SCRIPT_ITEM_SENTINEL,
        blockNumber: blockNow,
        chainId: context.chainId,
        address: item.expectAddress,
      });
      return;
    }
  }
  const tx = await context.provider.broadcastTransaction(item.rawTx);
  const receipt = await tx.wait();
  if (!receipt || receipt.status !== 1) throw new Error(`bootstrap presigned item ${item.id} tx ${tx.hash} failed`);
  // Verify code actually landed at expectAddress when one is pinned — a
  // "successful" broadcast isn't proof the deployment happened (e.g. it could
  // be a value-only transfer, or the raw tx wasn't actually a deployment).
  if (item.expectAddress) {
    const code = await context.provider.getCode(item.expectAddress);
    if (code === "0x") {
      throw new Error(
        `bootstrap presigned item ${item.id} produced no bytecode at expectAddress ${item.expectAddress}`,
      );
    }
  }
  await awaitBootstrapRecord({
    itemId: item.id,
    kind: item.kind,
    chain: context.chain,
    transactionHash: tx.hash,
    blockNumber: receipt.blockNumber,
    chainId: context.chainId,
    address: item.expectAddress,
  });
}

async function runScriptItem(item: ScriptBootstrapItem, nonce: number, context: BootstrapRunContext): Promise<void> {
  if (!context.scriptsDir) throw new Error(`bootstrap script item ${item.id} requires BOOTSTRAP_SCRIPTS_DIR`);
  const scriptPath = resolveBootstrapScriptPath(context.scriptsDir, item.id, item.script);
  const environment: NodeJS.ProcessEnv = {
    RPC_URL: process.env.RPC_URL ?? "",
    DEPLOYER_PRIVATE_KEY: process.env.DEPLOYER_PRIVATE_KEY ?? "",
    BOOTSTRAP_ITEM_ID: item.id,
    BOOTSTRAP_CHAIN: context.chain,
    BOOTSTRAP_CHAIN_ID: context.chainId,
    BOOTSTRAP_START_NONCE: nonce.toString(),
    // Let operator scripts require libraries bundled into the image (ethers).
    ...(process.env.BOOTSTRAP_LIB_NODE_PATH ? { NODE_PATH: process.env.BOOTSTRAP_LIB_NODE_PATH } : {}),
  };
  await new Promise<void>((resolve, reject) => {
    const child = spawn(process.execPath, [scriptPath], { env: environment, stdio: ["ignore", "inherit", "inherit"] });
    child.once("error", reject);
    child.once("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`bootstrap script item ${item.id} exited with code ${code ?? 1}`));
    });
  });
  // A script runs arbitrary actions (often many transactions), so there is no
  // single hash to record. The sentinel marks the item complete so a rerun
  // skips it; the script itself is responsible for idempotency of its own
  // on-chain actions. Record the chain head it completed at for traceability.
  const blockAfter = await context.provider.getBlockNumber();
  await awaitBootstrapRecord({
    itemId: item.id,
    kind: item.kind,
    chain: context.chain,
    transactionHash: SCRIPT_ITEM_SENTINEL,
    blockNumber: blockAfter,
    chainId: context.chainId,
  });
}

async function main(): Promise<void> {
  const chain = process.env.BOOTSTRAP_CHAIN;
  if (chain !== "l1" && chain !== "l2") throw new Error(`BOOTSTRAP_CHAIN must be "l1" or "l2", got: ${chain}`);
  const provider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const wallet = new ethers.Wallet(getRequiredEnvVar("DEPLOYER_PRIVATE_KEY"), provider);
  const rawNonce = getRequiredEnvVar("BOOTSTRAP_START_NONCE");
  if (!/^[0-9]+$/.test(rawNonce)) throw new Error(`BOOTSTRAP_START_NONCE must be a non-negative integer`);
  let nonce = Number(rawNonce);
  const chainId = (await provider.getNetwork()).chainId.toString();
  const context: BootstrapRunContext = {
    provider,
    wallet,
    chain,
    chainId,
    scriptsDir: process.env.BOOTSTRAP_SCRIPTS_DIR,
  };

  const items = requireItems();
  const ordered = orderBootstrapItems(items);

  for (const item of ordered) {
    if (item.kind === "sign") {
      await runSignItem(item, nonce, context);
      nonce += 1;
    } else if (item.kind === "presigned") {
      await runPresignedItem(item, context);
    } else {
      await runScriptItem(item, nonce, context);
      // A script may consume deployer nonces; resync from the chain.
      nonce = await provider.getTransactionCount(wallet.address, "pending");
    }
  }
}

main().catch((error: unknown) => {
  console.error(error);
  process.exitCode = 1;
});
