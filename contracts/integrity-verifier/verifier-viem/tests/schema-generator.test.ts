/**
 * Schema Generator Tests
 *
 * Tests the schema generator with complex ERC-7201 namespaced storage structures,
 * including nested structs and mappings.
 */

import { generateSchema, parseSoliditySource, calculateErc7201BaseSlot } from "../src/tools";

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

// ============================================================================
// Test Fixtures - Complex Solidity Storage Structs
// ============================================================================

const SIMPLE_STRUCT_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:test.storage.SimpleStorage
struct SimpleStorage {
    uint256 value;
    address owner;
    bool paused;
}
`;

const NESTED_STRUCT_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

struct Lowest {
    uint256 a;
    uint256 b;
    uint256 c;
}

struct Middle {
    mapping(uint256 => Lowest) lower;
}

/// @custom:storage-location erc7201:test.storage.TopLevel
struct TopLevelStorage {
    mapping(uint256 => address) numberedAddresses;
    Middle middle;
    uint256 totalCount;
}
`;

const COMPLEX_MAPPINGS_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

struct TokenData {
    uint256 balance;
    uint256 lastUpdated;
    bool isActive;
}

struct UserProfile {
    address wallet;
    mapping(address => TokenData) tokenBalances;
    mapping(bytes32 => bool) permissions;
}

/// @custom:storage-location erc7201:test.storage.Registry
struct RegistryStorage {
    mapping(address => UserProfile) users;
    mapping(uint256 => mapping(address => uint256)) nestedMapping;
    address[] registeredAddresses;
    uint256 totalUsers;
}
`;

const PACKED_STORAGE_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:test.storage.PackedVars
struct PackedVarsStorage {
    uint8 status;
    bool flag1;
    bool flag2;
    address owner;
    uint64 timestamp;
    uint32 counter;
}
`;

// Additional packing scenarios
const BYTES4_ADDRESS_PACKING_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:test.storage.Bytes4Packing
struct Bytes4PackingStorage {
    bytes4 selector;
    address target;
    uint64 nonce;
}
`;

const EXACT_SLOT_FILL_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:test.storage.ExactFill
struct ExactFillStorage {
    uint96 amount;
    address recipient;
}
`;

const OVERFLOW_NEXT_SLOT_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:test.storage.Overflow
struct OverflowStorage {
    uint128 bigValue;
    address target;
    uint64 timestamp;
}
`;

const MULTIPLE_SMALL_TYPES_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:test.storage.SmallTypes
struct SmallTypesStorage {
    bool a;
    bool b;
    bool c;
    bool d;
    uint8 e;
    uint8 f;
    uint16 g;
    uint32 h;
    uint64 i;
    uint128 j;
}
`;

// Enum test source - simulates YieldProviderVendor pattern
const ENUM_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

enum YieldProviderVendor {
    UNUSED,
    LIDO_STVAULT
}

enum LargeEnum {
    A, B, C, D, E, F, G, H, I, J,
    K, L, M, N, O, P, Q, R, S, T,
    U, V, W, X, Y, Z
}

/// @custom:storage-location erc7201:test.storage.YieldProvider
struct YieldProviderStorage {
    YieldProviderVendor yieldProviderVendor;
    bool isStakingPaused;
    bool isOssificationInitiated;
    bool isOssified;
    address primaryEntrypoint;
    address ossifiedEntrypoint;
    uint96 yieldProviderIndex;
    uint256 userFunds;
}
`;

// Cross-file enum test - enum in separate file
const ENUM_FILE_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

enum Status {
    PENDING,
    ACTIVE,
    COMPLETE,
    CANCELLED
}
`;

const STRUCT_USING_ENUM_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:test.storage.Task
struct TaskStorage {
    Status status;
    uint8 priority;
    address assignee;
    uint256 deadline;
}
`;

const MULTIPLE_NAMESPACES_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:app.storage.Config
struct ConfigStorage {
    uint256 maxLimit;
    uint256 minLimit;
    address admin;
}

/// @custom:storage-location erc7201:app.storage.State
struct StateStorage {
    uint256 currentValue;
    bool isActive;
    mapping(address => uint256) balances;
}

/// @custom:storage-location erc7201:app.storage.Access
struct AccessStorage {
    mapping(bytes32 => mapping(address => bool)) roleMembers;
    mapping(address => bytes32[]) userRoles;
}
`;

