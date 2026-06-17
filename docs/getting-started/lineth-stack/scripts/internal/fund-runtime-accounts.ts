import * as fs from "node:fs";
import * as path from "node:path";

import { isAddress, JsonRpcProvider, type TransactionReceipt, Wallet } from "ethers";

import { resolveL1DeployerConfig } from "./deployer-wallet";
import { buildSepoliaPolicyConfig, sanitizeExternalError } from "./sepolia-policy";

type AddressBook = {
  deployers?: {
    l1?: string;
  };
  signers?: {
    l1BlobSubmitterAddress?: string;
    l1FinalizationSubmitterAddress?: string;
    l1PostmanAddress?: string;
    l2DeployerAddress?: string;
    l2MessageAnchoringAddress?: string;
    l2PostmanAddress?: string;
  };
};

type FundingTarget = {
  label: string;
  address: string;
  minBalance: bigint;
  topUp: bigint;
};

type FundingPlan = FundingTarget & {
  balanceBefore: bigint;
};

type SentFundingTx = FundingPlan & {
  hash: string;
  nonce: number;
};

const PRECOMPUTED = process.env.PRECOMPUTED ?? "/accounts/addresses-precomputed.json";
const DEPLOY_TIMING_PATH = process.env.DEPLOY_TIMING_PATH ?? "/deployments/deploy-timing.jsonl";

function log(message: string) {
  console.log(`[fund-runtime-accounts] ${message}`);
}

function die(message: string): never {
  throw new Error(`[fund-runtime-accounts] ERROR: ${message}`);
}

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    die(`${name} must be set`);
  }
  return value;
}

function envNumber(name: string, fallback: number): number {
  const raw = process.env[name] ?? fallback.toString();
  if (!/^[0-9]+$/.test(raw)) {
    die(`${name} must be an integer value`);
  }
  return Number(raw);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function withTimeout<T>(promise: Promise<T>, timeoutMs: number, description: string): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new Error(`${description} timed out after ${timeoutMs}ms`)), timeoutMs);
      }),
    ]);
  } finally {
    if (timer !== undefined) {
      clearTimeout(timer);
    }
  }
}

function sameAddress(left: string, right: string): boolean {
  return left.toLowerCase() === right.toLowerCase();
}

function requiredAddress(value: unknown, label: string): string {
  if (typeof value !== "string" || !isAddress(value)) {
    die(`${label} is missing or invalid in ${PRECOMPUTED}`);
  }
  return value;
}

function loadAddressBook(): AddressBook {
  if (!fs.existsSync(PRECOMPUTED)) {
    die(`${PRECOMPUTED} not found; account-setup must run first`);
  }
  return JSON.parse(fs.readFileSync(PRECOMPUTED, "utf8")) as AddressBook;
}

function recordTiming(name: string, startedMs: number, endedMs: number, status: "ok" | "failed") {
  const durationMs = Math.max(0, endedMs - startedMs);
  fs.mkdirSync(path.dirname(DEPLOY_TIMING_PATH), { recursive: true });
  fs.appendFileSync(
    DEPLOY_TIMING_PATH,
    `${JSON.stringify({
      name,
      status,
      startedAt: new Date(startedMs).toISOString(),
      endedAt: new Date(endedMs).toISOString(),
      durationMs,
      durationSeconds: Number((durationMs / 1000).toFixed(3)),
    })}\n`,
  );
}

async function buildFundingPlans(provider: JsonRpcProvider, targets: FundingTarget[]): Promise<FundingPlan[]> {
  const balances = await Promise.all(targets.map((target) => provider.getBalance(target.address)));
  const plans: FundingPlan[] = [];

  for (const [index, target] of targets.entries()) {
    const balanceBefore = balances[index];
    if (balanceBefore === undefined) {
      die(`Missing balance result for ${target.label}`);
    }
    log(`${target.label} balance=${balanceBefore} wei address=${target.address}`);
    if (balanceBefore >= target.minBalance) {
      log(`${target.label} skipped: balance already >= ${target.minBalance} wei`);
      continue;
    }
    plans.push({ ...target, balanceBefore });
  }

  return plans;
}

