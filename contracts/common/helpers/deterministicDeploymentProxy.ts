import { ethers } from "ethers";

import { DeploymentRecordInput, formatDeploymentRecord } from "./deploymentRecord";
import { FeeOverrides } from "./feeOverrides";

const DETERMINISTIC_DEPLOYMENT_PROXY_CONTRACT_NAME = "DeterministicDeploymentProxy";

/**
 * EIP-7997 / Arachnid deterministic deployment proxy ("Create2Factory").
 *
 * This is the well-known keyless deployment pattern: a pre-signed legacy
 * transaction whose `r`/`s` are the fixed `0x2222...` values deploys the same
 * factory at the same address on every EVM chain. Because the transaction is
 * already signed, broadcasting it does not consume any nonce from our keys and
 * is not signed by the deployer wallet — we only need to fund the recovered
 * signer so the network accepts the pre-signed transaction.
 */
export const ARACHNID_SIGNER = ethers.getAddress("0x3fAB184622Dc19b6109349B94811493BF2a45362");
export const ARACHNID_FACTORY = ethers.getAddress("0x4e59b44847b379578588920cA78FbF26c0B4956C");

// gasLimit(0x186a0 = 100000) * gasPrice(0x174876e800 = 100 gwei) hardcoded by the raw tx.
export const ARACHNID_FUNDING_WEI = 10_000_000_000_000_000n;

export const ARACHNID_RAW_TX =
  "0xf8a58085174876e800830186a08080b853604580600e600039806000f350fe7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf31ba02222222222222222222222222222222222222222222222222222222222222222a02222222222222222222222222222222222222222222222222222222222222222";

// The runtime bytecode the factory is expected to have after a successful deploy
// (the portion of ARACHNID_RAW_TX's init code returned via CODECOPY/RETURN). Used
// to distinguish "factory genuinely deployed" from "unrelated bytecode happens to
// occupy this address", rather than trusting any non-empty code at ARACHNID_FACTORY.
export const ARACHNID_FACTORY_RUNTIME_CODE =
  "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3";
export const ARACHNID_FACTORY_RUNTIME_CODE_HASH = ethers.keccak256(ARACHNID_FACTORY_RUNTIME_CODE);

// Fails fast if ARACHNID_RAW_TX ever stops recovering to ARACHNID_SIGNER — e.g.
// a future edit corrupts the constant. Funding the wrong recovered signer would
// silently waste the transfer and leave the pre-signed broadcast unable to
// land. Called explicitly from ensureDeterministicDeploymentProxy (before any
// funds move) rather than at module import time, so merely importing a type
// from this module can never throw as a side effect.
function assertArachnidRawTxIntegrity(): void {
  const recoveredSigner = ethers.Transaction.from(ARACHNID_RAW_TX).from;
  if (!recoveredSigner || ethers.getAddress(recoveredSigner) !== ARACHNID_SIGNER) {
    throw new Error(
      `ARACHNID_RAW_TX recovers to signer ${recoveredSigner ?? "<none>"}, expected ${ARACHNID_SIGNER}; refusing to use the deterministic-deployment-proxy helper`,
    );
  }
}

export interface EnsureDeterministicProxyInput {
  provider: ethers.Provider;
  wallet: ethers.Wallet;
  fees: FeeOverrides;
  /** Nonce for the funding transfer (the deployer's next free nonce). */
  nonce: number;
}

export interface EnsureDeterministicProxyResult {
  factoryAddress: string;
  /** Present only when a funding transfer was actually broadcast. */
  fundingTxHash?: string;
  /** Present only when the raw deployment transaction was actually broadcast. */
  deployTxHash?: string;
  /** Block number the deployment transaction was mined in (when broadcast). */
  deployBlockNumber?: number;
  /** True when the factory already existed and nothing was broadcast. */
  alreadyDeployed: boolean;
}

/** Whether the on-chain code at ARACHNID_FACTORY is absent, the expected factory, or something else. */
export type DeterministicProxyCodeStatus = "absent" | "match" | "mismatch";

/**
 * Checks the bytecode at ARACHNID_FACTORY against the known factory runtime
 * code, rather than merely checking for non-empty code. Any non-empty code
 * that does not hash-match the expected factory is treated as a "mismatch",
 * not as "already deployed" — this guards against adopting or funding on top
 * of unrelated bytecode that could occupy the well-known address (e.g. a
 * misbehaving RPC endpoint, or a chain that does not actually preinstall it).
 */
export async function getDeterministicProxyCodeStatus(
  provider: ethers.Provider,
): Promise<DeterministicProxyCodeStatus> {
  const code = await provider.getCode(ARACHNID_FACTORY);
  if (code === "0x") return "absent";
  return ethers.keccak256(code) === ARACHNID_FACTORY_RUNTIME_CODE_HASH ? "match" : "mismatch";
}

