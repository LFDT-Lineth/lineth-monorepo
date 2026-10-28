package linea.clients

import linea.kotlin.trimToSecondPrecision
import kotlin.random.Random
import kotlin.time.Clock
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/** Builds a [RollupAggregationProofResponseV1] with random hashes; the RISC-V counterpart of `createProofToFinalize`. */
fun createRollupAggregationProofResponseV1(
  startBlockNumber: ULong,
  endBlockNumber: ULong,
  startBlockTime: Instant = Clock.System.now().trimToSecondPrecision(),
  endBlockTimestamp: Instant = startBlockTime.plus((2L * (endBlockNumber - startBlockNumber).toLong()).seconds),
  l2L1Roots: List<ByteArray> = listOf(Random.nextBytes(32)),
  filteredAddresses: List<ByteArray> = emptyList(),
  programIds: List<ByteArray> = listOf(Random.nextBytes(32)),
  l2MessagingBlocksOffsets: List<ULong> = emptyList(),
): RollupAggregationProofResponseV1 {
  return RollupAggregationProofResponseV1(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    proof = Random.nextBytes(64),
    publicInputs = FinalizationPublicInputs(
      endBlockNumber = endBlockNumber,
      endBlockTimestamp = endBlockTimestamp,
      endL1L2BridgeMessageRollingHash = Random.nextBytes(32),
      endL1L2BridgeMessageNumber = 2UL,
      dynamicChainConfigHash = Random.nextBytes(32),
      parentFtxRollingHash = Random.nextBytes(32),
      parentFtxNumber = 3UL,
      endFtxRollingHash = Random.nextBytes(32),
      endFtxNumber = 4UL,
      parentDataRollingHash = Random.nextBytes(32),
      endDataRollingHash = Random.nextBytes(32),
      parentBlockHash = Random.nextBytes(32),
      endBlockHash = Random.nextBytes(32),
      parentDataTailTakeBytes = 0,
      finalDataTailDiscardBytes = 1000,
      l2L1Roots = l2L1Roots,
      l2L1TreeDepth = 5UL,
      filteredAddresses = filteredAddresses,
      programIds = programIds,
      l2MessagingBlocksOffsets = l2MessagingBlocksOffsets,
    ),
  )
}