async function waitForReceipt(params: {
  provider: JsonRpcProvider;
  hash: string;
  confirmations: number;
  timeoutMs: number;
  pollIntervalMs: number;
  requestTimeoutMs: number;
}): Promise<TransactionReceipt> {
  const deadlineMs = Date.now() + params.timeoutMs;
  let lastError = "";

  while (Date.now() < deadlineMs) {
    try {
      const receipt = await withTimeout<TransactionReceipt | null>(
        params.provider.getTransactionReceipt(params.hash),
        params.requestTimeoutMs,
        `getTransactionReceipt ${params.hash}`,
      );
      if (receipt !== null) {
        const latestBlock = await withTimeout<number>(
          params.provider.getBlockNumber(),
          params.requestTimeoutMs,
          `getBlockNumber for ${params.hash}`,
        );
        const confirmations = latestBlock >= receipt.blockNumber ? latestBlock - receipt.blockNumber + 1 : 0;
        if (confirmations >= params.confirmations) {
          return receipt;
        }
      }
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);
    }

    await sleep(params.pollIntervalMs);
  }

  const suffix = lastError ? `; last RPC error: ${lastError}` : "";
  die(`Timed out waiting for funding tx ${params.hash}${suffix}`);
}

async function fundBatch(params: {
  label: string;
  timingName: string;
  provider: JsonRpcProvider;
  wallet: Wallet;
  targets: FundingTarget[];
  gasPrice: bigint;
  gasLimit: bigint;
}) {
  const startedMs = Date.now();
  const receiptTimeoutMs = envNumber("RUNTIME_FUNDING_RECEIPT_TIMEOUT_MS", 300_000);
  const receiptPollIntervalMs = envNumber("RUNTIME_FUNDING_RECEIPT_POLL_INTERVAL_MS", 2_000);
  const rpcRequestTimeoutMs = envNumber("RUNTIME_FUNDING_RPC_REQUEST_TIMEOUT_MS", 15_000);
  try {
    const plans = await buildFundingPlans(params.provider, params.targets);
    if (plans.length === 0) {
      log(`${params.label} batch: no top-ups needed`);
      recordTiming(params.timingName, startedMs, Date.now(), "ok");
      return;
    }

    const senderBalance = await params.provider.getBalance(params.wallet.address);
    const requiredValue = plans.reduce((total, plan) => total + plan.topUp, 0n);
    const requiredGas = BigInt(plans.length) * params.gasLimit * params.gasPrice;
    const requiredTotal = requiredValue + requiredGas;
    if (senderBalance < requiredTotal) {
      die(
        `Cannot fund ${params.label} runtime accounts: sender ${params.wallet.address} has ${senderBalance} wei, ` +
          `needs at least ${requiredTotal} wei including value ${requiredValue} and gas ${requiredGas}`,
      );
    }

    const firstNonce = await params.provider.getTransactionCount(params.wallet.address, "pending");
    const sent: SentFundingTx[] = [];
    for (let index = 0; index < plans.length; index++) {
      const plan = plans[index];
      if (plan === undefined) {
        die(`Missing funding plan at index ${index}`);
      }
      const nonce = firstNonce + index;
      log(`${params.label} funding ${plan.label}: value=${plan.topUp} wei nonce=${nonce}`);
      const tx = await params.wallet.sendTransaction({
        to: plan.address,
        value: plan.topUp,
        nonce,
        gasLimit: params.gasLimit,
        gasPrice: params.gasPrice,
      });
      log(`${params.label} funding ${plan.label}: tx=${tx.hash} nonce=${nonce}`);
      sent.push({ ...plan, hash: tx.hash, nonce });
    }

    const receipts = await Promise.all(
      sent.map(async (tx) => {
        const receipt = await waitForReceipt({
          provider: params.provider,
          hash: tx.hash,
          confirmations: 1,
          timeoutMs: receiptTimeoutMs,
          pollIntervalMs: receiptPollIntervalMs,
          requestTimeoutMs: rpcRequestTimeoutMs,
        });
        if (!receipt || receipt.status !== 1) {
          die(`${params.label} funding ${tx.label} failed: tx=${tx.hash}`);
        }
        log(`${params.label} funding ${tx.label}: confirmed block=${receipt.blockNumber} tx=${tx.hash}`);
        return receipt;
      }),
    );
    if (receipts.length !== sent.length) {
      die(`${params.label} funding receipt count mismatch`);
    }

    const finalBalances = await Promise.all(sent.map((tx) => params.provider.getBalance(tx.address)));
    for (let index = 0; index < sent.length; index++) {
      const tx = sent[index];
      const balanceAfter = finalBalances[index];
      if (tx === undefined || balanceAfter === undefined) {
        die(`Missing funding result at index ${index}`);
      }
      log(`${params.label} funding ${tx.label}: finalBalance=${balanceAfter} wei`);
      if (balanceAfter < tx.minBalance) {
        die(`${params.label} funding ${tx.label} left balance ${balanceAfter} below minimum ${tx.minBalance}`);
      }
    }

    recordTiming(params.timingName, startedMs, Date.now(), "ok");
  } catch (error) {
    recordTiming(params.timingName, startedMs, Date.now(), "failed");
    throw error;
  }
}