export async function isDeterministicProxyDeployed(provider: ethers.Provider): Promise<boolean> {
  return (await getDeterministicProxyCodeStatus(provider)) !== "absent";
}

/**
 * Idempotently installs the deterministic deployment proxy on the connected
 * chain. Returns early when factory bytecode already exists. Otherwise funds
 * the keyless signer with the exact gas budget and broadcasts the pre-signed
 * raw transaction, then verifies the factory has code.
 */
export async function ensureDeterministicDeploymentProxy(
  input: EnsureDeterministicProxyInput,
): Promise<EnsureDeterministicProxyResult> {
  assertArachnidRawTxIntegrity();
  const { provider, wallet, fees, nonce } = input;

  const initialStatus = await getDeterministicProxyCodeStatus(provider);
  if (initialStatus === "mismatch") {
    throw new Error(
      `Refusing to treat ${ARACHNID_FACTORY} as the deterministic deployment proxy; on-chain bytecode does not match the expected factory`,
    );
  }
  if (initialStatus === "match") {
    console.log(`DeterministicDeploymentProxy already deployed at ${ARACHNID_FACTORY}; skipping`);
    return { factoryAddress: ARACHNID_FACTORY, alreadyDeployed: true };
  }

  console.log(`Funding deterministic deployment signer ${ARACHNID_SIGNER} with ${ARACHNID_FUNDING_WEI} wei`);
  const fundingTx = await wallet.sendTransaction({
    to: ARACHNID_SIGNER,
    value: ARACHNID_FUNDING_WEI,
    nonce,
    ...fees,
  });
  const fundingReceipt = await fundingTx.wait();
  if (!fundingReceipt || fundingReceipt.status !== 1) {
    throw new Error(`Funding transaction ${fundingTx.hash} for ${ARACHNID_SIGNER} failed`);
  }
  console.log(`Funded deterministic deployment signer: tx=${fundingTx.hash} block=${fundingReceipt.blockNumber}`);

  const deployTx = await provider.broadcastTransaction(ARACHNID_RAW_TX);
  const deployReceipt = await deployTx.wait();
  if (!deployReceipt || deployReceipt.status !== 1) {
    throw new Error(`Deterministic deployment proxy broadcast ${deployTx.hash} failed`);
  }
  console.log(`Broadcast deterministic deployment proxy: tx=${deployTx.hash} block=${deployReceipt.blockNumber}`);

  const finalStatus = await getDeterministicProxyCodeStatus(provider);
  if (finalStatus !== "match") {
    throw new Error(
      `Deterministic deployment proxy broadcast ${deployTx.hash} left ${finalStatus === "absent" ? "no code" : "unexpected code"} at ${ARACHNID_FACTORY}`,
    );
  }

  return {
    factoryAddress: ARACHNID_FACTORY,
    fundingTxHash: fundingTx.hash,
    deployTxHash: deployTx.hash,
    deployBlockNumber: deployReceipt.blockNumber,
    alreadyDeployed: false,
  };
}

export interface EnsureAndDescribeDeterministicProxyOutput {
  result: EnsureDeterministicProxyResult;
  /** Ready to hand to `awaitParentCheckpoint`/a checkpoint store. */
  record: DeploymentRecordInput;
  /** `formatDeploymentRecord(record)`, provided so callers don't reformat it themselves. */
  formattedRecord: string;
}

/**
 * Installs the proxy (see `ensureDeterministicDeploymentProxy`) and builds the
 * single canonical `DeploymentRecordInput`/log line describing the result, so
 * every entrypoint (local dev, quickstart, forge-deployer) reports the same
 * shape instead of each hand-formatting its own log line. On an "already
 * deployed" skip there is no real transaction hash, so `transactionHash`
 * falls back to `ZeroHash` — `formatDeploymentRecord`'s output always
 * includes a txHash field, matching `DEPLOYMENT_RECORD_PATTERN`.
 */
export async function ensureAndDescribeDeterministicDeploymentProxy(
  input: EnsureDeterministicProxyInput,
): Promise<EnsureAndDescribeDeterministicProxyOutput> {
  const result = await ensureDeterministicDeploymentProxy(input);
  const [blockNumber, network] = await Promise.all([
    result.deployBlockNumber ? Promise.resolve(result.deployBlockNumber) : input.provider.getBlockNumber(),
    input.provider.getNetwork(),
  ]);
  const record: DeploymentRecordInput = {
    contractName: DETERMINISTIC_DEPLOYMENT_PROXY_CONTRACT_NAME,
    address: result.factoryAddress,
    transactionHash: result.deployTxHash ?? ethers.ZeroHash,
    blockNumber,
    chainId: network.chainId,
  };
  return { result, record, formattedRecord: formatDeploymentRecord(record) };
}
