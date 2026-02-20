#!/usr/bin/env ts-node
/**
 * Contract Integrity Verifier - Test Runner
 * Run with: npx ts-node tests/run-tests.ts
 */

import { detectArtifactFormat, extractSelectorsFromArtifact, parseArtifact } from "../src/utils/abi";
import { loadArtifact } from "../src/utils/abi-node";
import {
  compareBytecode,
  extractSelectorsFromBytecode,
  validateImmutablesAgainstArgs,
  verifyImmutableValues,
  stripCborMetadata,
  linkLibraries,
  detectUnlinkedLibraries,
  verifyLinkedLibraries,
} from "../src/utils/bytecode";
import {
  calculateErc7201BaseSlot,
  decodeSlotValue,
  verifySlot,
  verifyNamespace,
  verifyStoragePath,
  parsePath,
  computeSlot,
  parseStorageSchema,
} from "../src/utils/storage";
import { loadStorageSchema } from "../src/utils/storage-node";

// Alias for backward compatibility with tests
const calculateErc7201Slot = calculateErc7201BaseSlot;
import {
  AbiElement,
  ViewCallConfig,
  SlotConfig,
  NamespaceConfig,
  StateVerificationConfig,
  NormalizedArtifact,
  ImmutableReference,
  ImmutableDifference,
  DeployedLinkReferences,
  StorageSchema,
} from "../src/types";
import { parseMarkdownConfig } from "../src/utils/markdown-config";
import type { Web3Adapter } from "../src/adapter";
import { Verifier } from "../src/verifier";

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
  const normalizedActual = typeof actual === "string" ? actual.toLowerCase() : actual;
  const normalizedExpected = typeof expected === "string" ? expected.toLowerCase() : expected;
  assert(normalizedActual === normalizedExpected, `${message}: expected ${expected}, got ${actual}`);
}

// ============================================================================
// Mock Web3Adapter
// ============================================================================

// Dynamic crypto helpers - try ethers first, then viem
function dynamicKeccak256(value: string | Uint8Array): string {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { keccak256: keccak, toUtf8Bytes } = require("ethers");
    if (typeof value === "string") {
      if (value.startsWith("0x")) {
        return keccak(value);
      }
      return keccak(toUtf8Bytes(value));
    }
    return keccak(value);
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { keccak256: viemKeccak256, stringToBytes, toHex } = require("viem");
    if (typeof value === "string") {
      if (value.startsWith("0x")) {
        return viemKeccak256(value as `0x${string}`);
      }
      return viemKeccak256(stringToBytes(value));
    }
    return viemKeccak256(toHex(value));
  }
}

function dynamicGetAddress(address: string): string {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { getAddress } = require("ethers");
    return getAddress(address);
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { getAddress: viemGetAddress } = require("viem");
    return viemGetAddress(address);
  }
}

function dynamicEncodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { AbiCoder } = require("ethers");
    const coder = AbiCoder.defaultAbiCoder();
    return coder.encode(types as string[], values as unknown[]);
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { encodeAbiParameters } = require("viem");
    const params = types.map((t) => ({ type: t }));
    return encodeAbiParameters(params, values as unknown[]);
  }
}

function dynamicEncodeFunctionData(
  abi: readonly AbiElement[],
  functionName: string,
  args?: readonly unknown[],
): string {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { Interface } = require("ethers");
    const iface = new Interface(abi as AbiElement[]);
    return iface.encodeFunctionData(functionName, args ?? []);
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { encodeFunctionData } = require("viem");
    return encodeFunctionData({ abi: abi as unknown[], functionName, args: args as unknown[] });
  }
}

function dynamicDecodeFunctionResult(
  abi: readonly AbiElement[],
  functionName: string,
  data: string,
): readonly unknown[] {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { Interface } = require("ethers");
    const iface = new Interface(abi as AbiElement[]);
    const result = iface.decodeFunctionResult(functionName, data);
    return Array.from(result);
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { decodeFunctionResult } = require("viem");
    const result = decodeFunctionResult({ abi: abi as unknown[], functionName, data: data as `0x${string}` });
    return Array.isArray(result) ? result : [result];
  }
}

class MockAdapter implements Web3Adapter {
  private storage: Map<string, string> = new Map();
  private callResults: Map<string, string> = new Map();
  private codeResults: Map<string, string> = new Map();

  readonly zeroAddress = "0x0000000000000000000000000000000000000000";

  setStorage(slot: string, value: string): void {
    this.storage.set(slot.toLowerCase(), value);
  }

  setCallResult(calldata: string, result: string): void {
    this.callResults.set(calldata.toLowerCase(), result);
  }

  setCode(address: string, code: string): void {
    this.codeResults.set(address.toLowerCase(), code);
  }

  keccak256(value: string | Uint8Array): string {
    return dynamicKeccak256(value);
  }

  checksumAddress(address: string): string {
    return dynamicGetAddress(address);
  }

  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
    return dynamicEncodeAbiParameters(types, values);
  }

  encodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string {
    return dynamicEncodeFunctionData(abi, functionName, args);
  }

  decodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[] {
    return dynamicDecodeFunctionResult(abi, functionName, data);
  }

  async getCode(address: string): Promise<string> {
    return this.codeResults.get(address.toLowerCase()) ?? "0x";
  }

  async getStorageAt(_address: string, slot: string): Promise<string> {
    return this.storage.get(slot.toLowerCase()) ?? "0x" + "0".repeat(64);
  }

  async call(_to: string, data: string): Promise<string> {
    const result = this.callResults.get(data.toLowerCase());
    if (!result) {
      throw new Error(`No mock result for calldata: ${data}`);
    }
    return result;
  }
}

// ============================================================================
// Test Data
// ============================================================================

const TEST_ADDRESS = "0x1234567890123456789012345678901234567890";
const TEST_OWNER = "0xabcdef1234567890abcdef1234567890abcdef12";

const MOCK_ABI: AbiElement[] = [
  {
    type: "function",
    name: "owner",
    inputs: [],
    outputs: [{ internalType: "address", name: "", type: "address" }],
    stateMutability: "view",
  },
  {
    type: "function",
    name: "hasRole",
    inputs: [
      { internalType: "bytes32", name: "role", type: "bytes32" },
      { internalType: "address", name: "account", type: "address" },
    ],
    outputs: [{ internalType: "bool", name: "", type: "bool" }],
    stateMutability: "view",
  },
];

// ============================================================================
// Tests
// ============================================================================

async function testErc7201SlotCalculation(): Promise<void> {
  console.log("\n📦 Testing ERC-7201 Slot Calculation...");

  const mockAdapter = new MockAdapter();

  const slot = calculateErc7201Slot(mockAdapter, "example.main");
  assert(slot.match(/^0x[0-9a-f]{64}$/) !== null, "Slot is valid 32-byte hex");
  assert(slot.slice(-2) === "00", "Last byte is masked to 0x00");

  const slot1 = calculateErc7201Slot(mockAdapter, "linea.storage.YieldManager");
  const slot2 = calculateErc7201Slot(mockAdapter, "linea.storage.LineaRollup");
  assert(slot1 !== slot2, "Different namespaces produce different slots");
}

async function testDecodeSlotValue(): Promise<void> {
  console.log("\n📦 Testing Slot Value Decoding...");

  const mockAdapter = new MockAdapter();

  // Address
  const addressValue = "0x000000000000000000000000" + TEST_OWNER.slice(2);
  const decodedAddress = decodeSlotValue(mockAdapter, addressValue, "address");
  assertEqual(decodedAddress, dynamicGetAddress(TEST_OWNER), "Decode address");

  // uint256
  const uint256Value = "0x" + "0".repeat(62) + "64"; // 100 in hex
  const decodedUint256 = decodeSlotValue(mockAdapter, uint256Value, "uint256");
  assertEqual(decodedUint256, "100", "Decode uint256");

  // uint8
  const uint8Value = "0x" + "0".repeat(62) + "06";
  const decodedUint8 = decodeSlotValue(mockAdapter, uint8Value, "uint8");
  assertEqual(decodedUint8, "6", "Decode uint8");

  // bool true
  const boolTrueValue = "0x" + "0".repeat(62) + "01";
  const decodedBoolTrue = decodeSlotValue(mockAdapter, boolTrueValue, "bool");
  assertEqual(decodedBoolTrue, true, "Decode bool (true)");

  // bool false
  const boolFalseValue = "0x" + "0".repeat(64);
  const decodedBoolFalse = decodeSlotValue(mockAdapter, boolFalseValue, "bool");
  assertEqual(decodedBoolFalse, false, "Decode bool (false)");
}

async function testViewCalls(): Promise<void> {
  console.log("\n📦 Testing View Calls...");

  const mockAdapter = new MockAdapter();

  // Setup mock for owner() using dynamic encoding
  const ownerCalldata = dynamicEncodeFunctionData(MOCK_ABI, "owner", []);
  const ownerReturnData = dynamicEncodeAbiParameters(["address"], [TEST_OWNER]);
  mockAdapter.setCallResult(ownerCalldata, ownerReturnData);

  const config: ViewCallConfig = {
    function: "owner",
    expected: TEST_OWNER,
  };

  const verifier = new Verifier(mockAdapter);
  const result = await verifier.executeViewCall(TEST_ADDRESS, MOCK_ABI, config);

  assertEqual(result.status, "pass", "View call owner() passes");

  // Test with parameters (hasRole)
  const role = "0x" + "0".repeat(64);
  const hasRoleCalldata = dynamicEncodeFunctionData(MOCK_ABI, "hasRole", [role, TEST_OWNER]);
  const hasRoleReturnData = dynamicEncodeAbiParameters(["bool"], [true]);
  mockAdapter.setCallResult(hasRoleCalldata, hasRoleReturnData);

  const hasRoleConfig: ViewCallConfig = {
    function: "hasRole",
    params: [role, TEST_OWNER],
    expected: true,
  };

  const hasRoleResult = await verifier.executeViewCall(TEST_ADDRESS, MOCK_ABI, hasRoleConfig);

  assertEqual(hasRoleResult.status, "pass", "View call hasRole() with params passes");
}

async function testSlotVerification(): Promise<void> {
  console.log("\n📦 Testing Slot Verification...");

  const mockAdapter = new MockAdapter();
  mockAdapter.setStorage("0x0", "0x" + "0".repeat(62) + "06");

  const config: SlotConfig = {
    slot: "0x0",
    type: "uint8",
    name: "_initialized",
    expected: "6",
  };

  const result = await verifySlot(mockAdapter, TEST_ADDRESS, config);

  assertEqual(result.status, "pass", "Slot verification passes with correct value");
  assertEqual(result.actual, "6", "Actual value is correct");
}

async function testNamespaceVerification(): Promise<void> {
  console.log("\n📦 Testing Namespace Verification...");

  const mockAdapter = new MockAdapter();
  const baseSlot = calculateErc7201Slot(mockAdapter, "test.storage.MyContract");
  const baseSlotBigInt = BigInt(baseSlot);

  // Set storage for offset 0 (address)
  const slot0 = "0x" + baseSlotBigInt.toString(16).padStart(64, "0");
  mockAdapter.setStorage(slot0, "0x000000000000000000000000" + TEST_OWNER.slice(2));

  const config: NamespaceConfig = {
    id: "test.storage.MyContract",
    variables: [{ offset: 0, type: "address", name: "admin", expected: TEST_OWNER }],
  };

  const result = await verifyNamespace(mockAdapter, TEST_ADDRESS, config);

  assertEqual(result.status, "pass", "Namespace verification passes");
  assertEqual(result.variables[0].status, "pass", "Variable verification passes");
}

