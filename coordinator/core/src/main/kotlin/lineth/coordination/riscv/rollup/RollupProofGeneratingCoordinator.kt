package lineth.coordination.riscv.rollup

import io.vertx.core.Vertx
import linea.clients.ConflationWitness
import linea.clients.RollupProofRequestV1
import linea.clients.RollupProverClientV1
import linea.domain.BlobData
import linea.domain.Block
import linea.domain.BlockIntervalProofIndex
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.Constants
import linea.domain.DataRollingHashCalculator
import linea.domain.StreamPosition
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import lineth.conflation.ConflationHandler
import lineth.coordination.riscv.conflation.ConflationSegmentBuilder
import lineth.encoding.BlockEncoder
import lineth.persistence.BatchesRepository
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.ByteArrayOutputStream
import kotlin.time.Duration
import kotlin.time.Instant

fun interface StreamPositionProvider {
  fun getStreamPosition(): SafeFuture<StreamPosition>
}

class RollupProofGeneratingCoordinator(
  private val chainId: ULong,
  private val conflationsPerRollupProof: Int,
  private val rollupProverClient: RollupProverClientV1,
  private val batchesRepository: BatchesRepository,
  private val streamPositionProvider: StreamPositionProvider,
  private val rollupProofPoller: RollupProofPoller,
  private val blockEncoder: BlockEncoder,
  private val chunkHasher: (ByteArray) -> ByteArray,
  private val proofCheckingInterval: Duration,
  private val vertx: Vertx,
  private val log: Logger = LogManager.getLogger(RollupProofGeneratingCoordinator::class.java),
  @Suppress("UNUSED_PARAMETER") metricsFacade: MetricsFacade,
) : ConflationHandler,
  VertxPeriodicPollingService(
    vertx = vertx,
    name = "RollupProofGeneratingCoordinator",
    pollingIntervalMs = proofCheckingInterval.inWholeMilliseconds,
    log = log,
    timerSchedule = TimerSchedule.FIXED_DELAY,
  ) {

  private data class SealedChunk(
    val blobBytes: ByteArray,
    val chunkHash: ByteArray,
    val parentDataRollingHash: ByteArray,
    val endOffset: Int,
  )

  private data class PendingConflation(
    val conflationResult: ConflationCalculationResult,
    val blocks: List<Block>,
    val segmentSize: Int,
  )

  private val streamBuffer = ByteArrayOutputStream()
  private lateinit var parentDataRollingHash: ByteArray
  private var lastHandledBlockNumber: ULong = 0u
  private var proofWindowBytesSealed: Int = 0
  private val pendingConflations = ArrayDeque<PendingConflation>()
  private val sealedFullChunks = mutableListOf<SealedChunk>()

  private companion object {
    const val BLOB_BYTES_LENGTH = Constants.Eip4844BlobSize
  }

  override fun start(): SafeFuture<Unit> =
    streamPositionProvider.getStreamPosition()
      .thenCompose { position ->
        parentDataRollingHash = position.dataRollingHash
        lastHandledBlockNumber = position.lastConflationEndBlock
        super.start()
      }.toSafeFuture()

  @Synchronized
  override fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*> {
    require(conflation.conflationResult.startBlockNumber == lastHandledBlockNumber + 1u) {
      "Conflation out of order: expected startBlockNumber=${lastHandledBlockNumber + 1u}, " +
        "got ${conflation.conflationResult.startBlockNumber}"
    }

    val segment = ConflationSegmentBuilder.buildSegment(conflation.blocks, chainId)
    streamBuffer.write(segment)
    pendingConflations += PendingConflation(conflation.conflationResult, conflation.blocks, segment.size)
    lastHandledBlockNumber = conflation.conflationResult.endBlockNumber

    while (streamBuffer.size() >= BLOB_BYTES_LENGTH) {
      sealNextFullChunk()
    }
    return SafeFuture.completedFuture(Unit)
  }

  private fun sealNextFullChunk() {
    val raw = streamBuffer.toByteArray()
    val blobBytes = raw.copyOfRange(0, BLOB_BYTES_LENGTH)
    streamBuffer.reset()
    streamBuffer.write(raw, BLOB_BYTES_LENGTH, raw.size - BLOB_BYTES_LENGTH)

    val chunkHash = chunkHasher(blobBytes)
    val nextDrh = DataRollingHashCalculator.fold(parentDataRollingHash, chunkHash)

    proofWindowBytesSealed += BLOB_BYTES_LENGTH
    sealedFullChunks += SealedChunk(blobBytes, chunkHash, parentDataRollingHash, BLOB_BYTES_LENGTH)
    parentDataRollingHash = nextDrh
  }

  private fun trySubmitRollupProof(): SafeFuture<Unit> {
    val window = pendingConflations.take(conflationsPerRollupProof)
    if (window.size < conflationsPerRollupProof) return SafeFuture.completedFuture(Unit)

    return batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(
      window.first().conflationResult.startBlockNumber.toLong(),
    ).thenCompose { highestEndBlock ->
      val allProven =
        highestEndBlock != null &&
          highestEndBlock >= window.last().conflationResult.endBlockNumber.toLong()
      if (!allProven) return@thenCompose SafeFuture.completedFuture(Unit)

      val partialChunk = flushPartialChunk() ?: return@thenCompose SafeFuture.completedFuture(Unit)
      val allChunks = sealedFullChunks + partialChunk

      rollupProverClient.createProofRequest(buildRequest(allChunks, window))
        .thenCompose { proofIndex ->
          rollupProofPoller.addProofInProgress(
            proofIndex = proofIndex,
            blobsData =
            allChunks.map {
              BlobData(chunkHash = it.chunkHash, blobBytes = it.blobBytes, conflationsCount = 0u)
            },
            parentDataRollingHash = allChunks.first().parentDataRollingHash,
            dataRollingHash =
            allChunks.last().let {
              DataRollingHashCalculator.fold(it.parentDataRollingHash, it.chunkHash)
            },
            endOffset = partialChunk.endOffset,
            startBlockTimestamp = Instant.fromEpochSeconds(window.first().blocks.first().timestamp.toLong()),
            endBlockTimestamp = Instant.fromEpochSeconds(window.last().blocks.last().timestamp.toLong()),
            totalConflationsCount = window.size,
          )

          sealedFullChunks.clear()
          repeat(conflationsPerRollupProof) { pendingConflations.removeFirst() }
          proofWindowBytesSealed = 0

          trySubmitRollupProof()
        }.toSafeFuture()
    }.toSafeFuture()
  }

  private fun flushPartialChunk(): SealedChunk? {
    if (streamBuffer.size() == 0) return null
    val endOffset = streamBuffer.size()
    val blobBytes = streamBuffer.toByteArray().copyOf(BLOB_BYTES_LENGTH)
    val chunkHash = chunkHasher(blobBytes)
    val nextDrh = DataRollingHashCalculator.fold(parentDataRollingHash, chunkHash)
    val chunk = SealedChunk(blobBytes, chunkHash, parentDataRollingHash, endOffset)
    parentDataRollingHash = nextDrh
    streamBuffer.reset()
    return chunk
  }

  private fun buildRequest(chunks: List<SealedChunk>, conflations: List<PendingConflation>): RollupProofRequestV1 {
    val lastChunk = chunks.last()
    val opaqueSuffixBytes = ByteArray(BLOB_BYTES_LENGTH - lastChunk.endOffset)

    return RollupProofRequestV1(
      conflations = conflations.map { ConflationWitness(it.blocks.map(blockEncoder::encode)) },
      l2Executions =
      conflations.map {
        BlockIntervalProofIndex(
          startBlockNumber = it.conflationResult.startBlockNumber,
          endBlockNumber = it.conflationResult.endBlockNumber,
          startBlockTimestamp = Instant.fromEpochSeconds(it.blocks.first().timestamp.toLong()),
          hash = TODO("execution proof request hash — see Open Question #4"),
        )
      },
      chunks = chunks.map { it.chunkHash },
      parentDataRollingHash = chunks.first().parentDataRollingHash,
      startOffset = 0,
      opaquePrefixBytes = ByteArray(0),
      opaqueSuffixBytes = opaqueSuffixBytes,
      boundaryPrevDataRollingHash = null,
    )
  }

  override fun action(): SafeFuture<*> = trySubmitRollupProof()
}
