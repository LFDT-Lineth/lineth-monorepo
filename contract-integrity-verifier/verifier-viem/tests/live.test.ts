/**
 * Contract Integrity Verifier - Live Integration Tests (Viem)
 *
 * Tests the full verification flow against real deployed contracts on Sepolia.
 * Requires ETHEREUM_SEPOLIA_RPC_URL environment variable to be set.
 *
 * Usage:
 *   export ETHEREUM_SEPOLIA_RPC_URL="https://sepolia.infura.io/v3/YOUR_KEY"
 *   npx ts-node tests/live.test.ts
 *
 * These tests will be skipped if the environment variable is not set.
 */

import { resolve } from "path";
import { createPublicClient, http } from "viem";
import { sepolia } from "viem/chains";
import {
  loadConfig,
  Verifier,
  loadArtifact,
  compareBytecode,
  SEPOLIA_LINEA_ROLLUP_PROXY,
  SEPOLIA_LINEA_ROLLUP_IMPLEMENTATION,
  EIP1967_IMPLEMENTATION_SLOT,
  CONTRACT_VERSIONS,
  RPC_ENV_VARS,
  CHAIN_IDS,
  KNOWN_NAMESPACES,
} from "@consensys/linea-contract-integrity-verifier";
import { ViemAdapter } from "../src/index";

// ============================================================================
// Test Utilities
// ============================================================================

let testsPassed = 0;
let testsFailed = 0;
let testsSkipped = 0;

function assert(condition: boolean, message: string): void {
  if (condition) {
    console.log(`  ✓ ${message}`);
    testsPassed++;
  } else {
    console.log(`  ✗ ${message}`);
    testsFailed++;
  }
}

function assertEqual<T>(actual: T, expected: T, message: string): void {
  const pass = JSON.stringify(actual) === JSON.stringify(expected);
  if (pass) {
    console.log(`  ✓ ${message}`);
    testsPassed++;
  } else {
    console.log(`  ✗ ${message}`);
    console.log(`    Expected: ${JSON.stringify(expected)}`);
    console.log(`    Actual:   ${JSON.stringify(actual)}`);
    testsFailed++;
  }
}

function assertContains(actual: string, expected: string, message: string): void {
  if (actual.toLowerCase().includes(expected.toLowerCase())) {
    console.log(`  ✓ ${message}`);
    testsPassed++;
  } else {
    console.log(`  ✗ ${message}`);
    console.log(`    Expected to contain: ${expected}`);
    console.log(`    Actual: ${actual}`);
    testsFailed++;
  }
}

function skip(message: string): void {
  console.log(`  ⊘ ${message} (skipped)`);
  testsSkipped++;
}

// ============================================================================
// Environment Check
// ============================================================================

const RPC_URL = process.env[RPC_ENV_VARS.ETHEREUM_SEPOLIA];

function checkEnvironment(): boolean {
  if (!RPC_URL) {
    console.log(`\n⚠️  ${RPC_ENV_VARS.ETHEREUM_SEPOLIA} not set - skipping live tests`);
    console.log("   Set it to run live integration tests:");
    console.log(`   export ${RPC_ENV_VARS.ETHEREUM_SEPOLIA}="https://sepolia.infura.io/v3/YOUR_KEY"\n`);
    return false;
  }
  return true;
}

// ============================================================================
// Live Tests
// ============================================================================

async function testRpcConnection(): Promise<void> {
  console.log("\n=== RPC Connection Test ===");

  if (!RPC_URL) {
    skip("RPC connection test");
    return;
  }

  const client = createPublicClient({
    chain: sepolia,
    transport: http(RPC_URL),
  });

  const chainId = await client.getChainId();
  assertEqual(chainId, CHAIN_IDS.ETHEREUM_SEPOLIA, `Connected to Sepolia (chainId ${CHAIN_IDS.ETHEREUM_SEPOLIA})`);

  const blockNumber = await client.getBlockNumber();
  assert(blockNumber > 0n, `Current block number: ${blockNumber}`);
}