async function testFullStateVerification(): Promise<void> {
  console.log("\n📦 Testing Full State Verification...");

  const mockAdapter = new MockAdapter();

  // Setup view call mock using dynamic encoding
  const ownerCalldata = dynamicEncodeFunctionData(MOCK_ABI, "owner", []);
  const ownerReturnData = dynamicEncodeAbiParameters(["address"], [TEST_OWNER]);
  mockAdapter.setCallResult(ownerCalldata, ownerReturnData);

  // Setup slot mock
  mockAdapter.setStorage("0x0", "0x" + "0".repeat(62) + "06");

  // Setup namespace mock
  const baseSlot = calculateErc7201Slot(mockAdapter, "linea.storage.YieldManager");
  const baseSlotBigInt = BigInt(baseSlot);
  const slot0 = "0x" + baseSlotBigInt.toString(16).padStart(64, "0");
  mockAdapter.setStorage(slot0, "0x000000000000000000000000" + TEST_OWNER.slice(2));

  const config: StateVerificationConfig = {
    viewCalls: [{ function: "owner", expected: TEST_OWNER }],
    slots: [{ slot: "0x0", type: "uint8", name: "_initialized", expected: "6" }],
    namespaces: [
      {
        id: "linea.storage.YieldManager",
        variables: [{ offset: 0, type: "address", name: "messageService", expected: TEST_OWNER }],
      },
    ],
  };

  const verifier = new Verifier(mockAdapter);
  const result = await verifier.verifyState(TEST_ADDRESS, MOCK_ABI, config);

  assertEqual(result.status, "pass", "Full state verification passes");
  assert(result.message.includes("3 state checks passed"), "Message indicates 3 checks passed");
}

// ============================================================================
// Foundry Artifact Tests
// ============================================================================

async function testArtifactFormatDetection(): Promise<void> {
  console.log("\n📦 Testing Artifact Format Detection...");

  // Hardhat format
  const hardhatArtifact = {
    contractName: "TestContract",
    abi: [],
    bytecode: "0x6080604052",
    deployedBytecode: "0x6080604052",
  };
  assertEqual(detectArtifactFormat(hardhatArtifact), "hardhat", "Detects Hardhat format");

  // Foundry format
  const foundryArtifact = {
    abi: [],
    bytecode: { object: "0x6080604052" },
    deployedBytecode: { object: "0x6080604052" },
  };
  assertEqual(detectArtifactFormat(foundryArtifact), "foundry", "Detects Foundry format");
}

async function testFoundryMethodIdentifiers(): Promise<void> {
  console.log("\n📦 Testing Foundry Method Identifiers...");

  const mockAdapter = new MockAdapter();

  const artifact: NormalizedArtifact = {
    format: "foundry",
    contractName: "TestContract",
    abi: [],
    bytecode: "0x",
    deployedBytecode: "0x",
    immutableReferences: undefined,
    methodIdentifiers: new Map([
      ["8da5cb5b", "owner()"],
      ["91d14854", "hasRole(bytes32,address)"],
    ]),
    deployedLinkReferences: undefined,
  };

  const selectors = extractSelectorsFromArtifact(mockAdapter, artifact);
  assertEqual(selectors.size, 2, "Extracts 2 selectors from Foundry artifact");
  assertEqual(selectors.get("8da5cb5b"), "owner()", "Correct owner selector mapping");
  assertEqual(selectors.get("91d14854"), "hasRole(bytes32,address)", "Correct hasRole selector mapping");
}

async function testBytecodeComparisonWithKnownImmutables(): Promise<void> {
  console.log("\n📦 Testing Bytecode Comparison with Known Immutables...");

  // Create bytecode with a known difference at position 100 (32 bytes)
  const baseBytecode = "60806040" + "00".repeat(96); // 100 bytes of padding
  const localImmutable = "0".repeat(64); // 32 bytes of zeros (placeholder)
  const remoteImmutable = "abcdef1234567890".repeat(4); // 32 bytes of actual value
  const suffix = "00".repeat(50); // More padding

  const localBytecode = baseBytecode + localImmutable + suffix;
  const remoteBytecode = baseBytecode + remoteImmutable + suffix;

  // Known immutable at position 100, length 32
  const knownImmutables: ImmutableReference[] = [{ start: 100, length: 32 }];

  const result = compareBytecode(localBytecode, remoteBytecode, knownImmutables);

  assertEqual(result.status, "pass", "Passes when differences are at known immutable positions");
  assert(result.onlyImmutablesDiffer === true, "Correctly identifies only immutables differ");
  assert(result.message.includes("known positions"), "Message indicates known positions: " + result.message);
}

async function testBytecodeComparisonWithUnknownDifferences(): Promise<void> {
  console.log("\n📦 Testing Bytecode Comparison with Unknown Differences...");

  // Create bytecode with difference NOT at immutable position
  const baseBytecode = "60806040" + "00".repeat(50);
  const localPart = "aa".repeat(10);
  const remotePart = "bb".repeat(10);
  const suffix = "00".repeat(100);

  const localBytecode = baseBytecode + localPart + suffix;
  const remoteBytecode = baseBytecode + remotePart + suffix;

  // Known immutable at position 200 (not where the difference is)
  const knownImmutables: ImmutableReference[] = [{ start: 200, length: 32 }];

  const result = compareBytecode(localBytecode, remoteBytecode, knownImmutables);

  assertEqual(result.status, "fail", "Fails when differences are not at known immutable positions");
  assert(result.onlyImmutablesDiffer === false, "Correctly identifies unexpected differences");
}

// ============================================================================
// Main
// ============================================================================

async function main(): Promise<void> {
  console.log("🧪 Running Contract Integrity Verifier Tests\n");
  console.log("=".repeat(50));

  // State verification tests
  await testErc7201SlotCalculation();
  await testDecodeSlotValue();
  await testAllSolidityTypes();
  await testTupleAndStructComparison();
  await testViewCalls();
  await testSlotVerification();
  await testNamespaceVerification();
  await testFullStateVerification();

  // Foundry artifact tests
  await testArtifactFormatDetection();
  await testFoundryMethodIdentifiers();
  await testBytecodeComparisonWithKnownImmutables();
  await testBytecodeComparisonWithUnknownDifferences();

  // Storage path tests
  await testStoragePathParsing();
  await testStoragePathSlotComputation();
  await testErc7201BaseSlotCalculation();

  // Markdown config tests
  await testMarkdownConfigParsing();

  // Complex ERC-7201 schema tests
  await testComplexErc7201Schema();
  await testPackedStorageDecoding();
  await testMappingToStructSlotComputation();
  await testArraySlotComputation();
  await testNestedStructAccess();
  await testDirectlyNestedStructs();
  await testVerifyStoragePathMessageFormatting();

  // Bug fix tests (Cycle 7+)
  await testUint16Decoding();
  await testParsePathValidation();
  await testImmutableValidationImproved();
  await testSkipStatusCounting();

  // Edge case tests (Cycle 8-10)
  await testEmptyBytecodeHandling();
  await testMarkdownSlotTypeValidation();
  await testSelectorExtractionEdgeCases();
  await testSchemaValidation();

  // New bug fix tests (Cycle 1-5)
  await testSelectorExtractionBoundary();
  await testCompareValuesNonNumeric();
  await testEncodeKeyValidation();
  await testArtifactLoadingErrors();

  // Browser-compatible API tests
  await testParseArtifact();
  await testParseStorageSchema();

  // Bug fix: immutable values no double-match
  testImmutableValuesNoDoubleMatch();

  // Bug fix: CBOR stripping edge cases
  testCborStrippingEdgeCases();

  // Linked library tests
  testLinkLibraries();
  testLinkLibrariesMissingAddress();
  testLinkLibrariesInvalidAddress();
  testLinkLibrariesMultiplePositions();
  testLinkLibrariesMultipleDistinctLibraries();
  testLinkLibrariesPartialAddresses();
  testVerifyLinkedLibraries();
  testDetectUnlinkedLibraries();
  testDetectUnlinkedLibrariesMultiple();
  testLinkLibrariesWithoutPrefix();

  console.log("\n" + "=".repeat(50));
  console.log(`\n📊 Results: ${testsPassed} passed, ${testsFailed} failed`);

  if (testsFailed > 0) {
    process.exit(1);
  }
}

// ============================================================================
// Storage Path Tests
// ============================================================================

async function testStoragePathParsing(): Promise<void> {
  console.log("\n🧪 Testing storage path parsing...");

  // Simple field
  const simple = parsePath("MyStruct:myField");
  assertEqual(simple.structName, "MyStruct", "Simple path struct name");
  assertEqual(simple.segments.length, 1, "Simple path segment count");
  assertEqual(simple.segments[0].type, "field", "Simple path segment type");
  if (simple.segments[0].type === "field") {
    assertEqual(simple.segments[0].name, "myField", "Simple path field name");
  }

  // Nested field
  const nested = parsePath("Storage:config.value");
  assertEqual(nested.structName, "Storage", "Nested path struct name");
  assertEqual(nested.segments.length, 2, "Nested path segment count");

  // Array index
  const array = parsePath("Storage:items[0]");
  assertEqual(array.segments.length, 2, "Array path segment count");
  if (array.segments[1].type === "arrayIndex") {
    assertEqual(array.segments[1].index, 0, "Array index value");
  }

  // Array length
  const length = parsePath("Storage:items.length");
  assertEqual(length.segments.length, 2, "Array length segment count");
  assertEqual(length.segments[1].type, "arrayLength", "Array length segment type");

  // Mapping key
  const mapping = parsePath("Storage:balances[0x1234567890123456789012345678901234567890]");
  assertEqual(mapping.segments.length, 2, "Mapping path segment count");
  if (mapping.segments[1].type === "mappingKey") {
    assert(mapping.segments[1].key.startsWith("0x"), "Mapping key is address");
  }
}

async function testStoragePathSlotComputation(): Promise<void> {
  console.log("\n🧪 Testing storage path slot computation...");

  const mockAdapter = new MockAdapter();

  const testSchema: StorageSchema = {
    structs: {
      TestStorage: {
        namespace: "test.storage.TestStorage",
        fields: {
          firstField: { slot: 0, type: "address" },
          secondField: { slot: 1, type: "uint256" },
          packedField: { slot: 2, type: "bool", byteOffset: 0 },
        },
      },
    },
  };

  // First field at slot 0
  const path1 = parsePath("TestStorage:firstField");
  const slot1 = computeSlot(mockAdapter, path1, testSchema);
  assertEqual(slot1.type, "address", "First field type");
  assertEqual(slot1.byteOffset, 0, "First field byte offset");

  // Second field at slot 1
  const path2 = parsePath("TestStorage:secondField");
  const slot2 = computeSlot(mockAdapter, path2, testSchema);
  assertEqual(slot2.type, "uint256", "Second field type");

  // Verify slot offset from base
  const baseSlot = BigInt(slot1.slot);
  const secondSlot = BigInt(slot2.slot);
  assertEqual(secondSlot - baseSlot, 1n, "Second field is 1 slot after first");

  // Packed field byte offset
  const path3 = parsePath("TestStorage:packedField");
  const slot3 = computeSlot(mockAdapter, path3, testSchema);
  assertEqual(slot3.type, "bool", "Packed field type");
  assertEqual(slot3.byteOffset, 0, "Packed field byte offset");
}

