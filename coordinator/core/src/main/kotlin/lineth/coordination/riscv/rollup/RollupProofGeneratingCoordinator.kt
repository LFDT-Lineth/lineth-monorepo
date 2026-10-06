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
import lineth.conflation.calculators.RollupCalculator
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
  private val rollupCalculator: RollupCalculator,
  private val rollupProverClient: RollupProverClientV1,
  private val batchesRepository: BatchesRepository,
  private val streamPositionProvider: StreamPositionProvider,
  private val rollupProofPoller: RollupProofPoller,
  private val conflationSegmentBuilder: ConflationSegmentBuilder,
  private val dataRollingHashCalculator: DataRollingHashCalculator,
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
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false
      other as SealedChunk
      if (endOffset != other.endOffset) return false
      if (!blobBytes.contentEquals(other.blobBytes)) return false
      if (!chunkHash.contentEquals(other.chunkHash)) return false
      if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
      return true
    }

    override fun hashCode(): Int {
      var result = endOffset
      result = 31 * result + blobBytes.contentHashCode()
      result = 31 * result + chunkHash.contentHashCode()
      result = 31 * result + parentDataRollingHash.contentHashCode()
      return result
    }
  }

  private data class ChunkedData(
    val chunks: List<SealedChunk>,
    // data rolling hash after folding the last chunk
    val dataRollingHash: ByteArray,
  ) {
    val parentDataRollingHash: ByteArray get() = chunks.first().parentDataRollingHash
    val endOffset: Int get() = chunks.last().endOffset

    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false
      other as ChunkedData
      if (chunks != other.chunks) return false
      if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
      return true
    }

    override fun hashCode(): Int {
      var result = chunks.hashCode()
      result = 31 * result + dataRollingHash.contentHashCode()
      return result
    }
  }

  private data class PendingConflation(
    val conflationResult: ConflationCalculationResult,
    val blocks: List<Block>,
    val segmentBytes: ByteArray,
  ) {
    val startBlockNumber: ULong get() = conflationResult.startBlockNumber
    val endBlockNumber: ULong get() = conflationResult.endBlockNumber
    val startBlockTimestamp: Instant get() = Instant.fromEpochSeconds(blocks.first().timestamp.toLong())
    val endBlockTimestamp: Instant get() = Instant.fromEpochSeconds(blocks.last().timestamp.toLong())

    fun intervalString(): String = "$startBlockNumber..$endBlockNumber"

    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as PendingConflation

      if (conflationResult != other.conflationResult) return false
      if (blocks != other.blocks) return false
      if (!segmentBytes.contentEquals(other.segmentBytes)) return false

      return true
    }

    override fun hashCode(): Int {
      var result = conflationResult.hashCode()
      result = 31 * result + blocks.hashCode()
      result = 31 * result + segmentBytes.contentHashCode()
      return result
    }
  }

  private lateinit var parentDataRollingHash: ByteArray
  private var lastHandledBlockNumber: ULong = 0u
  private var nextBlockNumberToPoll: Long? = null
  private val pendingConflations = ArrayDeque<PendingConflation>()
  private var streamPositionFuture: SafeFuture<Unit>? = null

  init {
    rollupCalculator.onRollup { window -> submitProofWindow(window) }
  }

  private fun ensureStreamPositionInitialized(): SafeFuture<Unit> =
    synchronized(this) {
      if (::parentDataRollingHash.isInitialized) return SafeFuture.completedFuture(Unit)
      streamPositionFuture?.let { return it }
      streamPositionProvider.getStreamPosition()
        .thenApply { position ->
          synchronized(this) {
            if (!::parentDataRollingHash.isInitialized) {
              parentDataRollingHash = position.dataRollingHash
              lastHandledBlockNumber = position.lastConflationEndBlock
              nextBlockNumberToPoll = position.lastConflationEndBlock.toLong() + 1L
            }
          }
        }.toSafeFuture()
        .also { streamPositionFuture = it }
    }

  override fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*> {
    return ensureStreamPositionInitialized().thenCompose {
      synchronized(this) {
        require(conflation.conflationResult.startBlockNumber == lastHandledBlockNumber + 1u) {
          "Conflation out of order: expected startBlockNumber=${lastHandledBlockNumber + 1u}, " +
            "got ${conflation.conflationResult.startBlockNumber}"
        }
        val segmentBytes = conflationSegmentBuilder.buildSegment(conflation.blocks, chainId)
        pendingConflations += PendingConflation(conflation.conflationResult, conflation.blocks, segmentBytes)
        lastHandledBlockNumber = conflation.conflationResult.endBlockNumber
        SafeFuture.completedFuture(Unit)
      }
    }
  }

  override fun action(): SafeFuture<*> {
    val pollFrom = nextBlockNumberToPoll ?: return SafeFuture.completedFuture(Unit)
    return batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(pollFrom)
      .thenApply { highestProven ->
        if (highestProven != null) processProvenConflations(highestProven)
      }
  }

  @Synchronized
  private fun processProvenConflations(highestProven: Long) {
    val provenPending = buildList {
      for (pending in pendingConflations) {
        if (pending.endBlockNumber.toLong() > highestProven) break
        add(pending)
      }
    }
    provenPending.forEach { pending ->
      rollupCalculator.newConflation(BlocksConflation(pending.blocks, pending.conflationResult))
      nextBlockNumberToPoll = pending.endBlockNumber.toLong() + 1L
    }
  }

  @Synchronized
  private fun submitProofWindow(window: List<BlocksConflation>) {
    val windowPending = drainWindow(window)
    val chunkedData = chunkSegments(windowPending)
    parentDataRollingHash = chunkedData.dataRollingHash

    findL2Executions(windowPending)
      .thenCompose { l2Executions ->
        rollupProverClient.createProofRequest(buildRequest(chunkedData, windowPending, l2Executions))
      }
      .thenApply { proofIndex -> trackProofInProgress(proofIndex, chunkedData, windowPending) }
  }

  private fun chunkSegments(windowPending: List<PendingConflation>): ChunkedData {
    val buffer = ByteArrayOutputStream()
    val sealedChunks = mutableListOf<SealedChunk>()
    var currentDrh = parentDataRollingHash

    for (pending in windowPending) {
      buffer.write(pending.segmentBytes)
      while (buffer.size() >= Constants.Eip4844BlobSize) {
        val raw = buffer.toByteArray()
        val blobBytes = raw.copyOfRange(0, Constants.Eip4844BlobSize)
        buffer.reset()
        buffer.write(raw, Constants.Eip4844BlobSize, raw.size - Constants.Eip4844BlobSize)
        val chunkHash = chunkHasher(blobBytes)
        sealedChunks += SealedChunk(blobBytes, chunkHash, currentDrh, Constants.Eip4844BlobSize)
        currentDrh = dataRollingHashCalculator.fold(currentDrh, chunkHash)
      }
    }

    val partialChunk: SealedChunk? = if (buffer.size() > 0) {
      val endOffset = buffer.size()
      val blobBytes = buffer.toByteArray().copyOf(Constants.Eip4844BlobSize)
      val chunkHash = chunkHasher(blobBytes)
      val chunk = SealedChunk(blobBytes, chunkHash, currentDrh, endOffset)
      currentDrh = dataRollingHashCalculator.fold(currentDrh, chunkHash)
      chunk
    } else {
      null
    }

    return ChunkedData(sealedChunks + listOfNotNull(partialChunk), currentDrh)
  }

  private fun drainWindow(window: List<BlocksConflation>): List<PendingConflation> {
    val windowPending = (1..window.size).map { pendingConflations.removeFirst() }
    check(
      windowPending.zip(window).all { (pending, conflation) ->
        pending.startBlockNumber == conflation.conflationResult.startBlockNumber &&
          pending.endBlockNumber == conflation.conflationResult.endBlockNumber
      },
    ) {
      "submitProofWindow: drained conflations don't match calculator window — " +
        "pending ${windowPending.map { it.startBlockNumber..it.endBlockNumber }}, " +
        "window ${window.map { it.conflationResult.startBlockNumber..it.conflationResult.endBlockNumber }}"
    }
    return windowPending
  }

  private fun findL2Executions(windowPending: List<PendingConflation>): SafeFuture<List<BlockIntervalProofIndex>> {
    return batchesRepository.findBatchesByBlockRange(
      windowPending.first().startBlockNumber.toLong(),
      windowPending.last().endBlockNumber.toLong(),
    ).thenApply { batches ->
      val batchesByStart = batches.associateBy { it.startBlockNumber }
      windowPending.map { pending ->
        val batch = batchesByStart[pending.startBlockNumber]
          ?: error("Batch not found for conflation ${pending.intervalString()}")
        val hash = batch.proofIndexHash
          ?: error("proofIndexHash is null for batch ${pending.intervalString()}")
        BlockIntervalProofIndex(
          startBlockNumber = pending.startBlockNumber,
          endBlockNumber = pending.endBlockNumber,
          startBlockTimestamp = pending.startBlockTimestamp,
          hash = hash,
        )
      }
    }
  }

  private fun trackProofInProgress(
    proofIndex: BlockIntervalProofIndex,
    chunkedData: ChunkedData,
    windowPending: List<PendingConflation>,
  ) {
    rollupProofPoller.addProofInProgress(
      proofIndex = proofIndex,
      blobsData = chunkedData.chunks.map {
        BlobData(chunkHash = it.chunkHash, blobBytes = it.blobBytes, batchesCount = 0u)
      },
      parentDataRollingHash = chunkedData.parentDataRollingHash,
      dataRollingHash = chunkedData.dataRollingHash,
      endOffset = chunkedData.endOffset,
      startBlockTimestamp = windowPending.first().startBlockTimestamp,
      endBlockTimestamp = windowPending.last().endBlockTimestamp,
      totalBatchesCount = windowPending.size,
    )
  }

  private fun buildRequest(
    chunkedData: ChunkedData,
    conflations: List<PendingConflation>,
    l2Executions: List<BlockIntervalProofIndex>,
  ): RollupProofRequestV1 {
    return RollupProofRequestV1(
      conflations = conflations.map { ConflationWitness(it.blocks.map(blockEncoder::encode)) },
      l2Executions = l2Executions,
      chunks = chunkedData.chunks.map { it.chunkHash },
      parentDataRollingHash = chunkedData.parentDataRollingHash,
      startOffset = 0,
      opaquePrefixBytes = ByteArray(0),
      opaqueSuffixBytes = ByteArray(Constants.Eip4844BlobSize - chunkedData.endOffset),
      boundaryPrevDataRollingHash = null,
    )
  }
}
