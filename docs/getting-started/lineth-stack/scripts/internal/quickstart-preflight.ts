import * as path from "node:path";

import { JsonRpcProvider } from "ethers";

import { resolveL1DeployerConfig } from "./deployer-wallet";
import { buildSepoliaPolicyConfig, readDotEnvFile, runL1PolicyCheck, sanitizeExternalError } from "./sepolia-policy";

function log(message: string) {
  process.stdout.write(`[quickstart-preflight] ${sanitizeExternalError(message)}\n`);
}

function trimDecimal(value: string): string {
  return value.replace(/\.?0+$/, "");
}

function formatDecimal(value: bigint, decimals: number, precision: number): string {
  const scale = 10n ** BigInt(decimals);
  const whole = value / scale;
  const remainder = value % scale;
  if (remainder === 0n || precision === 0) {
    return whole.toString();
  }

  const fractional = remainder.toString().padStart(decimals, "0").slice(0, precision);
  return trimDecimal(`${whole}.${fractional}`);
}

function formatGwei(value: bigint | undefined): string {
  if (value === undefined) {
    return "unavailable";
  }
  return `${formatDecimal(value, 9, 3)} gwei`;
}

function formatEth(value: bigint): string {
  return `${formatDecimal(value, 18, 4)} ETH`;
}

class FundingRequiredError extends Error {
  constructor() {
    super("Sepolia deployer funding required");
  }
}

function isUnderfundedError(error: unknown): boolean {
  return error instanceof Error && /fund it to at least [0-9]+ wei/.test(error.message);
}

async function printFundingInstructions(params: {
  provider: JsonRpcProvider;
  address: string;
  minimumBalanceWei: bigint;
  keystorePath?: string;
  source: string;
}) {
  const currentBalanceWei = await params.provider.getBalance(params.address);
  log("Sepolia deployer funding required before Docker startup");
  log(`Deployer address              ${params.address}`);
  log(`Deployer source               ${params.source}`);
  if (params.keystorePath) {
    log(`Deployer keystore             ${params.keystorePath}`);
  }
  log(`Minimum required funding      ${params.minimumBalanceWei} wei (${formatEth(params.minimumBalanceWei)})`);
  log(`Current balance               ${currentBalanceWei} wei (${formatEth(currentBalanceWei)})`);
  log("Fund the deployer address on Sepolia, then rerun:");
  log("./scripts/start.sh --tail");
}

async function main() {
  const stackDir = process.env.LINETH_STACK_DIR ?? process.cwd();
  const envPath = path.join(stackDir, ".env");
  const fileEnv = readDotEnvFile(envPath);
  const env = { ...fileEnv, ...process.env };

  const l1Deployer = await resolveL1DeployerConfig(env, "host", { stackDir });
  const provider = new JsonRpcProvider(l1Deployer.rpcUrl);

  try {
    let report;
    try {
      report = await runL1PolicyCheck({
        provider,
        deployerAddress: l1Deployer.address,
        env,
      });
    } catch (error) {
      if (l1Deployer.mode === "sepolia" && isUnderfundedError(error)) {
        await printFundingInstructions({
          provider,
          address: l1Deployer.address,
          minimumBalanceWei: buildSepoliaPolicyConfig(env).l1DeployerMinBalanceWei,
          keystorePath: l1Deployer.keystorePath,
          source: l1Deployer.source,
        });
        throw new FundingRequiredError();
      }
      throw error;
    }

    for (const warning of report.warnings) {
      log(`warn: ${warning}`);
    }

    log(`mode                          ${report.mode === "local" ? "local L1" : "Sepolia"}`);
    if (l1Deployer.created && l1Deployer.keystorePath) {
      log(`Generated deployer keystore   ${l1Deployer.keystorePath}`);
    }
    log(`rpc                           reachable`);
    log(`chain                         chainId ${report.chainId}`);
    log(`deployer                      ${report.deployerAddress}`);
    log(`deployer source               ${l1Deployer.source}`);
    log(`balance                       ${formatEth(report.balanceWei)}`);
    log(`nonce                         latest ${report.latestNonce}, pending ${report.pendingNonce}`);
    if (report.mode === "sepolia") {
      log(
        `gas                           execution ${formatGwei(report.currentExecutionFeeWei)} / ` +
          `cap ${formatGwei(report.config.l1DeployGasPriceWei)}`,
      );
      log(
        `blob fee                      blob base ${formatGwei(report.blobBaseFeeWei)} / ` +
          `cap ${formatGwei(report.config.l1BlobMaxFeePerBlobGasCapWei)}`,
      );
      log(`blob tx cap                   ${formatGwei(report.config.l1BlobMaxFeePerGasCapWei)}`);
      log(`finalization cap              ${formatGwei(report.config.l1FinalizationMaxFeePerGasCapWei)}`);
    } else {
      log(`blocks                        advancing`);
      log("gas                           local dev mode; Sepolia gas/blob gates skipped");
    }
  } finally {
    provider.destroy();
  }
}

main().catch((error) => {
  if (error instanceof FundingRequiredError) {
    process.exit(1);
  }
  process.stderr.write(`[quickstart-preflight] ERROR: ${sanitizeExternalError(error)}\n`);
  process.exit(1);
});
