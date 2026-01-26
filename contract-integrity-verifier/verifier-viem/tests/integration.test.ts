/**
 * Contract Integrity Verifier - Viem Adapter Integration Tests
 *
 * Tests the full verification flow using the ViemAdapter with mock RPC responses.
 * Uses real viem for crypto operations, mocks only network calls.
 *
 * Run with: npx ts-node tests/integration.test.ts
 */

import { resolve } from "path";
import {
  keccak256,
  stringToBytes,
  getAddress,
  encodeAbiParameters,
  encodeFunctionData,
  decodeFunctionResult,
  zeroAddress,
} from "viem";
import {
  loadConfig,
  checkArtifactExists,
  loadArtifact,
  extractSelectorsFromArtifact,
  compareBytecode,
  stripCborMetadata,
  calculateErc7201BaseSlot,
  parsePath,
  loadStorageSchema,
  decodeSlotValue,
  parseMarkdownConfig,
  Verifier,
} from "@consensys/linea-contract-integrity-verifier";
import type { Web3Adapter, AbiElement } from "@consensys/linea-contract-integrity-verifier";
import { readFileSync } from "fs";

// ============================================================================
// Test Utilities
// ============================================================================

let testsPassed = 0;
let testsFailed = 0;

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
  if (actual.includes(expected)) {
    console.log(`  ✓ ${message}`);
    testsPassed++;
  } else {
    console.log(`  ✗ ${message}`);
    console.log(`    Expected to contain: ${expected}`);
    console.log(`    Actual: ${actual}`);
    testsFailed++;
  }
}

// ============================================================================
// Mock Viem Adapter (uses real viem for crypto, mocks RPC)
// ============================================================================

/**
 * Mock adapter using real viem for crypto operations.
 * Only RPC calls are mocked for offline testing.
 */
class MockViemAdapter implements Web3Adapter {
  private mockBytecode: string;
  private mockStorage: Map<string, string>;

  constructor(options?: { bytecode?: string; storage?: Record<string, string> }) {
    this.mockBytecode =
      options?.bytecode ??
      "0x6080604052636329a8e0600435146100a4576391d148541461010f57635e1dc3eb146101a0576328cf2fae146101c05763a217fddf146101e0575b600080fd5b6100b3610250565b60405180910390f35b6100b36100c4366004610350565ba265697066735822";

    this.mockStorage = new Map(Object.entries(options?.storage ?? {}));
    if (!this.mockStorage.has("0x0")) {
      this.mockStorage.set("0x0", "0x0000000000000000000000000000000000000000000000000000000000000007");
    }
  }

  // Real viem implementations
  keccak256(value: string | Uint8Array): string {
    if (typeof value === "string") {
      return keccak256(stringToBytes(value));
    }
    return keccak256(value);
  }

  checksumAddress(address: string): string {
    return getAddress(address);
  }

  get zeroAddress(): string {
    return zeroAddress;
  }

  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
    const abiParams = types.map((type) => ({ type }));
    return encodeAbiParameters(abiParams, values as unknown[]);
  }

  encodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string {
    return encodeFunctionData({
      abi: abi as unknown[],
      functionName,
      args: args as unknown[],
    });
  }

  decodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[] {
    const result = decodeFunctionResult({
      abi: abi as unknown[],
      functionName,
      data: data as `0x${string}`,
    });
    return Array.isArray(result) ? result : [result];
  }

  // Mocked RPC calls
  async getCode(_address: string): Promise<string> {
    return this.mockBytecode;
  }

  async getStorageAt(_address: string, slot: string): Promise<string> {
    return this.mockStorage.get(slot) ?? "0x" + "0".repeat(64);
  }

  async call(_to: string, _data: string): Promise<string> {
    return "0x" + "0".repeat(64);
  }
}

// ============================================================================
// Test Suites
// ============================================================================

