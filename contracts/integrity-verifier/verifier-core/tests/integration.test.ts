/**
 * Contract Integrity Verifier - Integration Tests
 *
 * Tests the full verification flow using mock artifacts and configs.
 * Tests both JSON and Markdown configuration formats.
 * Uses a mock Web3Adapter for offline testing without RPC calls.
 *
 * Run with: npx ts-node tests/integration.test.ts
 */

import { resolve } from "path";
import { loadConfig, checkArtifactExists } from "../src/config";
import { loadArtifact, extractSelectorsFromArtifact } from "../src/utils/abi";
import {
  compareBytecode,
  extractSelectorsFromBytecode,
  stripCborMetadata,
  verifyImmutableValues,
} from "../src/utils/bytecode";
import { calculateErc7201BaseSlot, parsePath, loadStorageSchema, decodeSlotValue } from "../src/utils/storage";
import { parseMarkdownConfig } from "../src/utils/markdown-config";
import { Verifier } from "../src/verifier";
import type { Web3Adapter } from "../src/adapter";
import type { AbiElement } from "../src/types";
import { readFileSync } from "fs";

// ============================================================================
// Test Utilities
// ============================================================================

let testsPassed = 0;
let testsFailed = 0;

/**
 * Asserts a condition is true.
 */
function assert(condition: boolean, message: string): void {
  if (condition) {
    console.log(`  ✓ ${message}`);
    testsPassed++;
  } else {
    console.log(`  ✗ ${message}`);
    testsFailed++;
  }
}

/**
 * Asserts two values are equal.
 */
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

/**
 * Asserts a value contains a substring.
 */
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

/**
 * Asserts a function throws an error.
 */
async function assertThrows(fn: () => Promise<unknown> | unknown, message: string): Promise<void> {
  try {
    await fn();
    console.log(`  ✗ ${message} (expected error, but none thrown)`);
    testsFailed++;
  } catch {
    console.log(`  ✓ ${message}`);
    testsPassed++;
  }
}

// ============================================================================
// Mock Web3Adapter
// ============================================================================

/**
 * Mock Web3Adapter for offline testing.
 * Returns predictable values for testing purposes.
 */
class MockWeb3Adapter implements Web3Adapter {
  private mockBytecode: string;
  private mockStorage: Map<string, string>;
  private mockCallResults: Map<string, string>;

  constructor(options?: { bytecode?: string; storage?: Record<string, string>; callResults?: Record<string, string> }) {
    // Default mock bytecode with some realistic patterns
    this.mockBytecode =
      options?.bytecode ??
      "0x6080604052636329a8e0600435146100a4576391d148541461010f5763" +
        "5e1dc3eb146101a0576328cf2fae146101c05763a217fddf146101e05" +
        "75b600080fd5b6100b3610250565b60405180910390f35b6100b36100" +
        "c4366004610350565b600091825260209081526040808320938352929" +
        "052205460ff1690565ba265697066735822";

    this.mockStorage = new Map(Object.entries(options?.storage ?? {}));
    this.mockCallResults = new Map(Object.entries(options?.callResults ?? {}));

    // Set default storage values
    if (!this.mockStorage.has("0x0")) {
      this.mockStorage.set("0x0", "0x0000000000000000000000000000000000000000000000000000000000000007");
    }
  }

  /**
   * Computes keccak256 hash using a simple implementation.
   * For testing, we return a deterministic hash based on input.
   */
  keccak256(value: string | Uint8Array): string {
    // Simple hash for testing - in real use would use crypto library
    const input = typeof value === "string" ? value : Buffer.from(value).toString("hex");
    // Return a deterministic "hash" based on input length and first chars
    const hash =
      "0x" +
      input
        .slice(0, 64)
        .padEnd(64, "0")
        .split("")
        .map((c) => c.charCodeAt(0).toString(16).padStart(2, "0"))
        .join("")
        .slice(0, 64);
    return hash;
  }