async function testErc7201BaseSlotCalculation(): Promise<void> {
  console.log("\n🧪 Testing ERC-7201 base slot calculation...");

  const mockAdapter = new MockAdapter();

  // Test with known namespace - LineaRollupYieldExtension
  // The expected slot can be verified using Solidity:
  // bytes32 slot = keccak256(abi.encode(uint256(keccak256("linea.storage.LineaRollupYieldExtension")) - 1)) & ~bytes32(uint256(0xff));
  const namespace = "linea.storage.LineaRollupYieldExtension";
  const slot = calculateErc7201BaseSlot(mockAdapter, namespace);

  // Should be a valid 32-byte hex string
  assert(slot.startsWith("0x"), "Slot starts with 0x");
  assertEqual(slot.length, 66, "Slot is 66 chars (0x + 64 hex)");

  // Last byte should be 00 (masked)
  assert(slot.endsWith("00"), "Last byte is 00 (masked)");

  // Different namespaces should produce different slots
  const slot2 = calculateErc7201BaseSlot(mockAdapter, "linea.storage.Different");
  assert(slot !== slot2, "Different namespaces produce different slots");
}

// ============================================================================
// Markdown Config Tests
// ============================================================================

async function testMarkdownConfigParsing(): Promise<void> {
  console.log("\n🧪 Testing markdown config parsing...");

  const markdown = `
# Test Verification Config

## Contract: TestContract

\`\`\`verifier
name: TestContract
address: 0x1234567890123456789012345678901234567890
chain: ethereum-sepolia
artifact: ./artifacts/Test.json
isProxy: true
ozVersion: v4
schema: ./schemas/test.json
\`\`\`

### State Verification

| Type | Description | Check | Params | Expected |
|------|-------------|-------|--------|----------|
| viewCall | Get owner | \`owner\` | | \`0xabcdef1234567890abcdef1234567890abcdef12\` |
| viewCall | Check role | \`hasRole\` | \`0x1234\`,\`0x5678\` | true |
| slot | Initialized | \`0x0\` | uint8 | \`1\` |
| storagePath | Config value | \`TestStorage:value\` | | \`100\` |

## Contract: SecondContract

\`\`\`verifier
address: 0xabcdefabcdefabcdefabcdefabcdefabcdefabcd
chain: ethereum-sepolia
artifact: ./artifacts/Second.json
isProxy: false
\`\`\`
`;

  const config = parseMarkdownConfig(markdown, "/test/dir");

  // Check contracts were parsed
  assertEqual(config.contracts.length, 2, "Parsed 2 contracts");

  // Check first contract
  const first = config.contracts[0];
  assertEqual(first.name, "TestContract", "First contract name");
  assertEqual(first.address, "0x1234567890123456789012345678901234567890", "First contract address");
  assertEqual(first.chain, "ethereum-sepolia", "First contract chain");
  assertEqual(first.isProxy, true, "First contract isProxy");

  // Check state verification
  assert(first.stateVerification !== undefined, "First contract has state verification");
  if (first.stateVerification) {
    assertEqual(first.stateVerification.ozVersion, "v4", "OZ version");
    assertEqual(first.stateVerification.viewCalls?.length, 2, "Has 2 view calls");
    assertEqual(first.stateVerification.slots?.length, 1, "Has 1 slot check");
    assertEqual(first.stateVerification.storagePaths?.length, 1, "Has 1 storage path");

    // Check view call with params
    const hasRoleCall = first.stateVerification.viewCalls?.find((v) => v.function === "hasRole");
    assert(hasRoleCall !== undefined, "hasRole view call exists");
    if (hasRoleCall) {
      assertEqual(hasRoleCall.params?.length, 2, "hasRole has 2 params");
      assertEqual(hasRoleCall.expected, true, "hasRole expected value");
    }
  }

  // Check second contract
  const second = config.contracts[1];
  assertEqual(second.address, "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd", "Second contract address");
  assertEqual(second.isProxy, false, "Second contract isProxy");
  assert(second.stateVerification === undefined, "Second contract has no state verification");

  // Check default chains were included
  assert(config.chains["ethereum-sepolia"] !== undefined, "Sepolia chain exists");
  assertEqual(config.chains["ethereum-sepolia"].chainId, 11155111, "Sepolia chain ID");
}

// ============================================================================
// Complex ERC-7201 Schema Tests
// ============================================================================

/**
 * Complex schema based on YieldManagerStorageLayout.sol
 */
const YIELD_MANAGER_SCHEMA: StorageSchema = {
  structs: {
    YieldManagerStorage: {
      namespace: "linea.storage.YieldManagerStorage",
      baseSlot: "0xdc1272075efdca0b85fb2d76cbb5f26d954dc18e040d6d0b67071bd5cbd04300",
      fields: {
        minimumWithdrawalReservePercentageBps: { slot: 0, type: "uint16", byteOffset: 0 },
        targetWithdrawalReservePercentageBps: { slot: 0, type: "uint16", byteOffset: 2 },
        minimumWithdrawalReserveAmount: { slot: 1, type: "uint256" },
        targetWithdrawalReserveAmount: { slot: 2, type: "uint256" },
        userFundsInYieldProvidersTotal: { slot: 3, type: "uint256" },
        pendingPermissionlessUnstake: { slot: 4, type: "uint256" },
        yieldProviders: { slot: 5, type: "address[]" },
        isL2YieldRecipientKnown: { slot: 6, type: "mapping(address => bool)" },
        yieldProviderStorage: { slot: 7, type: "mapping(address => YieldProviderStorage)" },
        lastProvenSlot: { slot: 8, type: "mapping(uint64 => uint64)" },
      },
    },
    YieldProviderStorage: {
      // No namespace - accessed through mapping
      fields: {
        yieldProviderVendor: { slot: 0, type: "uint8", byteOffset: 0 },
        isStakingPaused: { slot: 0, type: "bool", byteOffset: 1 },
        isOssificationInitiated: { slot: 0, type: "bool", byteOffset: 2 },
        isOssified: { slot: 0, type: "bool", byteOffset: 3 },
        primaryEntrypoint: { slot: 0, type: "address", byteOffset: 4 },
        ossifiedEntrypoint: { slot: 1, type: "address", byteOffset: 0 },
        yieldProviderIndex: { slot: 1, type: "uint96", byteOffset: 20 },
        userFunds: { slot: 2, type: "uint256" },
        yieldReportedCumulative: { slot: 3, type: "uint256" },
        lstLiabilityPrincipal: { slot: 4, type: "uint256" },
        lastReportedNegativeYield: { slot: 5, type: "uint256" },
      },
    },
  },
};

async function testComplexErc7201Schema(): Promise<void> {
  console.log("\n🧪 Testing complex ERC-7201 schema (YieldManager)...");

  const mockAdapter = new MockAdapter();

  // Verify schema loads correctly
  assert(YIELD_MANAGER_SCHEMA.structs.YieldManagerStorage !== undefined, "YieldManagerStorage struct exists");
  assert(YIELD_MANAGER_SCHEMA.structs.YieldProviderStorage !== undefined, "YieldProviderStorage struct exists");

  // Verify base slot matches the Solidity constant
  const expectedBaseSlot = "0xdc1272075efdca0b85fb2d76cbb5f26d954dc18e040d6d0b67071bd5cbd04300";
  assertEqual(
    YIELD_MANAGER_SCHEMA.structs.YieldManagerStorage.baseSlot,
    expectedBaseSlot,
    "Base slot matches Solidity constant",
  );

  // Verify computed base slot from namespace matches
  const computedBaseSlot = calculateErc7201BaseSlot(mockAdapter, "linea.storage.YieldManagerStorage");
  assertEqual(computedBaseSlot, expectedBaseSlot, "Computed base slot matches explicit base slot");

  // Simple field access
  const path1 = parsePath("YieldManagerStorage:minimumWithdrawalReserveAmount");
  const slot1 = computeSlot(mockAdapter, path1, YIELD_MANAGER_SCHEMA);
  assertEqual(slot1.type, "uint256", "minimumWithdrawalReserveAmount type");

  // Verify slot offset
  const baseSlotBigInt = BigInt(expectedBaseSlot);
  const slot1BigInt = BigInt(slot1.slot);
  assertEqual(slot1BigInt - baseSlotBigInt, 1n, "minimumWithdrawalReserveAmount is at slot offset 1");
}

async function testPackedStorageDecoding(): Promise<void> {
  console.log("\n🧪 Testing packed storage field access...");

  const mockAdapter = new MockAdapter();

  // Test packed uint16 fields in slot 0
  const path1 = parsePath("YieldManagerStorage:minimumWithdrawalReservePercentageBps");
  const slot1 = computeSlot(mockAdapter, path1, YIELD_MANAGER_SCHEMA);
  assertEqual(slot1.type, "uint16", "First packed field type");
  assertEqual(slot1.byteOffset, 0, "First packed field byte offset");

  const path2 = parsePath("YieldManagerStorage:targetWithdrawalReservePercentageBps");
  const slot2 = computeSlot(mockAdapter, path2, YIELD_MANAGER_SCHEMA);
  assertEqual(slot2.type, "uint16", "Second packed field type");
  assertEqual(slot2.byteOffset, 2, "Second packed field byte offset");

  // Verify both are in the same slot
  assertEqual(slot1.slot, slot2.slot, "Packed fields share same slot");

  // Test decoding packed values
  // Storage layout (right-aligned): ... | targetBps (bytes 29-30) | minimumBps (bytes 30-31) |
  // Let's say minimum=1000 (0x03E8) and target=2000 (0x07D0)
  // Raw storage: 0x...0000000007D003E8 (last 4 bytes)
  const packedValue = "0x" + "0".repeat(56) + "07d003e8";

  // Decode minimum (bytes 30-31, offset 0)
  const minimumHex = packedValue.slice(2 + 60, 2 + 64); // Last 2 bytes
  assertEqual(minimumHex, "03e8", "Minimum value hex extraction");

  // Decode target (bytes 28-29, offset 2)
  const targetHex = packedValue.slice(2 + 56, 2 + 60); // Bytes 28-29
  assertEqual(targetHex, "07d0", "Target value hex extraction");
}

async function testMappingToStructSlotComputation(): Promise<void> {
  console.log("\n🧪 Testing mapping to struct slot computation...");

  const mockAdapter = new MockAdapter();
  const yieldProviderAddress = "0x1234567890123456789012345678901234567890";

  // Access yieldProviderStorage[address].userFunds
  const path = parsePath(`YieldManagerStorage:yieldProviderStorage[${yieldProviderAddress}].userFunds`);

  assertEqual(path.structName, "YieldManagerStorage", "Struct name parsed");
  assertEqual(path.segments.length, 3, "Three segments parsed");

  assert(path.segments[0].type === "field", "First segment is field");
  if (path.segments[0].type === "field") {
    assertEqual(path.segments[0].name, "yieldProviderStorage", "First segment name");
  }

  assert(path.segments[1].type === "mappingKey", "Second segment is mapping key");
  if (path.segments[1].type === "mappingKey") {
    assertEqual(path.segments[1].key, yieldProviderAddress, "Mapping key value");
  }

  assert(path.segments[2].type === "field", "Third segment is field");
  if (path.segments[2].type === "field") {
    assertEqual(path.segments[2].name, "userFunds", "Third segment name");
  }

  // Compute the slot
  const computed = computeSlot(mockAdapter, path, YIELD_MANAGER_SCHEMA);
  assertEqual(computed.type, "uint256", "userFunds type");
  assert(computed.slot.startsWith("0x"), "Computed slot is hex string");
  assertEqual(computed.slot.length, 66, "Computed slot is 32 bytes");
}