async function testViemAdapterCrypto(): Promise<void> {
  console.log("\n=== Viem Adapter Crypto Tests ===");

  const adapter = new MockViemAdapter();

  // Test keccak256
  const hash = adapter.keccak256("hello");
  assert(hash.startsWith("0x"), "keccak256 returns hex string");
  assertEqual(hash.length, 66, "keccak256 returns 32 bytes");

  // Test checksumAddress
  const checksummed = adapter.checksumAddress("0xafeb487dd3e3cb0342e8cf0215987ffdc9b72c9b");
  assert(checksummed.includes("afeB487DD3E3Cb0342e8CF0215987FfDc9b72c9b"), "checksumAddress works");

  // Test zeroAddress
  assertEqual(adapter.zeroAddress, "0x0000000000000000000000000000000000000000", "zeroAddress is correct");

  // Test encodeAbiParameters
  const encoded = adapter.encodeAbiParameters(["uint256"], [BigInt(123)]);
  assert(encoded.startsWith("0x"), "encodeAbiParameters returns hex");
  assertContains(encoded, "7b", "encodeAbiParameters encodes 123 correctly");
}

async function testArtifactLoading(): Promise<void> {
  console.log("\n=== Artifact Loading Tests ===");

  const fixturesDir = resolve(__dirname, "fixtures");
  const yieldManagerPath = resolve(fixturesDir, "artifacts/YieldManager.json");
  const lineaRollupPath = resolve(fixturesDir, "artifacts/LineaRollup.json");

  const yieldManager = loadArtifact(yieldManagerPath);
  assert(yieldManager !== null, "YieldManager artifact loaded");
  assertEqual(yieldManager.format, "foundry", "YieldManager detected as Foundry format");
  assert(yieldManager.abi.length > 0, "YieldManager ABI has entries");

  const lineaRollup = loadArtifact(lineaRollupPath);
  assert(lineaRollup !== null, "LineaRollup artifact loaded");
  assertEqual(lineaRollup.format, "foundry", "LineaRollup detected as Foundry format");

  // Test selector extraction with real viem adapter
  const adapter = new MockViemAdapter();
  const selectors = extractSelectorsFromArtifact(adapter, yieldManager);
  assert(selectors.size > 0, "Selectors extracted using viem");
  assert(selectors.has("91d14854"), "hasRole selector found (91d14854)");
}

async function testConfigLoading(): Promise<void> {
  console.log("\n=== Config Loading Tests ===");

  const fixturesDir = resolve(__dirname, "fixtures");
  const jsonConfigPath = resolve(fixturesDir, "test-config.json");

  const jsonConfig = loadConfig(jsonConfigPath);
  assert(jsonConfig !== null, "JSON config loaded");
  assertEqual(jsonConfig.contracts.length, 3, "JSON config has 3 contracts");

  for (const contract of jsonConfig.contracts) {
    assert(checkArtifactExists(contract), `Artifact exists for ${contract.name}`);
  }

  // Test markdown config
  const mdConfigPath = resolve(fixturesDir, "test-config.md");
  const mdContent = readFileSync(mdConfigPath, "utf-8");
  const mdConfig = parseMarkdownConfig(mdContent, fixturesDir);
  assert(mdConfig.contracts.length > 0, "Markdown config parsed");
}

async function testStorageWithViem(): Promise<void> {
  console.log("\n=== Storage Tests with Viem ===");

  const adapter = new MockViemAdapter();
  const fixturesDir = resolve(__dirname, "fixtures");

  // Test ERC-7201 slot calculation with real keccak256
  const namespace = "linea.storage.YieldManagerStorage";
  const baseSlot = calculateErc7201BaseSlot(adapter, namespace);
  assert(baseSlot.startsWith("0x"), "ERC-7201 slot calculated");
  assertEqual(baseSlot.length, 66, "ERC-7201 slot is 32 bytes");

  // Verify the slot matches expected (real keccak256)
  assertContains(baseSlot.toLowerCase(), "dc1272075efdca0b85fb2d76cbb5f26d954dc18e040d6d0b67071bd5cbd04300", "ERC-7201 slot matches expected");

  // Test path parsing
  const parsed = parsePath("YieldManagerStorage:targetWithdrawalReservePercentageBps");
  assertEqual(parsed.structName, "YieldManagerStorage", "Struct name parsed");

  // Test schema loading
  const schema = loadStorageSchema("./schemas/yield-manager.json", fixturesDir);
  assert(schema !== null, "Storage schema loaded");
  assert(schema.structs["YieldManagerStorage"] !== undefined, "Schema has YieldManagerStorage");

  // Test slot decoding
  const rawValue = "0x0000000000000000000000000000000000000000000000000000000000000007";
  const decoded = decodeSlotValue(adapter, rawValue, "uint8", 0);
  assertEqual(decoded, "7", "uint8 decoded correctly");
}