const EXPLICIT_CONSTANT_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @custom:storage-location erc7201:linea.storage.LineaRollupYieldExtensionStorage
struct LineaRollupYieldExtensionStorage {
    address _yieldManager;
}

bytes32 private constant LineaRollupYieldExtensionStorageStorageLocation =
    0x594904a11ae10ad7613c91ac3c92c7c3bba397934d377ce6d3e0aaffbc17df00;
`;

// ============================================================================
// Tests
// ============================================================================

function testSimpleStruct(): void {
  console.log("\n=== Simple Struct Test ===");

  const { schema, warnings } = parseSoliditySource(SIMPLE_STRUCT_SOURCE, "SimpleStorage.sol");

  assert(warnings.length === 0, "No warnings generated");
  assert("SimpleStorage" in schema.structs, "SimpleStorage struct found");

  const struct = schema.structs["SimpleStorage"];
  assert(struct.namespace === "test.storage.SimpleStorage", "Namespace extracted correctly");
  assert(struct.baseSlot !== undefined, "baseSlot calculated");
  assert(struct.baseSlot!.startsWith("0x"), "baseSlot is hex string");
  assert(struct.baseSlot!.length === 66, "baseSlot is 32 bytes (66 chars with 0x)");

  // Verify fields - note that address (20 bytes) and bool (1 byte) pack together
  assertEqual(struct.fields["value"], { slot: 0, type: "uint256" }, "value field correct");
  // address is first in packed slot 1, gets explicit byteOffset: 0
  assertEqual(struct.fields["owner"], { slot: 1, type: "address", byteOffset: 0 }, "owner field correct");
  // bool (1 byte) packs after address (20 bytes) in the same slot
  assertEqual(struct.fields["paused"], { slot: 1, type: "bool", byteOffset: 20 }, "paused field packed correctly");
}

function testNestedStructWithMappings(): void {
  console.log("\n=== Nested Struct with Mappings Test ===");

  const { schema, warnings } = parseSoliditySource(NESTED_STRUCT_SOURCE, "NestedStorage.sol");

  assert(warnings.length === 0, "No warnings generated");

  // Check all structs are found
  assert("Lowest" in schema.structs, "Lowest struct found");
  assert("Middle" in schema.structs, "Middle struct found");
  assert("TopLevelStorage" in schema.structs, "TopLevelStorage struct found");

  // Lowest struct (no namespace)
  const lowest = schema.structs["Lowest"];
  assert(lowest.namespace === undefined, "Lowest has no namespace");
  assert(lowest.baseSlot === undefined, "Lowest has no baseSlot");
  assertEqual(lowest.fields["a"], { slot: 0, type: "uint256" }, "Lowest.a field correct");
  assertEqual(lowest.fields["b"], { slot: 1, type: "uint256" }, "Lowest.b field correct");
  assertEqual(lowest.fields["c"], { slot: 2, type: "uint256" }, "Lowest.c field correct");

  // Middle struct (has mapping)
  const middle = schema.structs["Middle"];
  assertEqual(
    middle.fields["lower"],
    { slot: 0, type: "mapping(uint256 => Lowest)" },
    "Middle.lower mapping field correct",
  );

  // TopLevelStorage struct (has namespace)
  const top = schema.structs["TopLevelStorage"];
  assert(top.namespace === "test.storage.TopLevel", "TopLevelStorage namespace correct");
  assert(top.baseSlot !== undefined, "TopLevelStorage has baseSlot");

  assertEqual(
    top.fields["numberedAddresses"],
    { slot: 0, type: "mapping(uint256 => address)" },
    "numberedAddresses mapping correct",
  );
  assertEqual(top.fields["middle"], { slot: 1, type: "Middle" }, "middle nested struct correct");
  assertEqual(top.fields["totalCount"], { slot: 2, type: "uint256" }, "totalCount field correct");
}

function testComplexMappings(): void {
  console.log("\n=== Complex Mappings Test ===");

  const { schema, warnings } = parseSoliditySource(COMPLEX_MAPPINGS_SOURCE, "ComplexMappings.sol");

  assert(warnings.length === 0, "No warnings generated");

  // Check RegistryStorage
  assert("RegistryStorage" in schema.structs, "RegistryStorage found");
  const registry = schema.structs["RegistryStorage"];

  assert(registry.namespace === "test.storage.Registry", "Namespace correct");
  assert(registry.baseSlot !== undefined, "baseSlot calculated");

  // Check mapping fields
  assertEqual(registry.fields["users"], { slot: 0, type: "mapping(address => UserProfile)" }, "users mapping correct");
  assertEqual(
    registry.fields["nestedMapping"],
    { slot: 1, type: "mapping(uint256 => mapping(address => uint256))" },
    "nestedMapping correct",
  );
  assertEqual(
    registry.fields["registeredAddresses"],
    { slot: 2, type: "address[]" },
    "registeredAddresses array correct",
  );
  assertEqual(registry.fields["totalUsers"], { slot: 3, type: "uint256" }, "totalUsers field correct");

  // Check TokenData struct
  const tokenData = schema.structs["TokenData"];
  assertEqual(tokenData.fields["balance"], { slot: 0, type: "uint256" }, "TokenData.balance correct");
  assertEqual(tokenData.fields["lastUpdated"], { slot: 1, type: "uint256" }, "TokenData.lastUpdated correct");
  assertEqual(tokenData.fields["isActive"], { slot: 2, type: "bool" }, "TokenData.isActive correct");

  // Check UserProfile struct
  const userProfile = schema.structs["UserProfile"];
  assertEqual(userProfile.fields["wallet"], { slot: 0, type: "address" }, "UserProfile.wallet correct");
  assertEqual(
    userProfile.fields["tokenBalances"],
    { slot: 1, type: "mapping(address => TokenData)" },
    "UserProfile.tokenBalances correct",
  );
  assertEqual(
    userProfile.fields["permissions"],
    { slot: 2, type: "mapping(bytes32 => bool)" },
    "UserProfile.permissions correct",
  );
}

function testPackedStorage(): void {
  console.log("\n=== Packed Storage Test ===");

  const { schema, warnings } = parseSoliditySource(PACKED_STORAGE_SOURCE, "PackedStorage.sol");

  assert(warnings.length === 0, "No warnings generated");

  const packed = schema.structs["PackedVarsStorage"];
  assert(packed.namespace === "test.storage.PackedVars", "Namespace correct");

  // All small types should be packed into slot 0
  // First field in packed slot gets explicit byteOffset: 0
  assertEqual(packed.fields["status"], { slot: 0, type: "uint8", byteOffset: 0 }, "status at slot 0");
  assertEqual(packed.fields["flag1"], { slot: 0, type: "bool", byteOffset: 1 }, "flag1 packed at offset 1");
  assertEqual(packed.fields["flag2"], { slot: 0, type: "bool", byteOffset: 2 }, "flag2 packed at offset 2");
  assertEqual(packed.fields["owner"], { slot: 0, type: "address", byteOffset: 3 }, "owner packed at offset 3");

  // These should be in the next slot (offset 3 + 20 = 23, then uint64 (8 bytes) fits, total 31)
  // Actually let me recalculate: uint8=1, bool=1, bool=1, address=20 = 23 bytes
  // uint64=8 bytes, 23+8=31 fits in 32
  assertEqual(packed.fields["timestamp"], { slot: 0, type: "uint64", byteOffset: 23 }, "timestamp packed at offset 23");
  // 23+8=31, counter is uint32=4 bytes, 31+4=35 > 32, so next slot (only field, no byteOffset)
  assertEqual(packed.fields["counter"], { slot: 1, type: "uint32" }, "counter at slot 1");
}

function testBytes4AddressPacking(): void {
  console.log("\n=== Bytes4 + Address Packing Test ===");

  const { schema, warnings } = parseSoliditySource(BYTES4_ADDRESS_PACKING_SOURCE, "Bytes4Packing.sol");

  assert(warnings.length === 0, "No warnings generated");

  const packed = schema.structs["Bytes4PackingStorage"];

  // bytes4 (4 bytes) + address (20 bytes) + uint64 (8 bytes) = 32 bytes exactly
  // First field in packed slot gets explicit byteOffset: 0
  assertEqual(packed.fields["selector"], { slot: 0, type: "bytes4", byteOffset: 0 }, "selector at slot 0 offset 0");
  assertEqual(packed.fields["target"], { slot: 0, type: "address", byteOffset: 4 }, "target at slot 0 offset 4");
  assertEqual(packed.fields["nonce"], { slot: 0, type: "uint64", byteOffset: 24 }, "nonce at slot 0 offset 24");
}

function testExactSlotFill(): void {
  console.log("\n=== Exact Slot Fill Test ===");

  const { schema, warnings } = parseSoliditySource(EXACT_SLOT_FILL_SOURCE, "ExactFill.sol");

  assert(warnings.length === 0, "No warnings generated");

  const packed = schema.structs["ExactFillStorage"];

  // uint96 (12 bytes) + address (20 bytes) = 32 bytes exactly fills one slot
  // First field in packed slot gets explicit byteOffset: 0
  assertEqual(packed.fields["amount"], { slot: 0, type: "uint96", byteOffset: 0 }, "amount at slot 0 offset 0");
  assertEqual(
    packed.fields["recipient"],
    { slot: 0, type: "address", byteOffset: 12 },
    "recipient at slot 0 offset 12",
  );
}

function testOverflowNextSlot(): void {
  console.log("\n=== Overflow to Next Slot Test ===");

  const { schema, warnings } = parseSoliditySource(OVERFLOW_NEXT_SLOT_SOURCE, "Overflow.sol");

  assert(warnings.length === 0, "No warnings generated");

  const packed = schema.structs["OverflowStorage"];

  // uint128 (16 bytes) + address (20 bytes) = 36 bytes > 32, so address goes to next slot
  assertEqual(packed.fields["bigValue"], { slot: 0, type: "uint128" }, "bigValue at slot 0");
  // address starts new slot 1 and packs with uint64, so gets byteOffset: 0
  assertEqual(packed.fields["target"], { slot: 1, type: "address", byteOffset: 0 }, "target at slot 1 (overflow)");
  // uint64 (8 bytes) can pack with address (20 bytes) = 28 bytes
  assertEqual(packed.fields["timestamp"], { slot: 1, type: "uint64", byteOffset: 20 }, "timestamp packed after target");
}

function testMultipleSmallTypes(): void {
  console.log("\n=== Multiple Small Types Packing Test ===");

  const { schema, warnings } = parseSoliditySource(MULTIPLE_SMALL_TYPES_SOURCE, "SmallTypes.sol");

  assert(warnings.length === 0, "No warnings generated");

  const packed = schema.structs["SmallTypesStorage"];

  // bool a (1) + bool b (1) + bool c (1) + bool d (1) + uint8 e (1) + uint8 f (1)
  // + uint16 g (2) + uint32 h (4) = 12 bytes total
  // First field in packed slot gets explicit byteOffset: 0
  assertEqual(packed.fields["a"], { slot: 0, type: "bool", byteOffset: 0 }, "a at slot 0 offset 0");
  assertEqual(packed.fields["b"], { slot: 0, type: "bool", byteOffset: 1 }, "b at offset 1");
  assertEqual(packed.fields["c"], { slot: 0, type: "bool", byteOffset: 2 }, "c at offset 2");
  assertEqual(packed.fields["d"], { slot: 0, type: "bool", byteOffset: 3 }, "d at offset 3");
  assertEqual(packed.fields["e"], { slot: 0, type: "uint8", byteOffset: 4 }, "e at offset 4");
  assertEqual(packed.fields["f"], { slot: 0, type: "uint8", byteOffset: 5 }, "f at offset 5");
  assertEqual(packed.fields["g"], { slot: 0, type: "uint16", byteOffset: 6 }, "g at offset 6");
  assertEqual(packed.fields["h"], { slot: 0, type: "uint32", byteOffset: 8 }, "h at offset 8");
  // Offset is now 12, uint64 (8 bytes) = 20 bytes total, still fits
  assertEqual(packed.fields["i"], { slot: 0, type: "uint64", byteOffset: 12 }, "i at offset 12");
  // Offset is now 20, uint128 (16 bytes) = 36 bytes > 32, so next slot (only field, no byteOffset)
  assertEqual(packed.fields["j"], { slot: 1, type: "uint128" }, "j at slot 1 (overflow)");
}

function testEnumSupport(): void {
  console.log("\n=== Enum Support Test ===");

  const { schema, warnings } = parseSoliditySource(ENUM_SOURCE, "YieldProvider.sol");

  assert(warnings.length === 0, "No warnings generated");
  assert("YieldProviderStorage" in schema.structs, "YieldProviderStorage found");

  const yps = schema.structs["YieldProviderStorage"];
  assert(yps.namespace === "test.storage.YieldProvider", "Namespace correct");

  // Enum YieldProviderVendor (2 values) should be uint8 (1 byte)
  // It should pack with the bools and address in slot 0:
  // uint8(1) + bool(1) + bool(1) + bool(1) + address(20) = 24 bytes in slot 0
  assertEqual(yps.fields["yieldProviderVendor"], { slot: 0, type: "uint8", byteOffset: 0 }, "enum converted to uint8");
  assertEqual(yps.fields["isStakingPaused"], { slot: 0, type: "bool", byteOffset: 1 }, "bool at offset 1");
  assertEqual(yps.fields["isOssificationInitiated"], { slot: 0, type: "bool", byteOffset: 2 }, "bool at offset 2");
  assertEqual(yps.fields["isOssified"], { slot: 0, type: "bool", byteOffset: 3 }, "bool at offset 3");
  assertEqual(yps.fields["primaryEntrypoint"], { slot: 0, type: "address", byteOffset: 4 }, "address at offset 4");

  // Slot 1: address (20 bytes) + uint96 (12 bytes) = 32 bytes exactly
  assertEqual(
    yps.fields["ossifiedEntrypoint"],
    { slot: 1, type: "address", byteOffset: 0 },
    "address at slot 1 offset 0",
  );
  assertEqual(
    yps.fields["yieldProviderIndex"],
    { slot: 1, type: "uint96", byteOffset: 20 },
    "uint96 at slot 1 offset 20",
  );

  // Slot 2: uint256 (32 bytes)
  assertEqual(yps.fields["userFunds"], { slot: 2, type: "uint256" }, "uint256 at slot 2");
}

function testCrossFileEnumSupport(): void {
  console.log("\n=== Cross-file Enum Support Test ===");

  // Use generateSchema to parse multiple files
  const { schema, warnings } = generateSchema([
    { source: ENUM_FILE_SOURCE, fileName: "Status.sol" },
    { source: STRUCT_USING_ENUM_SOURCE, fileName: "Task.sol" },
  ]);

  assert(warnings.length === 0, "No warnings generated");
  assert("TaskStorage" in schema.structs, "TaskStorage found");

  const task = schema.structs["TaskStorage"];
  assert(task.namespace === "test.storage.Task", "Namespace correct");

  // Enum Status (4 values) should be uint8 (1 byte)
  // It should pack with uint8 priority and address:
  // uint8(1) + uint8(1) + address(20) = 22 bytes in slot 0
  assertEqual(
    task.fields["status"],
    { slot: 0, type: "uint8", byteOffset: 0 },
    "enum from other file converted to uint8",
  );
  assertEqual(task.fields["priority"], { slot: 0, type: "uint8", byteOffset: 1 }, "uint8 at offset 1");
  assertEqual(task.fields["assignee"], { slot: 0, type: "address", byteOffset: 2 }, "address at offset 2");

  // Slot 1: uint256 (32 bytes)
  assertEqual(task.fields["deadline"], { slot: 1, type: "uint256" }, "uint256 at slot 1");
}

function testMultipleNamespaces(): void {
  console.log("\n=== Multiple Namespaces Test ===");

  const { schema, warnings } = parseSoliditySource(MULTIPLE_NAMESPACES_SOURCE, "MultipleNamespaces.sol");

  assert(warnings.length === 0, "No warnings generated");

  // Check all three namespaced structs
  assert("ConfigStorage" in schema.structs, "ConfigStorage found");
  assert("StateStorage" in schema.structs, "StateStorage found");
  assert("AccessStorage" in schema.structs, "AccessStorage found");

  const config = schema.structs["ConfigStorage"];
  const state = schema.structs["StateStorage"];
  const access = schema.structs["AccessStorage"];

  // Each should have unique namespace and baseSlot
  assert(config.namespace === "app.storage.Config", "Config namespace correct");
  assert(state.namespace === "app.storage.State", "State namespace correct");
  assert(access.namespace === "app.storage.Access", "Access namespace correct");

  assert(config.baseSlot !== undefined, "Config has baseSlot");
  assert(state.baseSlot !== undefined, "State has baseSlot");
  assert(access.baseSlot !== undefined, "Access has baseSlot");

  // All baseSlots should be different
  assert(config.baseSlot !== state.baseSlot, "Config and State have different baseSlots");
  assert(config.baseSlot !== access.baseSlot, "Config and Access have different baseSlots");
  assert(state.baseSlot !== access.baseSlot, "State and Access have different baseSlots");

  // Verify baseSlot calculation is deterministic
  const expectedConfigSlot = calculateErc7201BaseSlot("app.storage.Config");
  assertEqual(config.baseSlot, expectedConfigSlot, "Config baseSlot matches calculated value");
}

function testExplicitConstantValidation(): void {
  console.log("\n=== Explicit Constant Validation Test ===");

  const { schema } = parseSoliditySource(EXPLICIT_CONSTANT_SOURCE, "ExplicitConstant.sol");

  // Should find the struct
  assert("LineaRollupYieldExtensionStorage" in schema.structs, "Struct found");

  const struct = schema.structs["LineaRollupYieldExtensionStorage"];

  // The baseSlot should match the explicit constant
  assertEqual(
    struct.baseSlot,
    "0x594904a11ae10ad7613c91ac3c92c7c3bba397934d377ce6d3e0aaffbc17df00",
    "baseSlot matches explicit constant",
  );

  // Verify namespace
  assert(struct.namespace === "linea.storage.LineaRollupYieldExtensionStorage", "Namespace correct");

  // Check field
  assertEqual(struct.fields["_yieldManager"], { slot: 0, type: "address" }, "_yieldManager field correct");
}

function testMultipleFiles(): void {
  console.log("\n=== Multiple Files Test ===");

  const sources = [
    {
      source: `
        struct SharedConfig {
            uint256 value;
        }
      `,
      fileName: "SharedConfig.sol",
    },
    {
      source: `
        /// @custom:storage-location erc7201:app.storage.Main
        struct MainStorage {
            SharedConfig config;
            address owner;
        }
      `,
      fileName: "MainStorage.sol",
    },
  ];

  const { schema, warnings } = generateSchema(sources);

  assert(warnings.length === 0, "No warnings generated");

  // Both structs should be merged
  assert("SharedConfig" in schema.structs, "SharedConfig found from first file");
  assert("MainStorage" in schema.structs, "MainStorage found from second file");

  const main = schema.structs["MainStorage"];
  assert(main.namespace === "app.storage.Main", "MainStorage has namespace");
  assert(main.baseSlot !== undefined, "MainStorage has baseSlot");
}

function testErc7201BaseSlotCalculation(): void {
  console.log("\n=== ERC-7201 Base Slot Calculation Test ===");

  // Test known namespace from Linea contracts
  const lineaYieldSlot = calculateErc7201BaseSlot("linea.storage.LineaRollupYieldExtensionStorage");
  assertEqual(
    lineaYieldSlot,
    "0x594904a11ae10ad7613c91ac3c92c7c3bba397934d377ce6d3e0aaffbc17df00",
    "Linea yield extension slot matches known value",
  );

  // Test OpenZeppelin known namespace
  const ozInitSlot = calculateErc7201BaseSlot("openzeppelin.storage.Initializable");
  assertEqual(
    ozInitSlot,
    "0xf0c57e16840df040f15088dc2f81fe391c3923bec73e23a9662efc9c229c6a00",
    "OZ Initializable slot matches known value",
  );

  // Test that different namespaces produce different slots
  const slot1 = calculateErc7201BaseSlot("test.namespace.One");
  const slot2 = calculateErc7201BaseSlot("test.namespace.Two");
  assert(slot1 !== slot2, "Different namespaces produce different slots");

  // Test slot format
  assert(slot1.startsWith("0x"), "Slot starts with 0x");
  assert(slot1.length === 66, "Slot is 32 bytes (66 hex chars)");
  assert(slot1.endsWith("00"), "ERC-7201 slots end with 00 (masked)");
}

// ============================================================================
// Inline Nested Struct Layout Test (Bug Fix)
// ============================================================================

/**
 * Test that nested structs stored inline are computed with correct slot counts.
 * Bug fix: Previously, nested structs were treated as 1 slot (like mappings),
 * but Solidity stores structs inline with their actual field layout.
 */
const INLINE_NESTED_STRUCT_SOURCE = `
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