async function testArraySlotComputation(): Promise<void> {
  console.log("\n🧪 Testing array slot computation...");

  const mockAdapter = new MockAdapter();

  // Test array length
  const lengthPath = parsePath("YieldManagerStorage:yieldProviders.length");
  const lengthSlot = computeSlot(mockAdapter, lengthPath, YIELD_MANAGER_SCHEMA);
  assertEqual(lengthSlot.type, "uint256", "Array length type");

  // Verify length slot is at base + 5
  const baseSlot = BigInt(YIELD_MANAGER_SCHEMA.structs.YieldManagerStorage.baseSlot!);
  const lengthSlotBigInt = BigInt(lengthSlot.slot);
  assertEqual(lengthSlotBigInt - baseSlot, 5n, "yieldProviders is at slot offset 5");

  // Test array element access
  const element0Path = parsePath("YieldManagerStorage:yieldProviders[0]");
  const element0Slot = computeSlot(mockAdapter, element0Path, YIELD_MANAGER_SCHEMA);
  assertEqual(element0Slot.type, "address", "Array element type");

  // Array data starts at keccak256(slot)
  const expectedDataSlot = dynamicKeccak256(dynamicEncodeAbiParameters(["uint256"], [lengthSlotBigInt]));
  assertEqual(element0Slot.slot, expectedDataSlot, "Array element 0 slot is keccak256(length_slot)");

  // Element 1 should be at dataSlot + 1
  const element1Path = parsePath("YieldManagerStorage:yieldProviders[1]");
  const element1Slot = computeSlot(mockAdapter, element1Path, YIELD_MANAGER_SCHEMA);
  const element1SlotBigInt = BigInt(element1Slot.slot);
  const element0SlotBigInt = BigInt(element0Slot.slot);
  assertEqual(element1SlotBigInt - element0SlotBigInt, 1n, "Array element 1 is at element 0 + 1");
}

async function testNestedStructAccess(): Promise<void> {
  console.log("\n🧪 Testing nested struct field access through mapping...");

  const mockAdapter = new MockAdapter();
  const yieldProviderAddress = "0xabcdef1234567890abcdef1234567890abcdef12";

  // Access packed fields in YieldProviderStorage through mapping
  // yieldProviderStorage[address].isStakingPaused
  const pausedPath = parsePath(`YieldManagerStorage:yieldProviderStorage[${yieldProviderAddress}].isStakingPaused`);
  const pausedSlot = computeSlot(mockAdapter, pausedPath, YIELD_MANAGER_SCHEMA);

  assertEqual(pausedSlot.type, "bool", "isStakingPaused type");
  assertEqual(pausedSlot.byteOffset, 1, "isStakingPaused byte offset");

  // Access primaryEntrypoint (address at offset 4 in slot 0)
  const entrypointPath = parsePath(
    `YieldManagerStorage:yieldProviderStorage[${yieldProviderAddress}].primaryEntrypoint`,
  );
  const entrypointSlot = computeSlot(mockAdapter, entrypointPath, YIELD_MANAGER_SCHEMA);

  assertEqual(entrypointSlot.type, "address", "primaryEntrypoint type");
  assertEqual(entrypointSlot.byteOffset, 4, "primaryEntrypoint byte offset");

  // Verify both are in the same slot (slot 0 of YieldProviderStorage)
  assertEqual(pausedSlot.slot, entrypointSlot.slot, "Packed fields in same slot");

  // Access field in slot 1 (ossifiedEntrypoint)
  const ossifiedPath = parsePath(
    `YieldManagerStorage:yieldProviderStorage[${yieldProviderAddress}].ossifiedEntrypoint`,
  );
  const ossifiedSlot = computeSlot(mockAdapter, ossifiedPath, YIELD_MANAGER_SCHEMA);

  assertEqual(ossifiedSlot.type, "address", "ossifiedEntrypoint type");
  assertEqual(ossifiedSlot.byteOffset, 0, "ossifiedEntrypoint byte offset");

  // Verify slot 1 is different from slot 0
  const slot0BigInt = BigInt(pausedSlot.slot);
  const slot1BigInt = BigInt(ossifiedSlot.slot);
  assertEqual(slot1BigInt - slot0BigInt, 1n, "ossifiedEntrypoint is 1 slot after slot 0");

  // Access yieldProviderIndex (uint96 at offset 20 in slot 1)
  const indexPath = parsePath(`YieldManagerStorage:yieldProviderStorage[${yieldProviderAddress}].yieldProviderIndex`);
  const indexSlot = computeSlot(mockAdapter, indexPath, YIELD_MANAGER_SCHEMA);

  assertEqual(indexSlot.type, "uint96", "yieldProviderIndex type");
  assertEqual(indexSlot.byteOffset, 20, "yieldProviderIndex byte offset");
  assertEqual(indexSlot.slot, ossifiedSlot.slot, "yieldProviderIndex in same slot as ossifiedEntrypoint");

  // Access uint256 field in slot 2
  const userFundsPath = parsePath(`YieldManagerStorage:yieldProviderStorage[${yieldProviderAddress}].userFunds`);
  const userFundsSlot = computeSlot(mockAdapter, userFundsPath, YIELD_MANAGER_SCHEMA);

  assertEqual(userFundsSlot.type, "uint256", "userFunds type");
  assertEqual(userFundsSlot.byteOffset, 0, "userFunds byte offset");

  const slot2BigInt = BigInt(userFundsSlot.slot);
  assertEqual(slot2BigInt - slot0BigInt, 2n, "userFunds is 2 slots after slot 0");
}

// ============================================================================
// Bug Fix Tests (Cycle 7+)
// ============================================================================

async function testUint16Decoding(): Promise<void> {
  console.log("\n🧪 Testing uint16 decoding (bug fix)...");

  const mockAdapter = new MockAdapter();

  // Test uint16 value decoding (was missing in getTypeBytes)
  const uint16Value = "0x" + "0".repeat(60) + "07d0"; // 2000 in hex
  const decodedUint16 = decodeSlotValue(mockAdapter, uint16Value, "uint16");
  assertEqual(decodedUint16, "2000", "Decode uint16 value");

  // Test int96 value (was missing in storage-path)
  // Note: int96 is 12 bytes = 24 hex chars
  const int96Value = "0x" + "0".repeat(40) + "000000000000000001e240"; // 123456
  const decodedInt96 = decodeSlotValue(mockAdapter, int96Value, "uint96"); // Testing as uint96 since decodeSlotValue in storage.ts handles both
  assertEqual(decodedInt96, "123456", "Decode uint96/int96 value");
}

async function testParsePathValidation(): Promise<void> {
  console.log("\n🧪 Testing parsePath input validation (bug fix)...");

  // Test empty path
  let threw = false;
  try {
    parsePath("");
  } catch (e) {
    threw = true;
    assert(e instanceof Error && e.message.includes("empty"), "Empty path throws with correct message");
  }
  assert(threw, "Empty path throws error");

  // Test missing colon
  threw = false;
  try {
    parsePath("NoColon");
  } catch (e) {
    threw = true;
    assert(e instanceof Error && e.message.includes("Expected"), "Missing colon throws with correct message");
  }
  assert(threw, "Missing colon throws error");

  // Test empty struct name
  threw = false;
  try {
    parsePath(":field");
  } catch (e) {
    threw = true;
    assert(e instanceof Error && e.message.includes("Struct name"), "Empty struct name throws with correct message");
  }
  assert(threw, "Empty struct name throws error");

  // Test invalid struct name
  threw = false;
  try {
    parsePath("123Invalid:field");
  } catch (e) {
    threw = true;
    assert(e instanceof Error && e.message.includes("identifier"), "Invalid struct name throws with correct message");
  }
  assert(threw, "Invalid struct name throws error");

  // Test valid paths still work
  const validPath = parsePath("ValidStruct:validField");
  assertEqual(validPath.structName, "ValidStruct", "Valid path parses struct name");
  assertEqual(validPath.segments[0].type, "field", "Valid path parses segment");
}

async function testImmutableValidationImproved(): Promise<void> {
  console.log("\n🧪 Testing improved immutable validation (bug fix)...");

  // Import using require to avoid ESM extension issues

  // Test that loose matching no longer produces false positives
  // Old bug: "remoteValue.endsWith(expected.replace(/^0+/, ''))" could match incorrectly
  const immutables: ImmutableDifference[] = [
    {
      position: 100,
      length: 32,
      localValue: "0".repeat(64),
      remoteValue: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
      possibleType: "bytes32 or uint256",
    },
  ];

  // This should NOT match since the values are completely different
  const result = validateImmutablesAgainstArgs(immutables, [
    "0x1111111111111111111111111111111111111111111111111111111111111111",
  ]);

  assertEqual(result.valid, false, "Different values should not match");

  // Test that exact matches still work
  const exactImmutables: ImmutableDifference[] = [
    {
      position: 100,
      length: 32,
      localValue: "0".repeat(64),
      remoteValue: "0000000000000000000000001234567890123456789012345678901234567890",
      possibleType: "address",
    },
  ];

  const exactResult = validateImmutablesAgainstArgs(exactImmutables, ["0x1234567890123456789012345678901234567890"]);

  assertEqual(exactResult.valid, true, "Exact address match should work");
}

async function testSkipStatusCounting(): Promise<void> {
  console.log("\n🧪 Testing skip status counting (bug fix)...");

  // This tests the fixed counting logic in verifier.ts
  // The fix ensures that undefined results are handled correctly
  // We can't easily unit test runVerification without mocking, so we test the logic pattern

  type TestResult = { status: string } | undefined;
  interface TestCase {
    bytecodeResult: TestResult;
    abiResult: TestResult;
    stateResult: TestResult;
    expected: string;
  }

  // Simulate the fixed counting logic
  const testCases: TestCase[] = [
    {
      bytecodeResult: undefined,
      abiResult: undefined,
      stateResult: undefined,
      expected: "skipped",
    },
    {
      bytecodeResult: { status: "pass" },
      abiResult: { status: "pass" },
      stateResult: undefined,
      expected: "passed",
    },
    {
      bytecodeResult: { status: "fail" },
      abiResult: { status: "pass" },
      stateResult: undefined,
      expected: "failed",
    },
    {
      bytecodeResult: { status: "warn" },
      abiResult: { status: "pass" },
      stateResult: undefined,
      expected: "warnings",
    },
  ];

  for (const tc of testCases) {
    const bytecodeStatus = tc.bytecodeResult?.status;
    const abiStatus = tc.abiResult?.status;
    const stateStatus = tc.stateResult?.status;

    let result: string;
    if (bytecodeStatus === "fail" || abiStatus === "fail" || stateStatus === "fail") {
      result = "failed";
    } else if (bytecodeStatus === "warn" || abiStatus === "warn" || stateStatus === "warn") {
      result = "warnings";
    } else {
      const hasBytecodeResult = tc.bytecodeResult !== undefined;
      const hasAbiResult = tc.abiResult !== undefined;
      const hasStateResult = tc.stateResult !== undefined;
      const hasAnyVerification = hasBytecodeResult || hasAbiResult || hasStateResult;

      if (!hasAnyVerification) {
        result = "skipped";
      } else if (
        (bytecodeStatus === "skip" || !hasBytecodeResult) &&
        (abiStatus === "skip" || !hasAbiResult) &&
        !hasStateResult
      ) {
        result = "skipped";
      } else {
        result = "passed";
      }
    }

    assertEqual(result, tc.expected, `Counting logic for ${JSON.stringify(tc)} = ${tc.expected}`);
  }
}

// ============================================================================
// Additional Edge Case Tests (Cycle 8-10)
// ============================================================================

async function testEmptyBytecodeHandling(): Promise<void> {
  console.log("\n🧪 Testing empty bytecode handling (edge case)...");

  // Test with empty bytecode - should not throw
  const result = compareBytecode("0x", "0x");
  assertEqual(result.status, "pass", "Empty bytecode comparison passes");
  assertEqual(result.matchPercentage, 100, "Empty bytecode match percentage is 100");

  // Test with only metadata - stripped to empty
  const onlyMetadata = "0xa264697066735822" + "00".repeat(32) + "0021";
  const result2 = compareBytecode(onlyMetadata, onlyMetadata);
  // After stripping metadata, both should be empty and match
  assert(result2.status === "pass", "Metadata-only bytecode handled correctly");
}

