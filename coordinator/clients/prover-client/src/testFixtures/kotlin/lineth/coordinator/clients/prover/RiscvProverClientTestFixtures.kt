package lineth.coordinator.clients.prover

import linea.clients.ChunkWitness
import linea.clients.ConflationWitness
import linea.clients.ExecutionInfo
import linea.clients.ForcedTransaction
import linea.clients.L2ExecutionProofRequestV1
import linea.clients.RollupAggregationProofRequestV1
import linea.clients.RollupProofRequestV1
import linea.domain.BlockIntervalProofIndex
import linea.domain.ExecutionPayload
import linea.ethapi.ExecutionWitness
import linea.ethapi.SystemContracts
import linea.forcedtx.ForcedTransactionInclusionResult
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import lineth.coordinator.config.v2.FileBasedProverConfig
import java.math.BigInteger
import java.nio.file.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Shared test fixtures (constants, domain/DTO builders, fake transports) reused across the RISC-V prover-client
 * test suites (`FileBased*ProverClientTest`).
 */
object RiscvProverClientTestFixtures {
  const val PROVER_VERSION = "4.0.0-riscv"
  const val CHAIN_ID = 59144L
  const val FORK_NAME = "Amsterdam"
  const val PROVING_SYSTEM_VERSION = "0x6c775558affd1208d76f9adff8d011d1850947966f1df5211e96a6d369771581"
  const val L2_EXECUTION_PROGRAM_ID = "0x17d2e0660946012c80c5fe6bbecc2076a6f6f5aa58606efe66a14426d2ffe46f"
  const val ROLLUP_PROGRAM_ID = "0x31139b3eaece046f5675fe237c36246e7bb2a5acc4cf4b358aef65c6d3771f4d"
  const val ROLLUP_AGGREGATION_PROGRAM_ID = "0x8a5fdb137ddae03b9bad034500c0fcee76e1c61d70faca5f32bb7418d73392e1"
  const val L2_MESSAGE_SERVICE_ADDRESS = "0x508ca82df566dcd1b0019d2dedf7e3d6f7ad6dde"
  const val COINBASE = "0x0000000000000000000000000000000000000000"
  val SYSTEM_CONTRACTS = SystemContracts(
    beaconRoots = "0x000f3df6d732807ef1319fb7b8bb8522d0beac02",
    historyStorage = "0x0000f90827f1c53a10cb7a02335b175320002935",
    deposit = "0x94d4a7449b968b839dcbc64a61f195ca300986ae",
    withdrawalRequest = "0x66355689a9f067eeb9dc9d899e4192676988279c",
    consolidationRequest = "0xf0e003f0de2d583ae28fa8cbf66aa096cdace3ff",
    builderDepositRequest = "0x0000bff46984e3725691fa540a8c7589300d8282",
    builderExitRequest = "0x000064d678505ad48f8ccb093bc65613800e8282",
  )

  val jsonMapper = JsonSerialization.proofResponseMapperV1

  // --- file-based transport config ---

  fun fileBasedProverConfig(tempDir: Path): FileBasedProverConfig = FileBasedProverConfig(
    requestsDirectory = tempDir.resolve("requests"),
    responsesDirectory = tempDir.resolve("responses"),
    inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
    inprogressRequestWritingSuffix = "coordinator_writing_inprogress",
    pollingInterval = 100.milliseconds,
    pollingTimeout = 2.seconds,
  )

  // --- domain request builders ---

  fun blockIntervalProofIndex(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
    hash: ByteArray = ByteArray(32) { 0x1a },
    startBlockTimestamp: Instant = Instant.fromEpochSeconds(1763000000),
  ): BlockIntervalProofIndex = BlockIntervalProofIndex(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    hash = hash,
    startBlockTimestamp = startBlockTimestamp,
  )