struct InnerStruct {
    uint256 fieldA;
    uint256 fieldB;
    uint256 fieldC;
}

/// @custom:storage-location erc7201:test.storage.Outer
struct OuterStorage {
    uint256 beforeValue;
    InnerStruct nested;
    uint256 afterValue;
}
`;

function testInlineNestedStructLayout(): void {
  console.log("\n=== Inline Nested Struct Layout Test (Bug Fix) ===");

  const { schema, warnings } = parseSoliditySource(INLINE_NESTED_STRUCT_SOURCE, "InlineNested.sol");

  assert(warnings.length === 0, "No warnings generated");

  // Check InnerStruct is found
  assert("InnerStruct" in schema.structs, "InnerStruct found");
  const innerStruct = schema.structs["InnerStruct"];
  assertEqual(innerStruct.fields["fieldA"], { slot: 0, type: "uint256" }, "InnerStruct fieldA at slot 0");
  assertEqual(innerStruct.fields["fieldB"], { slot: 1, type: "uint256" }, "InnerStruct fieldB at slot 1");
  assertEqual(innerStruct.fields["fieldC"], { slot: 2, type: "uint256" }, "InnerStruct fieldC at slot 2");

  // Check OuterStorage is found
  assert("OuterStorage" in schema.structs, "OuterStorage found");
  const outerStruct = schema.structs["OuterStorage"];

  // beforeValue should be at slot 0
  assertEqual(outerStruct.fields["beforeValue"], { slot: 0, type: "uint256" }, "beforeValue at slot 0");

  // nested (InnerStruct) should be at slot 1 and takes 3 slots
  assertEqual(outerStruct.fields["nested"], { slot: 1, type: "InnerStruct" }, "nested at slot 1");

  // afterValue should be at slot 4 (1 + 3 = 4), not slot 2!
  // This is the bug fix - nested struct consumes 3 slots, not 1
  assertEqual(
    outerStruct.fields["afterValue"],
    { slot: 4, type: "uint256" },
    "afterValue at slot 4 (after 3-slot nested struct)",
  );
}

// ============================================================================
// Main
// ============================================================================

async function main(): Promise<void> {
  console.log("============================================================");
  console.log("Schema Generator Tests");
  console.log("============================================================");

  testSimpleStruct();
  testNestedStructWithMappings();
  testComplexMappings();
  testPackedStorage();
  testBytes4AddressPacking();
  testExactSlotFill();
  testOverflowNextSlot();
  testMultipleSmallTypes();
  testEnumSupport();
  testCrossFileEnumSupport();
  testMultipleNamespaces();
  testExplicitConstantValidation();
  testMultipleFiles();
  testErc7201BaseSlotCalculation();
  testInlineNestedStructLayout();

  console.log("\n============================================================");
  console.log(`RESULTS: ${testsPassed} passed, ${testsFailed} failed`);
  console.log("============================================================");

  if (testsFailed > 0) {
    process.exit(1);
  }
}

// Run with Jest
describe("Schema Generator", () => {
  it("should pass all schema generator tests", async () => {
    await main();
    expect(testsFailed).toBe(0);
  });
});