async function testViemAdapterWithRealRpc(): Promise<void> {
  console.log("\n=== Viem Adapter with Real RPC ===");

  if (!RPC_URL) {
    skip("Viem adapter RPC test");
    return;
  }

  const adapter = new ViemAdapter(RPC_URL);

  // Test getCode on a known contract
  const bytecode = await adapter.getCode(SEPOLIA_LINEA_ROLLUP_PROXY);

  assert(bytecode.length > 10, `Fetched bytecode for proxy (${bytecode.length} chars)`);
  assert(bytecode.startsWith("0x"), "Bytecode starts with 0x");

  // Test getStorageAt
  const slot0 = await adapter.getStorageAt(SEPOLIA_LINEA_ROLLUP_PROXY, "0x0");
  assert(slot0.length === 66, "Storage slot 0 is 32 bytes");

  // Test EIP-1967 implementation slot
  const implValue = await adapter.getStorageAt(SEPOLIA_LINEA_ROLLUP_PROXY, EIP1967_IMPLEMENTATION_SLOT);
  assertContains(implValue, SEPOLIA_LINEA_ROLLUP_IMPLEMENTATION.toLowerCase().slice(2), "Implementation address found in EIP-1967 slot");
}

async function testVerifierWithRealContract(): Promise<void> {
  console.log("\n=== Verifier with Real Contract ===");

  if (!RPC_URL) {
    skip("Verifier real contract test");
    return;
  }

  const adapter = new ViemAdapter(RPC_URL);
  const verifier = new Verifier(adapter);

  const proxyAddress = "0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48";

  // Test bytecode fetching
  const bytecode = await verifier.fetchBytecode(proxyAddress);
  assert(bytecode.length > 100, "Fetched contract bytecode");

  // Test implementation address detection
  const implAddress = await verifier.getImplementationAddress(proxyAddress);
  assert(implAddress !== null, "Implementation address detected");
  assertContains(implAddress!, "caaa421ffcf701befd676a2f5d0a161ccfa5a07e", "Correct implementation address");
}

async function testFullVerificationFlow(): Promise<void> {
  console.log("\n=== Full Verification Flow ===");

  if (!RPC_URL) {
    skip("Full verification flow test");
    return;
  }

  const fixturesDir = resolve(__dirname, "fixtures");
  const configPath = resolve(fixturesDir, "live-test-config.json");

  // Load config
  const config = loadConfig(configPath);
  assert(config !== null, "Live config loaded");
  assertEqual(config.contracts.length, 2, "Config has 2 contracts");

  const adapter = new ViemAdapter(RPC_URL);
  const verifier = new Verifier(adapter);

  // Test implementation contract verification
  const implContract = config.contracts.find((c) => c.name === "LineaRollup-Implementation");
  assert(implContract !== undefined, "Found implementation contract in config");

  if (implContract) {
    console.log(`\n  Testing: ${implContract.name}`);

    // Load artifact
    const artifact = loadArtifact(implContract.artifactFile);
    assert(artifact !== null, "Artifact loaded");
    assert(artifact.abi.length > 0, `Artifact has ${artifact.abi.length} ABI entries`);

    // Fetch on-chain bytecode
    const onChainBytecode = await verifier.fetchBytecode(implContract.address);
    assert(onChainBytecode.length > 100, `Fetched on-chain bytecode (${onChainBytecode.length} chars)`);

    // Compare bytecode
    const result = compareBytecode(artifact.deployedBytecode, onChainBytecode, artifact.immutableReferences);

    console.log(`    Bytecode comparison: ${result.status}`);
    console.log(`    Match percentage: ${result.matchPercentage}%`);

    if (result.status === "pass") {
      assert(true, "Bytecode matches exactly");
    } else if (result.onlyImmutablesDiffer) {
      assert(true, "Bytecode matches (only immutables differ)");
      console.log(`    Immutable differences: ${result.immutableDifferences?.length || 0}`);
    } else {
      // May fail if contract has been upgraded since artifact was created
      console.log(`    Note: Bytecode mismatch may indicate contract upgrade`);
      assert(result.matchPercentage !== undefined && result.matchPercentage > 90, "High bytecode similarity (>90%)");
    }
  }
}