  fun executionPayload(
    blockNumber: ULong,
  ): ExecutionPayload = ExecutionPayload(
    parentHash = ByteArray(32) { 0x1a },
    feeRecipient = ByteArray(20) { 0x1b },
    stateRoot = ByteArray(32) { 0x1c },
    receiptsRoot = ByteArray(32) { 0x1d },
    logsBloom = ByteArray(256) { 0x1e },
    prevRandao = ByteArray(32) { 0x1f },
    blockNumber = blockNumber,
    gasLimit = 100000UL,
    gasUsed = 90000UL,
    timestamp = 1000UL,
    extraData = ByteArray(32) { 0x2a },
    baseFeePerGas = BigInteger.valueOf(10000000L),
    blockHash = ByteArray(32) { 0x2b },
    transactions = emptyList(),
    withdrawals = emptyList(),
    blobGasUsed = 0UL,
    excessBlobGas = 0UL,
    blockAccessList = ByteArray(0),
  )

  fun executionInfo(
    blockNumber: ULong,
    executionWitness: ExecutionWitness = ExecutionWitness(emptyList(), emptyList(), emptyList()),
    executionRequests: List<ByteArray> = emptyList(),
  ): ExecutionInfo = ExecutionInfo(
    blockNumber = blockNumber,
    executionPayload = executionPayload(blockNumber),
    executionWitness = executionWitness,
    executionRequests = executionRequests,
    forcedTransactions = listOf(
      ForcedTransaction(
        ftxNumber = 101UL,
        deadlineBlockNumber = blockNumber + 100UL,
        signedTxRlp = byteArrayOf(0x2a),
        acceptance = ForcedTransactionInclusionResult.FilteredAddressTo,
      ),
      ForcedTransaction(
        ftxNumber = 102UL,
        deadlineBlockNumber = blockNumber + 100UL,
        signedTxRlp = byteArrayOf(0x2b),
        acceptance = ForcedTransactionInclusionResult.Included,
      ),
    ),
  )

  fun l2ExecutionProofRequestV1(
    executions: List<ExecutionInfo> = listOf(executionInfo(1000501UL), executionInfo(1000502UL)),
    coinbase: String = COINBASE,
    parentFtxRollingHash: ByteArray = ByteArray(32) { 1 },
    parentFtxNumber: ULong = 100UL,
  ): L2ExecutionProofRequestV1 = L2ExecutionProofRequestV1(
    executions = executions,
    chainId = CHAIN_ID.toULong(),
    coinbase = coinbase,
    systemContracts = SYSTEM_CONTRACTS,
    parentFtxRollingHash = parentFtxRollingHash,
    parentFtxNumber = parentFtxNumber,
  )

  fun conflationWitness(
    blockCount: Int = 1,
  ): ConflationWitness = ConflationWitness(
    blockRlps = List(blockCount) { byteArrayOf(0x3c) },
  )

  fun blobChunkWitness(fill: Byte = 0x20): ChunkWitness = ChunkWitness(
    chunkHash = ByteArray(32) { fill },
    isCalldata = false,
    blobBytes = ByteArray(ChunkWitness.BLOB_BYTES_LENGTH) { fill },
    calldataBytes = ByteArray(0),
  )

  fun rollupProofRequestV1(
    conflations: List<ConflationWitness> = emptyList(),
    parentDataRollingHash: ByteArray = ByteArray(32) { 0x19 },
    chunks: List<ChunkWitness> = listOf(blobChunkWitness()),
    parentDataTailTakeBytes: Int = 0,
    l2Executions: List<BlockIntervalProofIndex> = listOf(blockIntervalProofIndex(1000501UL, 1000520UL)),
  ): RollupProofRequestV1 = RollupProofRequestV1(
    conflations = conflations,
    l2Executions = l2Executions,
    chunks = chunks,
    parentDataRollingHash = parentDataRollingHash,
    parentDataTailTakeBytes = parentDataTailTakeBytes,
  )

  fun rollupAggregationProofRequestV1(
    rollupProofs: List<BlockIntervalProofIndex> = listOf(blockIntervalProofIndex(1000501UL, 1000567UL)),
  ): RollupAggregationProofRequestV1 = RollupAggregationProofRequestV1(rollupProofs = rollupProofs)

  // --- response DTO builders ---