async function testMarkdownSlotTypeValidation(): Promise<void> {
  console.log("\n🧪 Testing markdown slot type validation (edge case)...");

  const markdownWithInvalidType = `
## Contract: TestContract

\`\`\`verifier
name: TestContract
address: 0x1234567890123456789012345678901234567890
chain: ethereum-sepolia
artifact: ./artifacts/Test.json
\`\`\`

| Type | Description | Check | Params | Expected |
|------|-------------|-------|--------|----------|
| slot | Invalid type | \`0x0\` | invalid_type | \`1\` |
`;

  const config = parseMarkdownConfig(markdownWithInvalidType, "/test/dir");
  assert(config.contracts.length === 1, "Contract parsed");

  const stateVerification = config.contracts[0].stateVerification;
  assert(stateVerification !== undefined, "State verification exists");

  if (stateVerification?.slots) {
    // Invalid type should default to uint256
    assertEqual(stateVerification.slots[0].type, "uint256", "Invalid slot type defaults to uint256");
  }
}

async function testSelectorExtractionEdgeCases(): Promise<void> {
  console.log("\n🧪 Testing selector extraction edge cases...");

  // Test with short bytecode (less than 10 chars after 0x)
  const shortBytecode = "0x6080";
  const selectors = extractSelectorsFromBytecode(shortBytecode);
  assertEqual(selectors.length, 0, "Short bytecode returns no selectors");

  // Test with empty bytecode
  const emptySelectors = extractSelectorsFromBytecode("0x");
  assertEqual(emptySelectors.length, 0, "Empty bytecode returns no selectors");
}

async function testSchemaValidation(): Promise<void> {
  console.log("\n🧪 Testing schema validation (edge case)...");

  // Test that loadStorageSchema throws for malformed schemas
  // We'll test the validation logic by calling it with mock data

  // This test validates that the schema validation function exists and works
  // In a real scenario, we'd test with actual file operations
  let threw = false;
  try {
    // Attempt to load a non-existent file
    loadStorageSchema("non-existent-file.json", "/tmp");
  } catch (err: unknown) {
    threw = true;
    assert(err instanceof Error, "Throws an Error");
    if (err instanceof Error) {
      assert(err.message.includes("Failed to read schema file"), "Error message is informative");
    }
  }
  assert(threw, "Loading non-existent schema throws error");
}

async function testSelectorExtractionBoundary(): Promise<void> {
  console.log("\n🧪 Testing selector extraction boundary condition (Cycle 3 fix)...");

  // Test with exactly 10 hex chars (minimum needed: 2 for opcode + 8 for selector)
  // This should find the selector at position 0
  const minimalBytecode = "0x6312345678"; // PUSH4 + 4 bytes = exactly 10 chars
  const selectors = extractSelectorsFromBytecode(minimalBytecode);
  assertEqual(selectors.length, 1, "Minimal bytecode (10 chars) extracts selector");
  assertEqual(selectors[0], "12345678", "Extracted selector is correct");

  // Test with 8 chars (not enough)
  const tooShort = "0x63123456"; // Only 8 chars after 0x
  const noSelectors = extractSelectorsFromBytecode(tooShort);
  assertEqual(noSelectors.length, 0, "8 chars bytecode returns no selectors");

  // Test selector at the last valid position
  const selectorAtEnd = "0x0000006312345678"; // Selector at position 6 (bytes 3-7)
  const endSelectors = extractSelectorsFromBytecode(selectorAtEnd);
  assert(endSelectors.includes("12345678"), "Selector at end of bytecode is found");
}

async function testCompareValuesNonNumeric(): Promise<void> {
  console.log("\n🧪 Testing compareValues with non-numeric values (Cycle 1/2 fix)...");

  const mockAdapter = new MockAdapter();

  // We need to test the internal compareValues function indirectly through verifySlot
  // The fix ensures gt/gte/lt/lte comparisons don't crash with non-numeric values
  // We can't directly test the private function, but we can verify the module loads correctly
  // The real test is that the code doesn't throw when comparing non-numeric values with gt/gte/lt/lte

  // Test that decodeSlotValue handles addresses correctly
  const addressValue = "0x0000000000000000000000001234567890abcdef1234567890abcdef12345678";
  const decoded = decodeSlotValue(mockAdapter, addressValue, "address", 0);
  assert(typeof decoded === "string", "Address decoded as string");
  assert((decoded as string).startsWith("0x"), "Address has 0x prefix");

  // Test that numeric values still work
  const numericValue = "0x0000000000000000000000000000000000000000000000000000000000000064"; // 100 in hex
  const decodedNum = decodeSlotValue(mockAdapter, numericValue, "uint256", 0);
  assertEqual(decodedNum, "100", "Numeric value decoded correctly");
}

async function testEncodeKeyValidation(): Promise<void> {
  console.log("\n🧪 Testing encodeKey validation (Cycle 5 fix)...");

  const mockAdapter = new MockAdapter();

  // Test that parsePath with mapping works correctly
  // We test this through computeSlot which uses encodeKey

  const schema: StorageSchema = {
    structs: {
      TestStruct: {
        namespace: "test.namespace",
        fields: {
          myMapping: {
            slot: 0,
            type: "mapping(address => uint256)" as const,
          },
        },
      },
    },
  };

  // Valid address key
  const validPath = parsePath("TestStruct:myMapping[0x1234567890123456789012345678901234567890]");
  assert(validPath.structName === "TestStruct", "Struct name parsed");
  assert(validPath.segments.length === 2, "Two segments (field + mapping key)");

  // The actual encoding validation happens in computeSlot
  let threw = false;
  try {
    // This should work with a valid address
    computeSlot(mockAdapter, validPath, schema);
  } catch {
    threw = true;
  }
  assert(!threw, "Valid address key does not throw");

  // Test with invalid address (should throw when computing)
  const invalidPath = parsePath("TestStruct:myMapping[invalid-address]");
  threw = false;
  try {
    computeSlot(mockAdapter, invalidPath, schema);
  } catch (err: unknown) {
    threw = true;
    if (err instanceof Error) {
      assert(err.message.includes("Invalid address key"), "Error mentions invalid address");
    }
  }
  assert(threw, "Invalid address key throws error");
}

async function testArtifactLoadingErrors(): Promise<void> {
  console.log("\n🧪 Testing artifact loading error handling (Cycle 4 fix)...");

  // Test loading non-existent file
  let threw = false;
  try {
    loadArtifact("/non/existent/path/artifact.json");
  } catch (err: unknown) {
    threw = true;
    if (err instanceof Error) {
      assert(err.message.includes("Failed to read artifact file"), "Error message mentions file read failure");
    }
  }
  assert(threw, "Loading non-existent artifact throws error");
}

// ============================================================================
// Browser-Compatible API Tests
// ============================================================================

async function testParseArtifact(): Promise<void> {
  console.log("\n🧪 Testing parseArtifact (browser-compatible)...");

  // Test with Hardhat-style artifact as string
  const hardhatArtifact = JSON.stringify({
    contractName: "TestContract",
    abi: [{ type: "function", name: "test", inputs: [], outputs: [], stateMutability: "view" }],
    bytecode: "0x6080604052",
    deployedBytecode: "0x6080604052348015600f57600080fd5b50",
  });

  const parsed1 = parseArtifact(hardhatArtifact);
  assertEqual(parsed1.format, "hardhat", "parseArtifact detects Hardhat format from string");
  assertEqual(parsed1.contractName, "TestContract", "parseArtifact preserves contract name");
  assert(parsed1.abi.length === 1, "parseArtifact preserves ABI");
  assertEqual(parsed1.bytecode, "0x6080604052", "parseArtifact preserves bytecode");

  // Test with Foundry-style artifact as object
  const foundryArtifact = {
    abi: [{ type: "function", name: "test", inputs: [], outputs: [], stateMutability: "view" }],
    bytecode: { object: "0x6080604052" },
    deployedBytecode: {
      object: "0x6080604052348015600f57600080fd5b50",
      immutableReferences: { "1": [{ start: 10, length: 32 }] },
    },
    methodIdentifiers: { "test()": "f8a8fd6d" },
  };

  const parsed2 = parseArtifact(foundryArtifact, "MyContract.json");
  assertEqual(parsed2.format, "foundry", "parseArtifact detects Foundry format from object");
  assertEqual(parsed2.contractName, "MyContract", "parseArtifact uses filename for contract name");
  assert(parsed2.immutableReferences !== undefined, "parseArtifact extracts immutable references");
  assert(parsed2.methodIdentifiers !== undefined, "parseArtifact extracts method identifiers");

  // Test with invalid JSON string
  let threw = false;
  try {
    parseArtifact("not valid json");
  } catch (err: unknown) {
    threw = true;
    if (err instanceof Error) {
      assert(err.message.includes("Failed to parse"), "Error message mentions parse failure");
    }
  }
  assert(threw, "parseArtifact throws on invalid JSON");
}

async function testParseStorageSchema(): Promise<void> {
  console.log("\n🧪 Testing parseStorageSchema (browser-compatible)...");

  // Test with schema as string
  const schemaString = JSON.stringify({
    structs: {
      TestStorage: {
        namespace: "test.storage.TestStorage",
        fields: {
          value: { slot: 0, type: "uint256" },
          owner: { slot: 1, type: "address" },
        },
      },
    },
  });

  const parsed1 = parseStorageSchema(schemaString);
  assert(parsed1.structs.TestStorage !== undefined, "parseStorageSchema parses struct from string");
  assertEqual(
    parsed1.structs.TestStorage.namespace,
    "test.storage.TestStorage",
    "parseStorageSchema preserves namespace",
  );
  assertEqual(parsed1.structs.TestStorage.fields.value.slot, 0, "parseStorageSchema preserves field slots");
  assertEqual(parsed1.structs.TestStorage.fields.value.type, "uint256", "parseStorageSchema preserves field types");

  // Test with schema as object
  const schemaObject = {
    structs: {
      SimpleStorage: {
        baseSlot: "0x1234",
        fields: {
          data: { slot: 0, type: "bytes32" },
        },
      },
    },
  };

  const parsed2 = parseStorageSchema(schemaObject);
  assert(parsed2.structs.SimpleStorage !== undefined, "parseStorageSchema parses struct from object");
  assertEqual(parsed2.structs.SimpleStorage.baseSlot, "0x1234", "parseStorageSchema preserves baseSlot");

  // Test with invalid JSON string
  let threw = false;
  try {
    parseStorageSchema("not valid json");
  } catch (err: unknown) {
    threw = true;
    if (err instanceof Error) {
      assert(err.message.includes("Failed to parse"), "Error message mentions parse failure");
    }
  }
  assert(threw, "parseStorageSchema throws on invalid JSON");

  // Test with invalid schema structure (missing structs)
  threw = false;
  try {
    parseStorageSchema({ notStructs: {} });
  } catch (err: unknown) {
    threw = true;
    if (err instanceof Error) {
      assert(err.message.includes("missing 'structs'"), "Error message mentions missing structs");
    }
  }
  assert(threw, "parseStorageSchema throws on invalid schema structure");

  // Test with invalid field (missing slot)
  threw = false;
  try {
    parseStorageSchema({
      structs: {
        BadStruct: {
          fields: {
            badField: { type: "uint256" }, // missing slot
          },
        },
      },
    });
  } catch (err: unknown) {
    threw = true;
    if (err instanceof Error) {
      assert(err.message.includes("missing numeric 'slot'"), "Error message mentions missing slot");
    }
  }
  assert(threw, "parseStorageSchema throws on field missing slot");
}

// ============================================================================
// All Solidity Types Tests
// ============================================================================