  checksumAddress(address: string): string {
    // Simple checksum - just return with 0x prefix normalized
    return address.toLowerCase().startsWith("0x") ? address : "0x" + address;
  }

  get zeroAddress(): string {
    return "0x0000000000000000000000000000000000000000";
  }

  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
    // Simple encoding for testing
    let result = "0x";
    for (let i = 0; i < values.length; i++) {
      const val = values[i];
      if (typeof val === "bigint") {
        result += val.toString(16).padStart(64, "0");
      } else if (typeof val === "string" && val.startsWith("0x")) {
        result += val.slice(2).padStart(64, "0");
      } else {
        result += String(val).padStart(64, "0");
      }
    }
    return result;
  }

  encodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string {
    // Find function and return mock calldata
    const func = abi.find((e) => e.type === "function" && e.name === functionName);
    if (!func) {
      throw new Error(`Function ${functionName} not found in ABI`);
    }
    return "0x" + functionName.slice(0, 8).padEnd(8, "0") + (args?.length ? "00".repeat(32 * args.length) : "");
  }

  decodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[] {
    // Return mock result based on function name
    if (functionName === "CONTRACT_VERSION") {
      return ["7.0"];
    }
    if (functionName === "hasRole") {
      return [true];
    }
    if (functionName === "L1_MESSAGE_SERVICE") {
      return ["0x24B0E20c3Cec999C8A6723FCfC06d5c88fB4a056"];
    }
    return [data];
  }

  async getCode(_address: string): Promise<string> {
    return this.mockBytecode;
  }

  async getStorageAt(_address: string, slot: string): Promise<string> {
    return this.mockStorage.get(slot) ?? "0x" + "0".repeat(64);
  }

  async call(to: string, data: string): Promise<string> {
    const key = `${to}:${data.slice(0, 10)}`;
    return this.mockCallResults.get(key) ?? "0x" + "0".repeat(64);
  }
}

// ============================================================================
// Test Suites
// ============================================================================

/**
 * Tests for artifact loading and parsing.
 */
async function testArtifactLoading(): Promise<void> {
  console.log("\n=== Artifact Loading Tests ===");

  const fixturesDir = resolve(__dirname, "fixtures");
  const yieldManagerPath = resolve(fixturesDir, "artifacts/YieldManager.json");
  const lineaRollupPath = resolve(fixturesDir, "artifacts/LineaRollup.json");

  // Test 1: Load YieldManager artifact
  const yieldManager = loadArtifact(yieldManagerPath);
  assert(!!yieldManager, "YieldManager artifact loaded");
  assertEqual(yieldManager.format, "foundry", "YieldManager detected as Foundry format");
  assert(yieldManager.abi.length > 0, "YieldManager ABI has entries");
  assert(yieldManager.deployedBytecode.length > 0, "YieldManager has deployed bytecode");

  // Test 2: Load LineaRollup artifact
  const lineaRollup = loadArtifact(lineaRollupPath);
  assert(!!lineaRollup, "LineaRollup artifact loaded");
  assertEqual(lineaRollup.format, "foundry", "LineaRollup detected as Foundry format");
  assert(lineaRollup.abi.length > 0, "LineaRollup ABI has entries");

  // Test 3: Check immutable references
  assert(
    yieldManager.immutableReferences !== undefined && yieldManager.immutableReferences.length > 0,
    "YieldManager has immutable references",
  );
  assert(
    lineaRollup.immutableReferences !== undefined && lineaRollup.immutableReferences.length > 0,
    "LineaRollup has immutable references",
  );

  // Test 4: Check method identifiers
  assert(yieldManager.methodIdentifiers !== undefined, "YieldManager has method identifiers");
  assert(lineaRollup.methodIdentifiers !== undefined, "LineaRollup has method identifiers");

  // Test 5: Extract selectors
  const adapter = new MockWeb3Adapter();
  const yieldManagerSelectors = extractSelectorsFromArtifact(adapter, yieldManager);
  assert(yieldManagerSelectors.size > 0, "YieldManager selectors extracted");
  assert(yieldManagerSelectors.has("91d14854"), "YieldManager has hasRole selector");

  const lineaRollupSelectors = extractSelectorsFromArtifact(adapter, lineaRollup);
  assert(lineaRollupSelectors.size > 0, "LineaRollup selectors extracted");
  assert(lineaRollupSelectors.has("1c9b1ba7"), "LineaRollup has CONTRACT_VERSION selector");
}