  fun l2ExecutionProofPublicInputsDto(endBlockNumber: Long): L2ExecutionProofPublicInputsDto =
    L2ExecutionProofPublicInputsDto(
      parentBlockHash = "0x0a",
      endBlockHash = "0x0b",
      endBlockNumber = endBlockNumber,
      endBlockTimestamp = 1763000123,
      l2L1Messages = listOf("0xaa"),
      parentL1L2BridgeRollingHash = "0x02",
      parentL1L2BridgeRollingHashMessageNumber = 3,
      endL1L2BridgeRollingHash = "0x04",
      endL1L2BridgeRollingHashMessageNumber = 5,
      dynamicChainConfigHash = "0xc0ffee",
      parentFtxRollingHash = "0x06",
      parentFtxNumber = 7,
      endFtxRollingHash = "0x07",
      endProcessedFtxNumber = 8,
      filteredAddresses = emptyList(),
      txFromsHash = "0x0c",
      blockCount = 1,
      l2MessagingBlocksOffsets = emptyList(),
    )

  fun l2ExecutionProofResponseDto(
    startBlockNumber: Long,
    endBlockNumber: Long,
  ): L2ExecutionProofResponseDto = L2ExecutionProofResponseDto(
    proverVersion = PROVER_VERSION,
    startBlockNumber = startBlockNumber,
    proof = "0xabcd",
    publicInputs = l2ExecutionProofPublicInputsDto(endBlockNumber),
    programVk = L2_EXECUTION_PROGRAM_ID,
  )

  fun rollupProofPublicInputsDto(
    endBlockNumber: Long,
  ): RollupProofPublicInputsDto = RollupProofPublicInputsDto(
    endBlockNumber = endBlockNumber,
    endBlockTimestamp = 1763000457,
    parentL1L2BridgeRollingHash = "0x11",
    parentL1L2BridgeRollingHashMessageNumber = 12,
    endL1L2BridgeRollingHash = "0x13",
    endL1L2BridgeRollingHashMessageNumber = 14,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x15",
    parentFtxNumber = 16,
    endFtxRollingHash = "0x16",
    endProcessedFtxNumber = 17,
    parentDataRollingHash = "0x19",
    endDataRollingHash = "0x1a",
    parentBlockHash = "0x1b",
    endBlockHash = "0x1c",
    parentDataTailTakeBytes = 0,
    finalDataTailDiscardBytes = 1000,
    l2L1Messages = listOf("0xaa"),
    filteredAddresses = emptyList(),
    programVks = listOf(L2_EXECUTION_PROGRAM_ID),
    blockCount = 20,
    l2MessagingBlocksOffsets = emptyList(),
  )

  fun rollupProofResponseDto(
    startBlockNumber: Long,
    endBlockNumber: Long,
  ): RollupProofResponseDto = RollupProofResponseDto(
    proverVersion = PROVER_VERSION,
    programVk = ROLLUP_PROGRAM_ID,
    proof = "0xabcd",
    startBlockNumber = startBlockNumber,
    publicInputs = rollupProofPublicInputsDto(endBlockNumber),
  )

  fun finalizationPublicInputsDto(
    endBlockNumber: Long,
  ): FinalizationPublicInputsDto = FinalizationPublicInputsDto(
    endBlockNumber = endBlockNumber,
    endBlockTimestamp = 1763002301,
    parentL1L2BridgeRollingHash = "0x11",
    parentL1L2BridgeRollingHashMessageNumber = 12,
    endL1L2BridgeRollingHash = "0x13",
    endL1L2BridgeRollingHashMessageNumber = 14,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x15",
    parentFtxNumber = 16,
    endFtxRollingHash = "0x16",
    endProcessedFtxNumber = 17,
    parentDataRollingHash = "0x19",
    endDataRollingHash = "0x1a",
    parentBlockHash = "0x1b",
    endBlockHash = "0x1c",
    parentDataTailTakeBytes = 0,
    finalDataTailDiscardBytes = 1000,
    l2L1Roots = listOf("0xaa"),
    l2L1TreeDepth = 5,
    filteredAddresses = listOf("0xbb"),
    programIds = listOf(L2_EXECUTION_PROGRAM_ID, ROLLUP_PROGRAM_ID),
    l2MessagingBlocksOffsets = listOf(1, 20, 100),
  )

  fun rollupAggregationProofResponseDto(
    startBlockNumber: Long,
    endBlockNumber: Long,
  ): RollupAggregationProofResponseDto = RollupAggregationProofResponseDto(
    proverVersion = PROVER_VERSION,
    proof = "0xabcd",
    startBlockNumber = startBlockNumber,
    publicInputs = finalizationPublicInputsDto(endBlockNumber),
  )
}
