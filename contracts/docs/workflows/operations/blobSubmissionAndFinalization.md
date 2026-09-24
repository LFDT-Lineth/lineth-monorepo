# Blob Submission & Finalization

This document outlines the core data-availability and finalization flows involved in LinethRollup's lifecycle under the blob-spanning `dataRollingHash` model (`CONTRACT_VERSION()` returns `"9.0"`; see [lineth-rollup.md](../../deployment/l1/lineth-rollup.md) for the `"10.0"` cutover reinitializer).

---

## Data Submission

This flow is used by the **Data Submission Operator** to submit DA chunks (EIP-4844 blobs or calldata) to the LinethRollup system. Unlike the legacy shnarf model, a chunk carries no L2 block hash or conflation boundary — it is pure DA, folded into a running `dataRollingHash` accumulator. A single logical stream can span any number of submissions, and a single finalization can span a partial chunk (see `startOffset`/`endOffset` below), hence "blob-spanning".

### Steps — EIP-4844 blobs

1. **Data Submission Operator** calls `submitBlobs(bytes32 parentDataRollingHash, bytes32 storedDataRollingHash)` on the `LinethRollup` contract via a blob-carrying transaction (1 to N blobs, N = network maximum).
2. For each blob index `i` carried by the transaction:
   - The contract reads `blobhash(i)` as the chunk hash (reverts with `BlobSubmissionDataIsMissing` if `blobhash(0)` is zero).
   - Folds `dataRollingHash = keccak256(dataRollingHash || blobhash(i))`, starting from `parentDataRollingHash`.
3. The final folded value must equal `storedDataRollingHash` (`DataRollingHashMismatch` otherwise) and is anchored.
4. `parentDataRollingHash` must already be anchored, or be `EMPTY_HASH` (`ParentDataRollingHashNotAnchored` otherwise); `storedDataRollingHash` must not already be anchored (`DataRollingHashAlreadyAnchored` otherwise).
5. `DataSubmittedV4(parentDataRollingHash, dataRollingHash)` is emitted.

### Steps — calldata

Calldata DA (`CalldataOnlyDaRollup`, Validium, test harnesses) uses `submitDataAsCalldata(bytes compressedData, bytes32 parentDataRollingHash, bytes32 storedDataRollingHash)`. The chunk hash is `keccak256(compressedData)`, folded once with the same `dataRollingHash = keccak256(parentDataRollingHash || chunkHash)` accumulator and the same anchoring rules as above.

**Note:** L1 no longer runs the EIP-4844 point-evaluation precompile for DA acceptance; blob DA still requires a real blob-carrying transaction so `blobhash(i)` is non-zero.

**Note:** only the final `dataRollingHash` of each submission is anchored on-chain (`_dataRollingHashExists`); intermediate per-chunk folds within a multi-blob submission are not separately persisted. `EMPTY_HASH` is permanently treated as an anchored parent, covering both a genuine genesis fresh-start and a network still awaiting its one-time legacy-shnarf migration (see below).

---

## Finalization Submission

This flow finalizes 1 or more DA submissions by verifying correct L2 execution proven via zero-knowledge proofs, and advances the live DA stream position.

### Steps

