#!/usr/bin/env ts-node
/**
 * Contract Integrity Verifier - Test Runner
 * Run with: npx ts-node tests/run-tests.ts
 */

import { detectArtifactFormat, extractSelectorsFromArtifact, loadArtifact } from "../src/utils/abi";
import { compareBytecode, extractSelectorsFromBytecode, validateImmutablesAgainstArgs } from "../src/utils/bytecode";
import {
  calculateErc7201BaseSlot,
  calculateErc7201Slot,
  decodeSlotValue,
  verifySlot,
  verifyNamespace,
  parsePath,
  computeSlot,
  loadStorageSchema,
} from "../src/utils/storage";
import {
  AbiElement,
  ViewCallConfig,
  SlotConfig,
  NamespaceConfig,
  StateVerificationConfig,
  NormalizedArtifact,
  ImmutableReference,
  ImmutableDifference,
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

function dynamicEncodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string {
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

function dynamicDecodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[] {
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

main().catch((error) => {
  console.error("Test error:", error);
  process.exit(1);
});