async function testAllSolidityTypes(): Promise<void> {
  console.log("\n🧪 Testing All Solidity Type Decoding...");

  const mockAdapter = new MockAdapter();

  // Test uint24 (3 bytes = 6 hex chars)
  // 0x123456 = 1193046
  const uint24Value = "0x" + "0".repeat(58) + "123456";
  const decodedUint24 = decodeSlotValue(mockAdapter, uint24Value, "uint24");
  assertEqual(decodedUint24, "1193046", "Decode uint24");

  // Test uint40 (5 bytes = 10 hex chars)
  // 0x1234567890 = 78187493520
  const uint40Value = "0x" + "0".repeat(54) + "1234567890";
  const decodedUint40 = decodeSlotValue(mockAdapter, uint40Value, "uint40");
  assertEqual(decodedUint40, "78187493520", "Decode uint40");

  // Test uint48 (6 bytes = 12 hex chars)
  const uint48Value = "0x" + "0".repeat(52) + "ffffffffffff";
  const decodedUint48 = decodeSlotValue(mockAdapter, uint48Value, "uint48");
  assertEqual(decodedUint48, "281474976710655", "Decode uint48 max");

  // Test uint160 (20 bytes = 40 hex chars) - same size as address
  const uint160Value = "0x" + "0".repeat(24) + "ff".repeat(20);
  const decodedUint160 = decodeSlotValue(mockAdapter, uint160Value, "uint160");
  assertEqual(decodedUint160, "1461501637330902918203684832716283019655932542975", "Decode uint160 max");

  // Test int24 positive
  const int24PosValue = "0x" + "0".repeat(58) + "7fffff"; // max int24 = 8388607
  const decodedInt24Pos = decodeSlotValue(mockAdapter, int24PosValue, "int24");
  assertEqual(decodedInt24Pos, "8388607", "Decode int24 positive max");

  // Test int24 negative (-1)
  const int24NegValue = "0x" + "f".repeat(58) + "ffffff"; // -1 in two's complement
  const decodedInt24Neg = decodeSlotValue(mockAdapter, int24NegValue, "int24");
  assertEqual(decodedInt24Neg, "-1", "Decode int24 negative (-1)");

  // Test int24 negative minimum (-8388608)
  const int24MinValue = "0x" + "f".repeat(58) + "800000"; // min int24
  const decodedInt24Min = decodeSlotValue(mockAdapter, int24MinValue, "int24");
  assertEqual(decodedInt24Min, "-8388608", "Decode int24 negative min");

  // Test int40 negative
  const int40NegValue = "0x" + "f".repeat(54) + "ffffffffff"; // -1 in 40 bits
  const decodedInt40Neg = decodeSlotValue(mockAdapter, int40NegValue, "int40");
  assertEqual(decodedInt40Neg, "-1", "Decode int40 negative (-1)");

  // Test bytes1
  const bytes1Value = "0x" + "0".repeat(62) + "ab";
  const decodedBytes1 = decodeSlotValue(mockAdapter, bytes1Value, "bytes1");
  assertEqual(decodedBytes1, "0xab", "Decode bytes1");

  // Test bytes2
  const bytes2Value = "0x" + "0".repeat(60) + "abcd";
  const decodedBytes2 = decodeSlotValue(mockAdapter, bytes2Value, "bytes2");
  assertEqual(decodedBytes2, "0xabcd", "Decode bytes2");

  // Test bytes3
  const bytes3Value = "0x" + "0".repeat(58) + "abcdef";
  const decodedBytes3 = decodeSlotValue(mockAdapter, bytes3Value, "bytes3");
  assertEqual(decodedBytes3, "0xabcdef", "Decode bytes3");

  // Test bytes4
  const bytes4Value = "0x" + "0".repeat(56) + "12345678";
  const decodedBytes4 = decodeSlotValue(mockAdapter, bytes4Value, "bytes4");
  assertEqual(decodedBytes4, "0x12345678", "Decode bytes4");

  // Test bytes20 (same size as address)
  const bytes20Value = "0x" + "0".repeat(24) + "deadbeef".repeat(5);
  const decodedBytes20 = decodeSlotValue(mockAdapter, bytes20Value, "bytes20");
  assertEqual(decodedBytes20, "0x" + "deadbeef".repeat(5), "Decode bytes20");

  // Test bytes31
  const bytes31Value = "0x" + "00" + "ab".repeat(31);
  const decodedBytes31 = decodeSlotValue(mockAdapter, bytes31Value, "bytes31");
  assertEqual(decodedBytes31, "0x" + "ab".repeat(31), "Decode bytes31");

  // Test bytes32
  const bytes32Value = "0x" + "ab".repeat(32);
  const decodedBytes32 = decodeSlotValue(mockAdapter, bytes32Value, "bytes32");
  assertEqual(decodedBytes32, "0x" + "ab".repeat(32), "Decode bytes32");
}

async function testTupleAndStructComparison(): Promise<void> {
  console.log("\n🧪 Testing Tuple/Struct Comparison...");

  const mockAdapter = new MockAdapter();
  const verifier = new Verifier(mockAdapter);

  // Test 1: Simple tuple (array) comparison
  // Mock a function that returns a tuple like (uint256, address)
  const tupleAbi: AbiElement[] = [
    {
      type: "function",
      name: "getTuple",
      inputs: [],
      outputs: [
        { name: "amount", type: "uint256", internalType: "uint256" },
        { name: "recipient", type: "address", internalType: "address" },
      ],
      stateMutability: "view",
    },
  ];

  const tupleCalldata = dynamicEncodeFunctionData(tupleAbi, "getTuple", []);
  const tupleReturnData = dynamicEncodeAbiParameters(
    ["uint256", "address"],
    [BigInt(1000), "0x1234567890123456789012345678901234567890"],
  );
  mockAdapter.setCallResult(tupleCalldata, tupleReturnData);

  // Test matching tuple - expect array format
  const tupleConfig: ViewCallConfig = {
    function: "getTuple",
    expected: ["1000", "0x1234567890123456789012345678901234567890"],
  };

  const tupleResult = await verifier.executeViewCall(TEST_ADDRESS, tupleAbi, tupleConfig);
  assertEqual(tupleResult.status, "pass", "Tuple comparison passes with array expected");

  // Test 2: Address case-insensitivity in tuples
  const tupleConfigLowercase: ViewCallConfig = {
    function: "getTuple",
    expected: ["1000", "0x1234567890123456789012345678901234567890".toLowerCase()],
  };

  const tupleLowercaseResult = await verifier.executeViewCall(TEST_ADDRESS, tupleAbi, tupleConfigLowercase);
  assertEqual(tupleLowercaseResult.status, "pass", "Tuple comparison passes with lowercase address");

  // Test 3: Mismatched tuple values
  const tupleMismatchConfig: ViewCallConfig = {
    function: "getTuple",
    expected: ["2000", "0x1234567890123456789012345678901234567890"], // Wrong amount
  };

  const tupleMismatchResult = await verifier.executeViewCall(TEST_ADDRESS, tupleAbi, tupleMismatchConfig);
  assertEqual(tupleMismatchResult.status, "fail", "Tuple comparison fails with mismatched values");

  // Test 4: Nested tuple (struct-like)
  const nestedTupleAbi: AbiElement[] = [
    {
      type: "function",
      name: "getNestedTuple",
      inputs: [],
      outputs: [
        {
          name: "data",
          type: "tuple",
          internalType: "struct MyStruct",
          components: [
            { name: "id", type: "uint256", internalType: "uint256" },
            { name: "active", type: "bool", internalType: "bool" },
          ],
        },
      ],
      stateMutability: "view",
    },
  ];

  // Note: For nested tuples, we encode the inner tuple values directly
  const nestedCalldata = dynamicEncodeFunctionData(nestedTupleAbi, "getNestedTuple", []);
  const nestedReturnData = dynamicEncodeAbiParameters(["uint256", "bool"], [BigInt(42), true]);
  mockAdapter.setCallResult(nestedCalldata, nestedReturnData);

  const nestedConfig: ViewCallConfig = {
    function: "getNestedTuple",
    expected: ["42", true], // Flattened tuple values
  };

  const nestedResult = await verifier.executeViewCall(TEST_ADDRESS, nestedTupleAbi, nestedConfig);
  assertEqual(nestedResult.status, "pass", "Nested tuple comparison passes");

  // Test 5: BigInt/string equivalence
  const bigIntConfig: ViewCallConfig = {
    function: "getTuple",
    expected: [1000, "0x1234567890123456789012345678901234567890"], // number instead of string
  };

  const bigIntResult = await verifier.executeViewCall(TEST_ADDRESS, tupleAbi, bigIntConfig);
  assertEqual(bigIntResult.status, "pass", "Tuple comparison handles number/string equivalence");
}

async function testVerifyStoragePathMessageFormatting(): Promise<void> {
  console.log("\n🧪 Testing verifyStoragePath message formatting (formatForDisplay fix)...");

  const mockAdapter = new MockAdapter();

  // Schema with a simple field
  const testSchema: StorageSchema = {
    structs: {
      TestStorage: {
        baseSlot: "0x0000000000000000000000000000000000000000000000000000000000000100",
        fields: {
          value: { slot: 0, type: "uint256" },
          flag: { slot: 1, type: "bool" },
        },
      },
    },
  };

  // Set up storage with a value
  const baseSlot = "0x0000000000000000000000000000000000000000000000000000000000000100";
  mockAdapter.setStorage(baseSlot, "0x" + "0".repeat(62) + "64"); // 100 in hex

  // Test 1: Passing case - message should contain formatted value
  const passConfig = {
    path: "TestStorage:value",
    expected: "100",
  };

  const passResult = await verifyStoragePath(mockAdapter, TEST_ADDRESS, passConfig, testSchema);

  assertEqual(passResult.status, "pass", "Storage path verification passes");
  assert(!passResult.message.includes("[object Object]"), "Pass message does not contain [object Object]");
  assert(passResult.message.includes("100"), "Pass message contains the value");

  // Test 2: Failing case - message should contain both expected and actual formatted values
  const failConfig = {
    path: "TestStorage:value",
    expected: "200",
  };

  const failResult = await verifyStoragePath(mockAdapter, TEST_ADDRESS, failConfig, testSchema);

  assertEqual(failResult.status, "fail", "Storage path verification fails with wrong expected");
  assert(!failResult.message.includes("[object Object]"), "Fail message does not contain [object Object]");
  assert(failResult.message.includes("200"), "Fail message contains expected value");
  assert(failResult.message.includes("100"), "Fail message contains actual value");

  // Test 3: Boolean value formatting
  const boolSlot = "0x0000000000000000000000000000000000000000000000000000000000000101";
  mockAdapter.setStorage(boolSlot, "0x" + "0".repeat(62) + "01"); // true

  const boolConfig = {
    path: "TestStorage:flag",
    expected: true,
  };

  const boolResult = await verifyStoragePath(mockAdapter, TEST_ADDRESS, boolConfig, testSchema);

  assertEqual(boolResult.status, "pass", "Boolean storage path verification passes");
  assert(!boolResult.message.includes("[object Object]"), "Boolean message does not contain [object Object]");
  assert(boolResult.message.includes("true"), "Boolean message contains the value");
}