/**
 * Tests for configuration loading.
 */
async function testConfigLoading(): Promise<void> {
  console.log("\n=== Config Loading Tests ===");

  const fixturesDir = resolve(__dirname, "fixtures");

  // Test 1: Load JSON config
  const jsonConfigPath = resolve(fixturesDir, "test-config.json");
  const jsonConfig = loadConfig(jsonConfigPath);
  assert(!!jsonConfig, "JSON config loaded");
  assert(Object.keys(jsonConfig.chains).length > 0, "JSON config has chains");
  assert(jsonConfig.contracts.length > 0, "JSON config has contracts");
  assertEqual(jsonConfig.contracts.length, 3, "JSON config has 3 contracts");

  // Test 2: Verify contract properties
  const yieldManagerContract = jsonConfig.contracts.find((c) => c.name === "TestYieldManager-Proxy");
  assert(yieldManagerContract !== undefined, "YieldManager contract found in config");
  assert(yieldManagerContract!.isProxy === true, "YieldManager marked as proxy");
  assert(yieldManagerContract!.stateVerification !== undefined, "YieldManager has state verification config");

  // Test 3: Check artifact paths are resolved
  for (const contract of jsonConfig.contracts) {
    assert(checkArtifactExists(contract), `Artifact exists for ${contract.name}`);
  }

  // Test 4: Load Markdown config
  const mdConfigPath = resolve(fixturesDir, "test-config.md");
  const mdContent = readFileSync(mdConfigPath, "utf-8");
  const mdConfig = parseMarkdownConfig(mdContent, fixturesDir);
  assert(!!mdConfig, "Markdown config loaded");
  assert(mdConfig.contracts.length > 0, "Markdown config has contracts");
  assertEqual(mdConfig.contracts.length, 3, "Markdown config has 3 contracts");

  // Test 5: Verify markdown parsed state verification
  const mdYieldManager = mdConfig.contracts.find((c) => c.name === "TestYieldManager-Proxy");
  assert(mdYieldManager !== undefined, "YieldManager found in MD config");
  assert(mdYieldManager!.stateVerification?.viewCalls !== undefined, "MD config has view calls");
  assert(mdYieldManager!.stateVerification!.viewCalls!.length > 0, "MD config view calls parsed");
}

/**
 * Tests for bytecode comparison.
 */
async function testBytecodeComparison(): Promise<void> {
  console.log("\n=== Bytecode Comparison Tests ===");

  const fixturesDir = resolve(__dirname, "fixtures");
  const yieldManagerPath = resolve(fixturesDir, "artifacts/YieldManager.json");
  const artifact = loadArtifact(yieldManagerPath);

  // Test 1: Exact match
  const result1 = compareBytecode(artifact.deployedBytecode, artifact.deployedBytecode);
  assertEqual(result1.status, "pass", "Exact bytecode match passes");
  assertEqual(result1.matchPercentage, 100, "Exact match is 100%");

  // Test 2: Different bytecode length
  const shorterBytecode = artifact.deployedBytecode.slice(0, 100);
  const result2 = compareBytecode(artifact.deployedBytecode, shorterBytecode);
  assertEqual(result2.status, "fail", "Length mismatch fails");

  // Test 3: Strip CBOR metadata
  // CBOR metadata format: a2 (2-item map) + content + 2-byte length at the end
  // The length at the end (0x0033 = 51 bytes) tells us how much metadata to strip
  const cborMetadata = "a265697066735822" + "1220" + "a".repeat(64) + "6473" + "6f6c63" + "43" + "0a0813" + "0033";
  const bytecodeWithMetadata = artifact.deployedBytecode + cborMetadata;
  const stripped = stripCborMetadata(bytecodeWithMetadata);
  // The stripping function should remove the metadata section
  assert(stripped.length <= artifact.deployedBytecode.length, "CBOR metadata stripped (or original returned)");

  // Test 4: Extract selectors from bytecode
  const selectors = extractSelectorsFromBytecode(artifact.deployedBytecode);
  assert(Array.isArray(selectors), "Selectors is an array");
  // Note: selector extraction is heuristic and may not find all selectors in mock bytecode
}

