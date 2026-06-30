package linea.coordinator.clients.prover.riscv

import com.fasterxml.jackson.databind.JsonNode
import com.github.tomakehurst.wiremock.WireMockServer
import io.vertx.core.Vertx
import io.vertx.core.http.HttpVersion
import io.vertx.core.http.PoolOptions
import io.vertx.ext.web.client.WebClientOptions
import linea.clients.BlobWitness
import linea.clients.ChainConfig
import linea.clients.ExecutionInfo
import linea.clients.ExecutionPayload
import linea.clients.ForcedTransaction
import linea.clients.L2ExecutionProofRequestV1
import linea.clients.RollupAggregationProofRequestV1
import linea.clients.RollupProofRequestV1
import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.serialization.JsonSerialization
import linea.domain.BlockIntervalProofIndex
import linea.ethapi.ExecutionWitness
import linea.forcedtx.ForcedTransactionInclusionResult
import net.consensys.linea.httprest.client.VertxHttpRestClient
import java.math.BigInteger
import java.nio.file.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Shared test fixtures (constants, domain/DTO builders, fake transports and REST/WireMock helpers) reused across the
 * RISC-V prover-client test suites (`FileBased*ProverClientTest` / `Restful*ProverClientTest`).
 */
object RiscVProverClientTestFixtures {
  const val PROVER_VERSION = "4.0.0-riscv"
  const val CHAIN_ID = 59144L
  const val L2_EXECUTION_GUEST_PROGRAM_ID = "0x17d2e0660946012c80c5fe6bbecc2076a6f6f5aa58606efe66a14426d2ffe46f"
  const val ROLLUP_GUEST_PROGRAM_ID = "0x31139b3eaece046f5675fe237c36246e7bb2a5acc4cf4b358aef65c6d3771f4d"
  const val ROLLUP_AGGREGATION_GUEST_PROGRAM_ID = "0x8a5fdb137ddae03b9bad034500c0fcee76e1c61d70faca5f32bb7418d73392e1"

  val jsonMapper = JsonSerialization.proofResponseMapperV1

  val chainConfigDto = ChainConfigDto(
    l2MessageServiceAddress = "0x508ca82df566dcd1b0019d2dedf7e3d6f7ad6dde",
    coinbase = "0x0000000000000000000000000000000000000000",
    chainId = CHAIN_ID,
    forkName = "Amsterdam",
  )

  // --- file-based transport config ---

  fun fileBasedProverConfig(tempDir: Path): FileBasedProverConfig = FileBasedProverConfig(
    requestsDirectory = tempDir.resolve("requests"),
    responsesDirectory = tempDir.resolve("responses"),
    inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
    inprogressRequestWritingSuffix = "coordinator_writing_inprogress",
    pollingInterval = 100.milliseconds,
    pollingTimeout = 2.seconds,
  )

  // --- RESTful transport / WireMock helpers ---

  fun restClient(vertx: Vertx, wiremock: WireMockServer): VertxHttpRestClient {
    val webClientOptions = WebClientOptions()
      .setProtocolVersion(HttpVersion.HTTP_1_1)
      .setDefaultHost("localhost")
      .setDefaultPort(wiremock.port())
    return VertxHttpRestClient(webClientOptions, PoolOptions(), vertx)
  }