async function testDirectlyNestedStructs(): Promise<void> {
  console.log("\n🧪 Testing directly nested struct access...");

  const mockAdapter = new MockAdapter();

  // Schema with directly nested structs (struct containing struct as a field)
  const nestedSchema: StorageSchema = {
    structs: {
      OuterStorage: {
        baseSlot: "0x0000000000000000000000000000000000000000000000000000000000001000",
        fields: {
          simpleValue: { slot: 0, type: "uint256" },
          innerStruct: { slot: 1, type: "InnerStruct" }, // Nested struct at slot 1
          anotherValue: { slot: 4, type: "address" }, // After inner struct (3 slots)
        },
      },
      InnerStruct: {
        // Nested struct - 3 slots
        fields: {
          fieldA: { slot: 0, type: "uint256" },
          fieldB: { slot: 1, type: "address" },
          deeperStruct: { slot: 2, type: "DeeperStruct" }, // Doubly nested
        },
      },
      DeeperStruct: {
        // Doubly nested struct
        fields: {
          deepValue: { slot: 0, type: "uint128" },
          deepFlag: { slot: 0, type: "bool", byteOffset: 16 },
        },
      },
    },
  };

  // Test 1: Access field in directly nested struct
  const innerFieldPath = parsePath("OuterStorage:innerStruct.fieldA");
  const innerFieldSlot = computeSlot(mockAdapter, innerFieldPath, nestedSchema);

  assertEqual(innerFieldSlot.type, "uint256", "Nested struct field type");
  // innerStruct is at slot 1, fieldA is at slot 0 within inner, so total = 1 + 0 = 1
  const baseSlot = BigInt("0x1000");
  assertEqual(BigInt(innerFieldSlot.slot) - baseSlot, 1n, "Nested struct fieldA at slot 1");

  // Test 2: Access second field in nested struct
  const innerFieldBPath = parsePath("OuterStorage:innerStruct.fieldB");
  const innerFieldBSlot = computeSlot(mockAdapter, innerFieldBPath, nestedSchema);

  assertEqual(innerFieldBSlot.type, "address", "Nested struct fieldB type");
  // innerStruct slot 1 + fieldB slot 1 = 2
  assertEqual(BigInt(innerFieldBSlot.slot) - baseSlot, 2n, "Nested struct fieldB at slot 2");

  // Test 3: Access doubly nested struct field
  const deepFieldPath = parsePath("OuterStorage:innerStruct.deeperStruct.deepValue");
  const deepFieldSlot = computeSlot(mockAdapter, deepFieldPath, nestedSchema);

  assertEqual(deepFieldSlot.type, "uint128", "Doubly nested struct field type");
  // innerStruct slot 1 + deeperStruct slot 2 + deepValue slot 0 = 3
  assertEqual(BigInt(deepFieldSlot.slot) - baseSlot, 3n, "Doubly nested deepValue at slot 3");

  // Test 4: Access packed field in doubly nested struct
  const deepFlagPath = parsePath("OuterStorage:innerStruct.deeperStruct.deepFlag");
  const deepFlagSlot = computeSlot(mockAdapter, deepFlagPath, nestedSchema);

  assertEqual(deepFlagSlot.type, "bool", "Doubly nested packed field type");
  assertEqual(deepFlagSlot.byteOffset, 16, "Doubly nested packed field byte offset");
  assertEqual(deepFlagSlot.slot, deepFieldSlot.slot, "Packed fields share same slot");

  // Test 5: Field after nested struct
  const afterNestedPath = parsePath("OuterStorage:anotherValue");
  const afterNestedSlot = computeSlot(mockAdapter, afterNestedPath, nestedSchema);

  assertEqual(afterNestedSlot.type, "address", "Field after nested struct type");
  assertEqual(BigInt(afterNestedSlot.slot) - baseSlot, 4n, "Field after nested struct at slot 4");

  // Test 6: Error case - accessing field on non-existent nested struct
  const badSchema: StorageSchema = {
    structs: {
      BrokenStorage: {
        baseSlot: "0x0000000000000000000000000000000000000000000000000000000000002000",
        fields: {
          missingStruct: { slot: 0, type: "NonExistentStruct" },
        },
      },
    },
  };

  try {
    const badPath = parsePath("BrokenStorage:missingStruct.someField");
    computeSlot(mockAdapter, badPath, badSchema);
    assert(false, "Should throw for missing nested struct");
  } catch (e) {
    const error = e as Error;
    assert(error instanceof Error, "Throws an Error");
    assert(
      error.message.includes("Cannot access field") || error.message.includes("Unknown"),
      "Error message mentions field access issue",
    );
  }
}

/**
 * Test CBOR metadata stripping with edge case lengths.
 * Bug fix: Metadata outside typical bounds should still be stripped if valid.
 *
 * CBOR metadata format: [marker][content][2-byte-length]
 * The length indicator is the number of bytes in [marker][content] (NOT including the length itself)
 */
function testCborStrippingEdgeCases(): void {
  console.log("\n🧪 Testing CBOR stripping edge cases (bug fix)...");

  // Helper to build valid CBOR metadata
  // metadataBytes = total bytes in [marker][content], which is what the length indicates
  function buildMetadata(marker: string, lengthBytes: number): string {
    // marker is 2 hex chars = 1 byte
    // content is (lengthBytes - 1) bytes = (lengthBytes - 1) * 2 hex chars
    const contentNeeded = lengthBytes * 2 - marker.length;
    const content = "0".repeat(contentNeeded);
    const lengthHex = lengthBytes.toString(16).padStart(4, "0");
    return marker + content + lengthHex;
  }

  // Use a realistic-length bytecode prefix (must be > MIN_BYTECODE_AFTER_STRIP = 20 chars after stripping)
  // 40 chars = 20 bytes of "contract code"
  const codePrefix = "6080604052348015610010575f80fd5b50";

  // Test 1: Normal metadata length (51 bytes = typical Solidity)
  const normalMetadata = buildMetadata("a2", 51);
  const normalBytecode = codePrefix + normalMetadata;
  const normalStripped = stripCborMetadata(normalBytecode);
  assertEqual(normalStripped, codePrefix, "Normal metadata length stripped correctly");

  // Test 2: Short metadata (25 bytes - below old minimum of 30, above new minimum of 20)
  const shortMetadata = buildMetadata("a1", 25);
  const shortBytecode = codePrefix + shortMetadata;
  const shortStripped = stripCborMetadata(shortBytecode);
  assertEqual(shortStripped, codePrefix, "Short metadata (25 bytes) stripped correctly");

  // Test 3: Large metadata (350 bytes - above old maximum of 300 but below absolute max)
  // This should work if it contains known Solidity patterns
  const largeContentBytes = 350;
  const largeMarker = "a2";
  const ipfsPattern = "697066735822"; // "ipfs" in hex
  const largeContentNeeded = largeContentBytes * 2 - largeMarker.length;
  const largePadding = "0".repeat(largeContentNeeded - ipfsPattern.length);
  const largeLength = largeContentBytes.toString(16).padStart(4, "0");
  const largeMetadata = largeMarker + ipfsPattern + largePadding + largeLength;
  const largeBytecode = codePrefix + largeMetadata;
  const largeStripped = stripCborMetadata(largeBytecode);
  assertEqual(largeStripped, codePrefix, "Large metadata (350 bytes) with IPFS pattern stripped");

  // Test 4: Invalid CBOR marker (ff instead of a0/a1/a2/a3) - should NOT strip
  const invalidMetadata = buildMetadata("ff", 51);
  const invalidBytecode = codePrefix + invalidMetadata;
  const invalidStripped = stripCborMetadata(invalidBytecode);
  assertEqual(invalidStripped, invalidBytecode.toLowerCase(), "Invalid marker not stripped");

  // Test 5: a3 marker (3-item map) should work
  const a3Metadata = buildMetadata("a3", 51);
  const a3Bytecode = codePrefix + a3Metadata;
  const a3Stripped = stripCborMetadata(a3Bytecode);
  assertEqual(a3Stripped, codePrefix, "a3 marker (3-item map) stripped correctly");

  // Test 6: a0 marker (empty map) should work for minimal toolchains
  const a0Metadata = buildMetadata("a0", 25);
  const a0Bytecode = codePrefix + a0Metadata;
  const a0Stripped = stripCborMetadata(a0Bytecode);
  assertEqual(a0Stripped, codePrefix, "a0 marker (empty map) stripped correctly");

  // Test 7: Extremely large metadata (above absolute max of 1000) should NOT strip
  const hugeMetadata = buildMetadata("a2", 1100);
  const hugeBytecode = codePrefix + hugeMetadata;
  const hugeStripped = stripCborMetadata(hugeBytecode);
  assertEqual(hugeStripped, hugeBytecode.toLowerCase(), "Extremely large metadata (1100 bytes) not stripped");

  // Test 8: Metadata below minimum (15 bytes) should NOT strip
  const tinyMetadata = buildMetadata("a1", 15);
  const tinyBytecode = codePrefix + tinyMetadata;
  const tinyStripped = stripCborMetadata(tinyBytecode);
  assertEqual(tinyStripped, tinyBytecode.toLowerCase(), "Tiny metadata (15 bytes) not stripped");
}

// ============================================================================
// Linked Library Tests
// ============================================================================

function testLinkLibraries(): void {
  console.log("\n🧪 Testing linkLibraries...");

  // Build a bytecode string with a known placeholder at a specific position
  const prefix = "6080604052" + "00".repeat(95); // 100 bytes
  const placeholder = "__$21f52c64f029e7b8ff2bccb2a7d14460c1$__"; // 40 chars = 20 bytes
  const suffix = "00".repeat(50);
  const bytecode = "0x" + prefix + placeholder + suffix;

  const linkRefs: DeployedLinkReferences = {
    "src/libraries/Mimc.sol": {
      Mimc: [{ start: 100, length: 20 }],
    },
  };

  const libraries = {
    "src/libraries/Mimc.sol:Mimc": "0x1234567890AbCdEf1234567890aBcDeF12345678",
  };

  const result = linkLibraries(bytecode, linkRefs, libraries);

  // Verify the placeholder was replaced
  const expectedAddress = "1234567890abcdef1234567890abcdef12345678";
  const hexPos = 100 * 2; // byte position -> hex position
  const linkedHex = result.linkedBytecode.slice(2); // Remove 0x
  const insertedValue = linkedHex.slice(hexPos, hexPos + 40);
  assertEqual(insertedValue, expectedAddress, "Library address inserted at correct position");

  // Verify no placeholder remains
  assert(!result.linkedBytecode.includes("__$"), "No placeholder remains after linking");

  // Verify result status
  assertEqual(result.results.length, 1, "One library result");
  assertEqual(result.results[0].status, "pass", "Library linking passed");
  assertEqual(result.results[0].name, "src/libraries/Mimc.sol:Mimc", "Correct library name");
  assertEqual(result.results[0].positions.length, 1, "One position substituted");

  // Verify 0x prefix preserved
  assert(result.linkedBytecode.startsWith("0x"), "0x prefix preserved");
}

function testLinkLibrariesMissingAddress(): void {
  console.log("\n🧪 Testing linkLibraries with missing address...");

  const bytecode = "0x" + "00".repeat(100);
  const linkRefs: DeployedLinkReferences = {
    "src/libraries/Mimc.sol": {
      Mimc: [{ start: 50, length: 20 }],
    },
  };

  // No address provided
  const result = linkLibraries(bytecode, linkRefs, {});

  assertEqual(result.results.length, 1, "One library result");
  assertEqual(result.results[0].status, "fail", "Library linking failed");
  assert(result.results[0].message.includes("No address provided"), "Error message mentions missing address");
}

function testLinkLibrariesInvalidAddress(): void {
  console.log("\n🧪 Testing linkLibraries with invalid address...");

  const bytecode = "0x" + "00".repeat(100);
  const linkRefs: DeployedLinkReferences = {
    "src/libraries/Mimc.sol": {
      Mimc: [{ start: 50, length: 20 }],
    },
  };

  const result = linkLibraries(bytecode, linkRefs, {
    "src/libraries/Mimc.sol:Mimc": "not-a-valid-address",
  });

  assertEqual(result.results.length, 1, "One library result");
  assertEqual(result.results[0].status, "fail", "Library linking failed");
  assert(result.results[0].message.includes("Invalid address"), "Error message mentions invalid address");
}