1. **Finalization Submission Operator** calls `finalizeBlocks(bytes aggregatedProof, uint256 proofType, FinalizationDataV5 finalizationData)`.
2. `LinethRollup` contract:
   - Validates guest-program `verifierKeys` against the on-chain allowlist.
   - Validates the messaging rolling hash feedback loop (L1 → L2 → L1) preventing manipulation or censorship, and that no forced transaction due within the finalized range is missing.
   - Execution-rooting continuity:
     - If `blockHashes[lastFinalized]` is empty → **legacy migration path**: requires `parentBlockHash == EMPTY_HASH` and a matching `parentStateRootHash` against the legacy `stateRootHashes[lastFinalized]`.
     - Else → **block-hash path**: requires the caller-declared `parentBlockHash` to match the on-chain `blockHashes[lastFinalized]`.
   - Requires `finalBlockHash != EMPTY_HASH`.
   - **One-time legacy-shnarf migration** (only if `finalizationData.shnarfData` has any non-zero field): reconstructs the legacy 5-input shnarf (`keccak256(parentShnarf || snarkHash || finalStateRootHash || keccak256(snarkHash || blobHash) || dataEvaluationClaim)`) and requires it to equal the still-live `currentFinalizedShnarf`, requires the live DA position to still be at its zero default (`LegacyShnarfAlreadyMigrated` otherwise), then wipes `currentFinalizedShnarf` to `EMPTY_HASH` and emits `LegacyShnarfMigrated`. This is a one-way integrity check on the last legacy blob; it does **not** chain the new `dataRollingHash` stream from the legacy shnarf — the new stream still starts fresh from `EMPTY_HASH`.
   - DA continuity: requires `parentDataRollingHash == currentDataRollingHash` and `startOffset == currentDataAvailabilityOffset` (`DataRollingHashNotContinuous` / `StartOffsetNotContinuous` otherwise).
   - DA anchoring: requires `endDataRollingHash` to have been anchored by a prior submission (`FinalDataRollingHashNotAnchored` otherwise).
   - Stores **Merkle roots** of L2 → L1 messages for proof-based claiming, and emits events for L2 blocks containing `MessageSent` events.
3. Computes the **public input** as a single `keccak256(...) % MODULO_R` over one contiguous, assembly-packed memory region (in order): `parentBlockHash`, `finalBlockHash`, `finalTimestamp`, `endBlockNumber`, `lastFinalizedL1RollingHash`, `l1RollingHash`, `lastFinalizedL1RollingHashMessageNumber`, `l1RollingHashMessageNumber`, `lastFinalizedForcedTransactionRollingHash`, `finalForcedTransactionRollingHash`, `lastFinalizedForcedTransactionNumber`, `finalForcedTransactionNumber`, `l2MerkleTreesDepth`, `parentDataRollingHash`, `endDataRollingHash`, `startOffset`, `endOffset`, then the hashed dynamic arrays: `keccak256(l2MerkleRoots)`, the verifier chain configuration, `keccak256(filteredAddresses)`, `keccak256(verifierKeys)`. `shnarfData` (legacy migration only) is excluded from the public input.
4. Calls the Plonk-based **Verifier** to validate the provided zk-proof against that public input.
5. Upon success, updates finalized state:
   - Anchors `blockHashes[endBlockNumber] = finalBlockHash`.
   - Updates `currentL2BlockNumber`, `currentDataRollingHash`, `currentDataAvailabilityOffset`, and `currentFinalizedState`.
   - Emits `FinalizedStateUpdated` and `DataFinalizedV4`.

**Note:** the public-input byte layout above (a flat, assembly-packed `keccak256` over the fields in the order listed, with the legacy shnarf pair replaced by `parentBlockHash`/`finalBlockHash` and the DA stream-position fields appended just before the hashed dynamic arrays) differs from the layout used prior to blob spanning. Any change to this layout must be mirrored bit-for-bit by the off-chain prover/circuit before deployment.

---

### Verifier Contract

The verifier contract is an advanced zero-knowledge proof verifier specifically tailored for the PLONK protocol on Ethereum mainnet. It verifies zk-SNARK proofs generated using [gnark](https://github.com/Consensys/gnark), ensuring that a given proof corresponds to a valid computation without revealing inputs. The contract is written almost entirely in inline Yul assembly for gas efficiency and precision, and it uses elliptic curve operations, pairings, and the Fiat-Shamir heuristic to validate a serialized proof against public inputs. This is critical infrastructure for trustless, privacy-preserving applications such as rollups, where it ensures the integrity of off-chain computations before accepting their results on-chain.

---

<img src="../diagrams/blobSubmissionAndFinalization.png">