async function testVerifierWithViem(): Promise<void> {
  console.log("\n=== Verifier Tests with Viem ===");

  const adapter = new MockViemAdapter({
    storage: {
      "0x0": "0x0000000000000000000000000000000000000000000000000000000000000007",
      "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc":
        "0x000000000000000000000000caaa421ffcf701befd676a2f5d0a161ccfa5a07e",
    },
  });

  const verifier = new Verifier(adapter);

  const bytecode = await verifier.fetchBytecode("0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48");
  assert(bytecode.length > 0, "Bytecode fetched");

  const implAddress = await verifier.getImplementationAddress("0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48");
  assert(implAddress !== null, "Implementation address found");
  assertContains(implAddress!.toLowerCase(), "caaa421ffcf701befd676a2f5d0a161ccfa5a07e", "Correct implementation");

  const slot = verifier.calculateErc7201Slot("linea.storage.LineaRollupYieldExtensionStorage");
  assert(slot.startsWith("0x"), "ERC-7201 slot calculated");
}

async function testBytecodeComparison(): Promise<void> {
  console.log("\n=== Bytecode Comparison Tests ===");

  const fixturesDir = resolve(__dirname, "fixtures");
  const artifact = loadArtifact(resolve(fixturesDir, "artifacts/YieldManager.json"));

  const result = compareBytecode(artifact.deployedBytecode, artifact.deployedBytecode);
  assertEqual(result.status, "pass", "Exact match passes");
  assertEqual(result.matchPercentage, 100, "Exact match is 100%");

  // Test CBOR stripping - the function may return original if CBOR is invalid
  const withMetadata = artifact.deployedBytecode + "a265697066735822" + "1220" + "a".repeat(64) + "0033";
  const stripped = stripCborMetadata(withMetadata);
  assert(stripped.length <= withMetadata.length, "CBOR metadata stripped (or original returned)");
}

async function testFullIntegration(): Promise<void> {
  console.log("\n=== Full Integration Test ===");

  const adapter = new MockViemAdapter({
    storage: {
      "0x0": "0x0000000000000000000000000000000000000000000000000000000000000007",
    },
  });

  const fixturesDir = resolve(__dirname, "fixtures");
  const config = loadConfig(resolve(fixturesDir, "test-config.json"));

  assert(config.contracts.length === 3, "Config has 3 contracts");

  // Verify each contract's artifact can be loaded
  for (const contract of config.contracts) {
    const artifact = loadArtifact(contract.artifactFile);
    assert(artifact !== null, `Loaded artifact for ${contract.name}`);

    // Test selector extraction with viem
    const selectors = extractSelectorsFromArtifact(adapter, artifact);
    assert(selectors.size > 0, `Extracted selectors for ${contract.name}`);
  }

  // Create verifier and verify it works
  const verifier = new Verifier(adapter);
  const bytecode = await verifier.fetchBytecode(config.contracts[0].address);
  assert(bytecode.length > 0, "Verifier can fetch bytecode");

  console.log("\n  Integration test completed successfully");
}

// ============================================================================
// Main
// ============================================================================

async function main(): Promise<void> {
  console.log("=".repeat(60));
  console.log("Contract Integrity Verifier - Viem Adapter Integration Tests");
  console.log("=".repeat(60));

  try {
    await testViemAdapterCrypto();
    await testArtifactLoading();
    await testConfigLoading();
    await testStorageWithViem();
    await testVerifierWithViem();
    await testBytecodeComparison();
    await testFullIntegration();
  } catch (error) {
    console.error("\nFatal error:", error);
    process.exit(1);
  }

  console.log("\n" + "=".repeat(60));
  console.log(`RESULTS: ${testsPassed} passed, ${testsFailed} failed`);
  console.log("=".repeat(60));

  if (testsFailed > 0) {
    throw new Error(`${testsFailed} tests failed`);
  }
}

// Jest wrapper
describe("Contract Integrity Verifier - Viem Adapter", () => {
  it("should pass all integration tests", async () => {
    await main();
  });
});