/**
 * Tests for immutable values verification.
 */
async function testImmutableValues(): Promise<void> {
  console.log("\n=== Immutable Values Tests ===");

  // Mock immutable differences (as would be detected from bytecode comparison)
  const mockImmutableDifferences = [
    {
      position: 100,
      length: 32,
      localValue: "0".repeat(64),
      remoteValue: "000000000000000000000000d19d4b5d358258f05d7b411e21a1460d11b0876f",
      possibleType: "address",
    },
    {
      position: 200,
      length: 32,
      localValue: "0".repeat(64),
      remoteValue: "00000000000000000000000073bf00ad18c7c0871eba03bcbef8c98225f9ceaa",
      possibleType: "address",
    },
    {
      position: 300,
      length: 32,
      localValue: "0".repeat(64),
      remoteValue: "0000000000000000000000000000000000000000000000000000000000000064",
      possibleType: "uint256",
    },
  ];

  // Test 1: All immutable values match
  const immutableValues1 = {
    L1_MESSAGE_SERVICE: "0xd19d4b5d358258f05d7b411e21a1460d11b0876f",
    YIELD_MANAGER: "0x73bf00ad18c7c0871eba03bcbef8c98225f9ceaa",
    MAX_AMOUNT: 100, // 0x64 in hex
  };

  const result1 = verifyImmutableValues(immutableValues1, mockImmutableDifferences);
  assertEqual(result1.status, "pass", "All immutable values match");
  assertEqual(result1.results.length, 3, "Three results returned");
  assert(
    result1.results.every((r) => r.status === "pass"),
    "All individual results pass",
  );

  // Test 2: Partial match (one missing)
  const immutableValues2 = {
    L1_MESSAGE_SERVICE: "0xd19d4b5d358258f05d7b411e21a1460d11b0876f",
    YIELD_MANAGER: "0x73bf00ad18c7c0871eba03bcbef8c98225f9ceaa",
    WRONG_VALUE: "0x1234567890123456789012345678901234567890",
  };

  const result2 = verifyImmutableValues(immutableValues2, mockImmutableDifferences);
  assertEqual(result2.status, "fail", "Partial match fails");
  assertEqual(result2.results.filter((r) => r.status === "pass").length, 2, "Two results pass");
  assertEqual(result2.results.filter((r) => r.status === "fail").length, 1, "One result fails");

  // Test 3: Match with bigint
  const immutableValues3 = {
    MAX_AMOUNT: BigInt(100),
  };

  const result3 = verifyImmutableValues(immutableValues3, mockImmutableDifferences);
  assertEqual(result3.status, "pass", "BigInt value matches");

  // Test 4: Match with boolean
  const boolImmutableDifferences = [
    {
      position: 100,
      length: 32,
      localValue: "0".repeat(64),
      remoteValue: "0".repeat(63) + "1",
      possibleType: "bool",
    },
  ];

  const immutableValues4 = {
    IS_ENABLED: true,
  };

  const result4 = verifyImmutableValues(immutableValues4, boolImmutableDifferences);
  assertEqual(result4.status, "pass", "Boolean true value matches");

  // Test 5: Empty immutable values
  const result5 = verifyImmutableValues({}, mockImmutableDifferences);
  assertEqual(result5.status, "pass", "Empty immutable values passes (nothing to verify)");
  assertEqual(result5.results.length, 0, "No results for empty config");

  // Test 6: Fragment matching (addresses split due to matching bytes)
  // This simulates the case where YIELD_MANAGER 0x73bf00ad18c7c0871eba03bcbef8c98225f9ceaa
  // is split into "73bf" and "ad18c7c0871eba03bcbef8c98225f9ceaa" because of the "00" byte
  const fragmentedImmutableDifferences = [
    {
      position: 262,
      length: 2,
      localValue: "0000",
      remoteValue: "73bf",
      possibleType: "uint16",
    },
    {
      position: 265,
      length: 17,
      localValue: "0".repeat(34),
      remoteValue: "ad18c7c0871eba03bcbef8c98225f9ceaa",
      possibleType: undefined,
    },
    {
      position: 300,
      length: 20,
      localValue: "0".repeat(40),
      remoteValue: "d19d4b5d358258f05d7b411e21a1460d11b0876f",
      possibleType: "address",
    },
  ];

  const immutableValues6 = {
    YIELD_MANAGER: "0x73bf00ad18c7c0871eba03bcbef8c98225f9ceaa",
    L1_MESSAGE_SERVICE: "0xd19d4b5d358258f05d7b411e21a1460d11b0876f",
  };

  const result6 = verifyImmutableValues(immutableValues6, fragmentedImmutableDifferences);
  assertEqual(result6.status, "pass", "Fragment matching passes for split addresses");
  assertEqual(result6.results.filter((r) => r.status === "pass").length, 2, "Both fragmented values matched");
}