  /** Builds a `GET /v1/jobs/...` response body wrapping [proofResponse] under `proof_response`. */
  fun proverJobResponseBody(
    proofType: String,
    startBlock: Long,
    endBlock: Long,
    proofResponse: Any,
    status: String = "proved",
  ): String {
    val job = jsonMapper.createObjectNode().apply {
      put("proof_type", proofType)
      put("start_block", startBlock)
      put("end_block", endBlock)
      put("status", status)
      put("attempt", 1)
      set<JsonNode>("proof_response", jsonMapper.valueToTree(proofResponse))
    }
    return jsonMapper.writeValueAsString(job)
  }

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
    parentFtxRollingHash: ByteArray = ByteArray(32) { 1 },
    parentFtxNumber: ULong = 100UL,
  ): L2ExecutionProofRequestV1 = L2ExecutionProofRequestV1(
    executions = executions,
    chainConfig = ChainConfig(
      l2MessageServiceContract = ByteArray(20) { 1 },
      coinbase = ByteArray(20) { 2 },
      chainId = 1000UL,
    ),
    parentFtxRollingHash = parentFtxRollingHash,
    parentFtxNumber = parentFtxNumber,
  )

  fun blobWitness(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
  ): BlobWitness = BlobWitness(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    blobHash = ByteArray(32) { 0x3a },
    blobKzgProof = ByteArray(48) { 0x3b },
    blockRlps = listOf(byteArrayOf(0x3c)),
  )

  fun rollupProofRequestV1(
    blobs: List<BlobWitness> = emptyList(),
    parentShnarf: ByteArray = ByteArray(32) { 0x19 },
    endShnarf: ByteArray = ByteArray(32) { 0x20 },
    l2Executions: List<BlockIntervalProofIndex> = listOf(blockIntervalProofIndex(1000501UL, 1000520UL)),
  ): RollupProofRequestV1 = RollupProofRequestV1(
    blobs = blobs,
    parentShnarf = parentShnarf,
    endShnarf = endShnarf,
    l2Executions = l2Executions,
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
      l2L1MessagesHash = "0x01",
      parentL1L2BridgeRollingHash = "0x02",
      parentL1L2BridgeRollingHashMessageNumber = 3,
      endL1L2BridgeRollingHash = "0x04",
      endL1L2BridgeRollingHashMessageNumber = 5,
      dynamicChainConfigHash = "0xc0ffee",
      parentFtxRollingHash = "0x06",
      parentProcessedFtxNumber = 7,
      endFtxRollingHash = "0x07",
      endProcessedFtxNumber = 8,
      filteredAddressesHash = "0x09",
      txFromsHash = "0x0c",
    )

  fun l2ExecutionProofResponseDto(
    startBlockNumber: Long,
    endBlockNumber: Long,
  ): L2ExecutionProofResponseDto = L2ExecutionProofResponseDto(
    proverVersion = PROVER_VERSION,
    startBlockNumber = startBlockNumber,
    proof = "0xabcd",
    publicInputs = l2ExecutionProofPublicInputsDto(endBlockNumber),
    l2L1Messages = listOf("0xaa"),
    txFroms = listOf("0xbb"),
    filteredAddresses = emptyList(),
  )

  fun rollupProofPublicInputsDto(
    endBlockNumber: Long,
  ): RollupProofPublicInputsDto = RollupProofPublicInputsDto(
    endBlockNumber = endBlockNumber,
    endBlockTimestamp = 1763000457,
    l2L1BridgeTransactionTree = "0x10",
    parentL1L2BridgeRollingHash = "0x11",
    parentL1L2BridgeRollingHashMessageNumber = 12,
    endL1L2BridgeRollingHash = "0x13",
    endL1L2BridgeRollingHashMessageNumber = 14,
    dynamicChainConfigHash = "0xc0ffee",
    parentFtxRollingHash = "0x15",
    parentProcessedFtxNumber = 16,
    endFtxRollingHash = "0x16",
    endProcessedFtxNumber = 17,
    filteredAddressesHash = "0x18",
    parentShnarf = "0x19",
    endShnarf = "0x1a",
  )

  fun rollupProofResponseDto(
    startBlockNumber: Long,
    endBlockNumber: Long,
  ): RollupProofResponseDto = RollupProofResponseDto(
    proverVersion = PROVER_VERSION,
    startBlockNumber = startBlockNumber,
    proof = "0xabcd",
    publicInputs = rollupProofPublicInputsDto(endBlockNumber),
    l2L1Roots = listOf("0xaa"),
    filteredAddresses = emptyList(),
  )

  fun rollupAggregationProofResponseDto(
    startBlockNumber: Long,
    endBlockNumber: Long,
  ): RollupAggregationProofResponseDto = RollupAggregationProofResponseDto(
    proverVersion = PROVER_VERSION,
    startBlockNumber = startBlockNumber,
    proof = "0xabcd",
    publicInputs = rollupProofPublicInputsDto(
      endBlockNumber,
    ).copy(endBlockNumber = 1000567, endBlockTimestamp = 1763002301),
    l2L1Roots = listOf("0xaa"),
    filteredAddresses = listOf("0xbb"),
    l2MessagingBlocksOffsets = listOf(1, 20, 100),
  )
}