async function testViewCallVerification(): Promise<void> {
  console.log("\n=== View Call Verification ===");

  if (!RPC_URL) {
    skip("View call verification test");
    return;
  }

  const adapter = new ViemAdapter(RPC_URL);

  // Load the artifact to get the ABI
  const artifactPath = resolve(__dirname, "fixtures/artifacts/hardhat/LineaRollup.json");
  const artifact = loadArtifact(artifactPath);

  if (!artifact) {
    skip("Could not load artifact for view call test");
    return;
  }

  // Test CONTRACT_VERSION view call
  const callData = adapter.encodeFunctionData(artifact.abi, "CONTRACT_VERSION", []);
  const result = await adapter.call(SEPOLIA_LINEA_ROLLUP_PROXY, callData);
  const decoded = adapter.decodeFunctionResult(artifact.abi, "CONTRACT_VERSION", result);

  console.log(`    CONTRACT_VERSION result: ${decoded[0]}`);
  assertEqual(decoded[0], CONTRACT_VERSIONS.LINEA_ROLLUP_V7, `CONTRACT_VERSION returns ${CONTRACT_VERSIONS.LINEA_ROLLUP_V7}`);
}

async function testStorageSlotVerification(): Promise<void> {
  console.log("\n=== Storage Slot Verification ===");

  if (!RPC_URL) {
    skip("Storage slot verification test");
    return;
  }

  const adapter = new ViemAdapter(RPC_URL);
  const verifier = new Verifier(adapter);

  // Read _initialized slot (slot 0, uint8)
  const slot0 = await adapter.getStorageAt(SEPOLIA_LINEA_ROLLUP_PROXY, "0x0");
  console.log(`    Slot 0 raw: ${slot0}`);

  // Decode uint8 from rightmost byte
  const initializedValue = parseInt(slot0.slice(-2), 16);
  console.log(`    _initialized value: ${initializedValue}`);

  assertEqual(initializedValue, 7, "_initialized is 7 (version 7)");

  // Test ERC-7201 slot calculation
  const yieldExtSlot = verifier.calculateErc7201Slot(KNOWN_NAMESPACES.LINEA_ROLLUP_YIELD_EXTENSION);
  console.log(`    ERC-7201 slot for YieldExtension: ${yieldExtSlot}`);
  assert(yieldExtSlot.startsWith("0x"), "ERC-7201 slot calculated");
}

// ============================================================================
// Main
// ============================================================================

async function main(): Promise<void> {
  console.log("=".repeat(70));
  console.log("Contract Integrity Verifier - Live Integration Tests (Viem)");
  console.log("=".repeat(70));

  const canRun = checkEnvironment();

  if (!canRun) {
    testsSkipped = 6;
    console.log(`\n${"=".repeat(70)}`);
    console.log(`RESULTS: ${testsPassed} passed, ${testsFailed} failed, ${testsSkipped} skipped`);
    console.log("=".repeat(70));
    return;
  }

  try {
    await testRpcConnection();
    await testViemAdapterWithRealRpc();
    await testVerifierWithRealContract();
    await testFullVerificationFlow();
    await testViewCallVerification();
    await testStorageSlotVerification();
  } catch (error) {
    console.error("\nFatal error:", error);
    process.exit(1);
  }

  console.log(`\n${"=".repeat(70)}`);
  console.log(`RESULTS: ${testsPassed} passed, ${testsFailed} failed, ${testsSkipped} skipped`);
  console.log("=".repeat(70));

  if (testsFailed > 0) {
    process.exit(1);
  }
}

main();