/**
 * Tests for storage utilities.
 */
async function testStorageUtilities(): Promise<void> {
  console.log("\n=== Storage Utilities Tests ===");

  const adapter = new MockWeb3Adapter();
  const fixturesDir = resolve(__dirname, "fixtures");

  // Test 1: Calculate ERC-7201 base slot
  const namespace = "linea.storage.YieldManagerStorage";
  const baseSlot = calculateErc7201BaseSlot(adapter, namespace);
  assert(baseSlot.startsWith("0x"), "ERC-7201 slot is hex string");
  assertEqual(baseSlot.length, 66, "ERC-7201 slot is 32 bytes (66 chars with 0x)");

  // Test 2: Parse storage path
  const parsed = parsePath("YieldManagerStorage:targetWithdrawalReservePercentageBps");
  assertEqual(parsed.structName, "YieldManagerStorage", "Struct name parsed correctly");
  assert(parsed.segments.length > 0, "Path segments parsed");

  // Test 3: Load storage schema
  const schema = loadStorageSchema("../../examples/schemas/yield-manager.json", fixturesDir);
  assert(schema !== null, "Storage schema loaded");
  assert(schema.structs !== undefined, "Schema has structs");
  assert(schema.structs["YieldManagerStorage"] !== undefined, "Schema has YieldManagerStorage");

  // Test 4: Decode slot value
  const rawValue = "0x0000000000000000000000000000000000000000000000000000000000000007";
  const decoded = decodeSlotValue(adapter, rawValue, "uint8", 0);
  assertEqual(decoded, "7", "uint8 slot value decoded correctly");

  // Test 5: Decode address
  const addressValue = "0x000000000000000000000000afeb487dd3e3cb0342e8cf0215987ffdc9b72c9b";
  const decodedAddress = decodeSlotValue(adapter, addressValue, "address", 0);
  assertContains(String(decodedAddress).toLowerCase(), "afeb487dd3e3cb0342e8cf0215987ffdc9b72c9b", "Address decoded");

  // Test 6: Parse path with array access
  await assertThrows(() => parsePath(""), "Empty path throws error");
  await assertThrows(() => parsePath("NoColon"), "Path without colon throws error");
}

/**
 * Tests for the Verifier class.
 */