async function main() {
  const l1Config = await resolveL1DeployerConfig(process.env, "container");
  const l1Provider = new JsonRpcProvider(l1Config.rpcUrl);
  const l2Provider = new JsonRpcProvider(requiredEnv("L2_RPC_URL"));
  const l1Wallet = new Wallet(l1Config.privateKey, l1Provider);
  const l2Wallet = new Wallet(requiredEnv("L2_DEPLOYER_PRIVATE_KEY"), l2Provider);
  const addressBook = loadAddressBook();
  const signers = addressBook.signers ?? {};

  const expectedL1Deployer = requiredAddress(addressBook.deployers?.l1, "deployers.l1");
  const expectedL2Deployer = requiredAddress(signers.l2DeployerAddress, "signers.l2DeployerAddress");
  if (!sameAddress(l1Wallet.address, expectedL1Deployer)) {
    die(`resolved L1 deployer ${l1Wallet.address}, expected ${expectedL1Deployer}`);
  }
  if (!sameAddress(l2Wallet.address, expectedL2Deployer)) {
    die(`L2_DEPLOYER_PRIVATE_KEY resolves to ${l2Wallet.address}, expected ${expectedL2Deployer}`);
  }

  const policyConfig = buildSepoliaPolicyConfig(process.env);

  await Promise.all([
    fundBatch({
      label: "L1",
      timingName: "Runtime funding: L1 batch",
      provider: l1Provider,
      wallet: l1Wallet,
      gasPrice: policyConfig.l1DeployGasPriceWei,
      gasLimit: 21000n,
      targets: [
        {
          label: "blob submitter",
          address: requiredAddress(signers.l1BlobSubmitterAddress, "signers.l1BlobSubmitterAddress"),
          minBalance: policyConfig.l1RoleMinBalanceWei,
          topUp: policyConfig.l1RoleTopUpWei,
        },
        {
          label: "finalization submitter",
          address: requiredAddress(signers.l1FinalizationSubmitterAddress, "signers.l1FinalizationSubmitterAddress"),
          minBalance: policyConfig.l1RoleMinBalanceWei,
          topUp: policyConfig.l1RoleTopUpWei,
        },
        {
          label: "postman",
          address: requiredAddress(signers.l1PostmanAddress, "signers.l1PostmanAddress"),
          minBalance: policyConfig.l1PostmanMinBalanceWei,
          topUp: policyConfig.l1PostmanTopUpWei,
        },
      ],
    }),
    fundBatch({
      label: "L2",
      timingName: "Runtime funding: L2 batch",
      provider: l2Provider,
      wallet: l2Wallet,
      gasPrice: policyConfig.l2GasPriceWei,
      gasLimit: 21000n,
      targets: [
        {
          label: "message anchorer",
          address: requiredAddress(signers.l2MessageAnchoringAddress, "signers.l2MessageAnchoringAddress"),
          minBalance: policyConfig.l2RuntimeMinBalanceWei,
          topUp: policyConfig.l2RuntimeTopUpWei,
        },
        {
          label: "postman",
          address: requiredAddress(signers.l2PostmanAddress, "signers.l2PostmanAddress"),
          minBalance: policyConfig.l2RuntimeMinBalanceWei,
          topUp: policyConfig.l2RuntimeTopUpWei,
        },
      ],
    }),
  ]);

  l1Provider.destroy();
  l2Provider.destroy();
  log("Done.");
}

main().catch((error) => {
  process.stderr.write(`${sanitizeExternalError(error)}\n`);
  process.exit(1);
});
