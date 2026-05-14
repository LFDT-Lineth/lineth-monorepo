# Maru Utils

This module contains utility tools for Maru blockchain operations.

## Tools

### 1. PrivateKeyGenerator

A utility tool to generate a prefixed private key and corresponding node ID, following the same pattern as used in MaruFactory for validator private keys.

#### Purpose
Generates cryptographic keys for Maru validators and P2P networking, displaying both the prefixed private key and the corresponding LibP2P node ID.

#### Usage

**Generate keys:**
```bash
./gradlew :jvm-libs:utils:keytool --args='generateKeys --numberOfKeys=5'
```

**Get Key Info:**
```bash
./gradlew :jvm-libs:utils:keytool --args='prefixedKeyInfo --privKey=0x08021220abcd1234...'
```

```bash
./gradlew :jvm-libs:utils:keytool --args='secp256k1Info --privKey=0xabcd1234...'
```

**Help:**
```bash
./gradlew :jvm-libs:utils:keytool --args='--help'
./gradlew :jvm-libs:utils:keytool --args='secp256k1Info --help'
```

### 2. DifficultyCalculator

A utility tool to compute the expected difficulty at a given time for Clique blocks.

#### Purpose
Calculates the expected block number and difficulty for Clique consensus blocks at a specific future timestamp.

#### Clique Block Properties
- Difficulty = blockNumber × 2 + 1
- Block time = 2 seconds
- Expected timestamp[i] = timestamp[i-1] + 2

#### Usage

**Command Line:**
```bash
./gradlew -q :jvm-libs:utils:runDifficultyCalculator --args="<currentBlockNumber> <currentTimestamp> <desiredSwitchTime>"
```

**Parameters:**
- `currentBlockNumber`: The current block number (Long)
- `currentTimestamp`: The timestamp of the current block (Long, Unix timestamp)
- `desiredSwitchTime`: The target timestamp for which to compute the difficulty (Long, Unix timestamp)

**Example:**
```bash
./gradlew -q :jvm-libs:utils:runDifficultyCalculator --args="1000 1692000000 1692001000"
```

**Output:**
```
=== Difficulty Calculator Debug ===
Current block: 1000
Current timestamp: 1692000000
Desired switch time: 1692001000
Time difference: 1000 seconds
Blocks to add: 500
Expected block number: 1500
Expected difficulty: 3001

Expected result:
Block: 1500, Difficulty: 3001, Timestamp: 1692001000
```



**Example Output:**
```
Generated private key (prefixed): 0x08021220abcd1234...
Ethereum address: 0xabc
Corresponding node ID: 16Uiu2HAm...
```

#### Security Note
⚠️ **Warning**: The generated private keys are logged as plaintext for development and testing purposes. In production environments, ensure proper key management and never expose private keys in logs.

### Prerequisites
- Java 25+