async function testVerifier(): Promise<void> {
  console.log("\n=== Verifier Class Tests ===");

  const adapter = new MockWeb3Adapter({
    storage: {
      "0x0": "0x0000000000000000000000000000000000000000000000000000000000000007",
      // EIP-1967 implementation slot
      "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc":
        "0x000000000000000000000000caaa421ffcf701befd676a2f5d0a161ccfa5a07e",
    },
  });

  const verifier = new Verifier(adapter);

  // Test 1: Fetch bytecode
  const bytecode = await verifier.fetchBytecode("0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48");
  assert(bytecode.length > 0, "Bytecode fetched");
  assert(bytecode.startsWith("0x"), "Bytecode is hex string");

  // Test 2: Get implementation address for proxy
  const implAddress = await verifier.getImplementationAddress("0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48");
  assert(implAddress !== null, "Implementation address found");
  assertContains(implAddress!.toLowerCase(), "caaa421ffcf701befd676a2f5d0a161ccfa5a07e", "Correct implementation");

  // Test 3: Calculate ERC-7201 slot
  const slot = verifier.calculateErc7201Slot("linea.storage.LineaRollupYieldExtensionStorage");
  assert(slot.startsWith("0x"), "ERC-7201 slot calculated");
  assertEqual(slot.length, 66, "Slot is 32 bytes");
}

/**
 * Integration test using both JSON and MD configs.
 */
async function testFullIntegration(): Promise<void> {
  console.log("\n=== Full Integration Tests ===");

  const adapter = new MockWeb3Adapter({
    storage: {
      "0x0": "0x0000000000000000000000000000000000000000000000000000000000000007",
      "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc":
        "0x000000000000000000000000caaa421ffcf701befd676a2f5d0a161ccfa5a07e",
    },
  });

  const fixturesDir = resolve(__dirname, "fixtures");

  // Test 1: Load JSON config and verify structure
  const jsonConfig = loadConfig(resolve(fixturesDir, "test-config.json"));
  assert(jsonConfig.contracts.length === 3, "JSON config has expected contract count");

  // Test 2: Load MD config and verify structure
  const mdContent = readFileSync(resolve(fixturesDir, "test-config.md"), "utf-8");
  const mdConfig = parseMarkdownConfig(mdContent, fixturesDir);
  assert(mdConfig.contracts.length === 3, "MD config has expected contract count");

  // Test 3: Verify both configs have same contracts (by name)
  const jsonNames = jsonConfig.contracts.map((c) => c.name).sort();
  const mdNames = mdConfig.contracts.map((c) => c.name).sort();
  assertEqual(jsonNames, mdNames, "JSON and MD configs have same contract names");

  // Test 4: Verify artifacts can be loaded from configs
  for (const contract of jsonConfig.contracts) {
    const artifact = loadArtifact(contract.artifactFile);
    assert(!!artifact, `Artifact loaded for ${contract.name} from JSON config`);
  }

  // Test 5: Create verifier and verify contract structure
  new Verifier(adapter); // Verify Verifier can be instantiated
  const lineaRollup = jsonConfig.contracts.find((c) => c.name === "TestLineaRollup-Proxy");
  assert(lineaRollup !== undefined, "LineaRollup found in config");

  // Test 6: Verify bytecode comparison works with loaded artifact
  const artifact = loadArtifact(lineaRollup!.artifactFile);
  const bytecodeResult = compareBytecode(artifact.deployedBytecode, artifact.deployedBytecode);
  assertEqual(bytecodeResult.status, "pass", "Bytecode self-comparison passes");

  console.log("\n  Integration test completed successfully");
}

// ============================================================================
// Main
// ============================================================================

async function main(): Promise<void> {
  console.log("=".repeat(60));
  console.log("Contract Integrity Verifier - Integration Tests");
  console.log("=".repeat(60));

  try {
    await testArtifactLoading();
    await testConfigLoading();
    await testBytecodeComparison();
    await testImmutableValues();
    await testStorageUtilities();
    await testVerifier();
    await testFullIntegration();
  } catch (error) {
    console.error("\nFatal error:", error);
    process.exit(1);
  }

  console.log("\n" + "=".repeat(60));
  console.log(`RESULTS: ${testsPassed} passed, ${testsFailed} failed`);
  console.log("=".repeat(60));

  if (testsFailed > 0) {
    process.exit(1);
  }
}

main();