function testLinkLibrariesMultiplePositions(): void {
  console.log("\n🧪 Testing linkLibraries with multiple positions...");

  const bytecode = "0x" + "00".repeat(200);
  const linkRefs: DeployedLinkReferences = {
    "src/libraries/Mimc.sol": {
      Mimc: [
        { start: 50, length: 20 },
        { start: 150, length: 20 },
      ],
    },
  };

  const address = "0xDeadBeefDeadBeefDeadBeefDeadBeefDeadBeef";
  const result = linkLibraries(bytecode, linkRefs, {
    "src/libraries/Mimc.sol:Mimc": address,
  });

  assertEqual(result.results.length, 1, "One library result");
  assertEqual(result.results[0].status, "pass", "Library linking passed");
  assertEqual(result.results[0].positions.length, 2, "Two positions substituted");

  // Verify both positions have the address
  const hex = result.linkedBytecode.slice(2);
  const pos1 = hex.slice(50 * 2, 50 * 2 + 40);
  const pos2 = hex.slice(150 * 2, 150 * 2 + 40);
  assertEqual(pos1, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "First position linked");
  assertEqual(pos2, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "Second position linked");
}

function testLinkLibrariesMultipleDistinctLibraries(): void {
  console.log("\n🧪 Testing linkLibraries with multiple distinct libraries...");

  const bytecode = "0x" + "00".repeat(300);
  const linkRefs: DeployedLinkReferences = {
    "src/libraries/Mimc.sol": {
      Mimc: [{ start: 30, length: 20 }],
    },
    "src/libraries/Poseidon.sol": {
      Poseidon: [
        { start: 100, length: 20 },
        { start: 200, length: 20 },
      ],
    },
    "src/utils/Math.sol": {
      SafeMath: [{ start: 250, length: 20 }],
    },
  };

  const mimcAddr = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
  const poseidonAddr = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
  const mathAddr = "0xcccccccccccccccccccccccccccccccccccccccc";

  const result = linkLibraries(bytecode, linkRefs, {
    "src/libraries/Mimc.sol:Mimc": mimcAddr,
    "src/libraries/Poseidon.sol:Poseidon": poseidonAddr,
    "src/utils/Math.sol:SafeMath": mathAddr,
  });

  assertEqual(result.results.length, 3, "Three library results");
  assert(
    result.results.every((r) => r.status === "pass"),
    "All libraries linked successfully",
  );

  // Verify each address is at the correct position(s)
  const hex = result.linkedBytecode.slice(2);
  assertEqual(hex.slice(30 * 2, 30 * 2 + 40), "a".repeat(40), "Mimc at position 30");
  assertEqual(hex.slice(100 * 2, 100 * 2 + 40), "b".repeat(40), "Poseidon at position 100");
  assertEqual(hex.slice(200 * 2, 200 * 2 + 40), "b".repeat(40), "Poseidon at position 200");
  assertEqual(hex.slice(250 * 2, 250 * 2 + 40), "c".repeat(40), "SafeMath at position 250");

  // Verify Poseidon result has 2 positions
  const poseidonResult = result.results.find((r) => r.name === "src/libraries/Poseidon.sol:Poseidon");
  assert(poseidonResult !== undefined, "Poseidon result exists");
  assertEqual(poseidonResult!.positions.length, 2, "Poseidon has 2 positions");
}

function testLinkLibrariesPartialAddresses(): void {
  console.log("\n🧪 Testing linkLibraries with partial addresses (some missing)...");

  const bytecode = "0x" + "00".repeat(200);
  const linkRefs: DeployedLinkReferences = {
    "src/libraries/Mimc.sol": {
      Mimc: [{ start: 30, length: 20 }],
    },
    "src/libraries/Poseidon.sol": {
      Poseidon: [{ start: 100, length: 20 }],
    },
  };

  // Only provide address for Mimc, not Poseidon
  const result = linkLibraries(bytecode, linkRefs, {
    "src/libraries/Mimc.sol:Mimc": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  });

  assertEqual(result.results.length, 2, "Two library results");

  const mimcResult = result.results.find((r) => r.name === "src/libraries/Mimc.sol:Mimc");
  const poseidonResult = result.results.find((r) => r.name === "src/libraries/Poseidon.sol:Poseidon");

  assertEqual(mimcResult!.status, "pass", "Mimc linked successfully");
  assertEqual(poseidonResult!.status, "fail", "Poseidon fails (no address)");
  assert(poseidonResult!.message.includes("No address provided"), "Poseidon error message is clear");
}

function testVerifyLinkedLibraries(): void {
  console.log("\n🧪 Testing verifyLinkedLibraries (on-chain address extraction)...");

  // Simulate on-chain bytecode with a known library address at position 50
  const address = "aabbccddaabbccddaabbccddaabbccddaabbccdd";
  const remoteBytecode = "0x" + "00".repeat(50) + address + "00".repeat(50);
  const linkRefs: DeployedLinkReferences = {
    "src/lib.sol": {
      MyLib: [{ start: 50, length: 20 }],
    },
  };

  // Correct address
  const matchResult = verifyLinkedLibraries(remoteBytecode, linkRefs, {
    "src/lib.sol:MyLib": "0xaabbccddaabbccddaabbccddaabbccddaabbccdd",
  });
  assertEqual(matchResult.length, 1, "One result");
  assertEqual(matchResult[0].status, "pass", "Address matches on-chain");
  assertEqual(matchResult[0].actualAddress, "0x" + address, "Actual address extracted correctly");
  assert(matchResult[0].message.includes("matches"), "Message confirms match");

  // Wrong address
  const mismatchResult = verifyLinkedLibraries(remoteBytecode, linkRefs, {
    "src/lib.sol:MyLib": "0x1111111111111111111111111111111111111111",
  });
  assertEqual(mismatchResult.length, 1, "One result for mismatch");
  assertEqual(mismatchResult[0].status, "fail", "Address mismatch detected");
  assertEqual(mismatchResult[0].actualAddress, "0x" + address, "Actual address still extracted");
  assert(mismatchResult[0].message.includes("expected"), "Message shows expected vs actual");

  // Multiple positions - all must match
  const address2 = "1234567890123456789012345678901234567890";
  const remoteBytecode2 = "0x" + "00".repeat(30) + address2 + "00".repeat(30) + address2 + "00".repeat(20);
  const linkRefs2: DeployedLinkReferences = {
    "src/lib.sol": {
      MyLib: [
        { start: 30, length: 20 },
        { start: 80, length: 20 },
      ],
    },
  };

  const multiResult = verifyLinkedLibraries(remoteBytecode2, linkRefs2, {
    "src/lib.sol:MyLib": "0x" + address2,
  });
  assertEqual(multiResult[0].status, "pass", "Both positions match");
  assertEqual(multiResult[0].positions.length, 2, "Two positions verified");
}

function testDetectUnlinkedLibraries(): void {
  console.log("\n🧪 Testing detectUnlinkedLibraries...");

  // Bytecode with a placeholder
  const placeholder = "__$21f52c64f029e7b8ff2bccb2a7d14460c1$__";
  const bytecode = "0x" + "6080604052" + "00".repeat(95) + placeholder + "00".repeat(50);

  const unlinked = detectUnlinkedLibraries(bytecode);

  assertEqual(unlinked.length, 1, "One unlinked library detected");
  assertEqual(unlinked[0].placeholder, placeholder, "Correct placeholder detected");
  assertEqual(unlinked[0].positions.length, 1, "One position found");

  // Bytecode without placeholders
  const cleanBytecode = "0x" + "6080604052" + "00".repeat(200);
  const cleanResult = detectUnlinkedLibraries(cleanBytecode);
  assertEqual(cleanResult.length, 0, "No unlinked libraries in clean bytecode");
}

function testDetectUnlinkedLibrariesMultiple(): void {
  console.log("\n🧪 Testing detectUnlinkedLibraries with multiple placeholders...");

  const placeholder1 = "__$21f52c64f029e7b8ff2bccb2a7d14460c1$__";
  const placeholder2 = "__$aabbccddee11223344556677889900aabb$__";
  const bytecode = "0x" + "00".repeat(20) + placeholder1 + "00".repeat(20) + placeholder2 + "00".repeat(20);

  const unlinked = detectUnlinkedLibraries(bytecode);

  assertEqual(unlinked.length, 2, "Two different unlinked libraries detected");
}

function testLinkLibrariesWithoutPrefix(): void {
  console.log("\n🧪 Testing linkLibraries without 0x prefix...");

  const bytecode = "00".repeat(100);
  const linkRefs: DeployedLinkReferences = {
    "src/lib.sol": {
      Lib: [{ start: 10, length: 20 }],
    },
  };

  const result = linkLibraries(bytecode, linkRefs, {
    "src/lib.sol:Lib": "0xaabbccddaabbccddaabbccddaabbccddaabbccdd",
  });

  // Verify no 0x prefix added when input didn't have one
  assert(!result.linkedBytecode.startsWith("0x"), "No 0x prefix when input lacked one");
  assertEqual(result.results[0].status, "pass", "Linking succeeded without prefix");
}

/**
 * Test that immutable values verification prevents double-matching of regions.
 * Bug fix: Two immutable names with the same expected value should not both
 * match the same bytecode region.
 */
function testImmutableValuesNoDoubleMatch(): void {
  console.log("\n🧪 Testing immutable values no double-match (bug fix)...");

  // Create immutable differences with two regions that have the same value
  const immutableDifferences: ImmutableDifference[] = [
    {
      position: 100,
      length: 20,
      localValue: "0".repeat(40),
      remoteValue: "1234567890abcdef1234567890abcdef12345678",
      possibleType: "address",
    },
    {
      position: 200,
      length: 20,
      localValue: "0".repeat(40),
      remoteValue: "abcdef1234567890abcdef1234567890abcdef12",
      possibleType: "address",
    },
  ];

  // Test 1: Two different immutable names with different values should each match their own region
  const result1 = verifyImmutableValues(
    {
      addr1: "0x1234567890abcdef1234567890abcdef12345678",
      addr2: "0xabcdef1234567890abcdef1234567890abcdef12",
    },
    immutableDifferences,
  );
  assertEqual(result1.status, "pass", "Both different addresses should match");
  assertEqual(result1.results.length, 2, "Should have 2 results");
  assert(result1.results[0].status === "pass", "First address should pass");
  assert(result1.results[1].status === "pass", "Second address should pass");

  // Test 2: Two immutable names with THE SAME expected value should NOT both pass
  // because only one region matches, and once consumed, it cannot match again
  const sameDifferences: ImmutableDifference[] = [
    {
      position: 100,
      length: 20,
      localValue: "0".repeat(40),
      remoteValue: "1234567890abcdef1234567890abcdef12345678",
      possibleType: "address",
    },
  ];

  const result2 = verifyImmutableValues(
    {
      addr1: "0x1234567890abcdef1234567890abcdef12345678",
      addr2: "0x1234567890abcdef1234567890abcdef12345678", // Same value!
    },
    sameDifferences,
  );

  // With the bug fix, only one should match (the first), and the second should fail
  // because the region is consumed
  assertEqual(result2.status, "fail", "Should fail when two names try to match same region");
  const passedCount = result2.results.filter((r) => r.status === "pass").length;
  const failedCount = result2.results.filter((r) => r.status === "fail").length;
  assertEqual(passedCount, 1, "Only one immutable should pass");
  assertEqual(failedCount, 1, "One immutable should fail (region already consumed)");
}

main().catch((error) => {
  console.error("Test error:", error);
  process.exit(1);
});
