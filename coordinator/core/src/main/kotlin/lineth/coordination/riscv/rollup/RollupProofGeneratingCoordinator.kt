package lineth.coordination.riscv.rollup

import io.vertx.core.Vertx
import linea.clients.ChunkWitness
import linea.clients.ConflationWitness
import linea.clients.RollupProofRequestV1
import linea.clients.RollupProverClientV1
import linea.da.DaCodecVersion
import linea.da.DaCompressor
import linea.da.DaCompressorFactory
import linea.da.DaStream
import linea.da.DaStreamFactory
import linea.da.RollupResumePoint
import linea.domain.Batch
import linea.domain.BlobData
import linea.domain.Block
import linea.domain.BlockIntervalProofIndex
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.kotlin.toInstantFromEpochSeconds
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import lineth.conflation.ConflationHandler
import lineth.conflation.calculators.BlobTriggerCalculator
import lineth.conflation.calculators.GlobalBlobCalculator
import lineth.conflation.calculators.GlobalRollupCalculator
import lineth.conflation.calculators.Rollup
import lineth.conflation.calculators.RollupDaAssembler
import lineth.conflation.calculators.RollupHandler
import lineth.conflation.calculators.RollupTriggerCalculator
import lineth.encoding.BlockEncoder
import lineth.persistence.BatchesRepository
import net.consensys.linea.traces.TracesCountersV5
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import java.util.concurrent.ConcurrentHashMap
import kotlin.time.Clock
import kotlin.time.Duration

/** Fetches the L2 blocks `startBlockNumber..endBlockNumber`, in order. */
fun interface L2BlocksFetcher {
  fun fetchBlocks(startBlockNumber: ULong, endBlockNumber: ULong): SafeFuture<List<Block>>
}

/**
 * Builds RISC-V rollup proof requests from the DA stream.
 *
 * The DA stream is fed only from proven, persisted `batches` rows, using their boundaries: conflation boundaries are
 * not guaranteed to match across restarts, but persisted ones are, so a restart rebuilds byte-identical blobs. Blocks
 * come from conflations cached by [handleConflatedBatch] when their boundaries match a persisted batch, and from
 * [l2BlocksFetcher] otherwise.
 *
 * Each tick of [action]:
 * 1. until the resume point is known, only waits for it: the [resumePointSupplier] future may take long (e.g. until L1
 *    finalizes the last pre-RISC-V block), so a tick never blocks on it. Then opens the [DaStream] and the calculators
 *    at the resume point;
 * 2. feeds the next persisted batches to the [GlobalBlobCalculator], which emits rollups to [onRollup];
 * 3. checks the blob and rollup deadlines;
 * 4. submits the pending rollups in order. A rollup is dropped from the queue only once its request succeeds.
 *
 * All DA stream and calculator calls run under this object's monitor, and ticks never overlap (fixed-delay timer).
 * An error from the DA stream or the calculators leaves them in an unknown state, so the rollup stage stops there.
 */
class RollupProofGeneratingCoordinator(
  private val rollupProverClient: RollupProverClientV1,
  private val batchesRepository: BatchesRepository,
  private val resumePointSupplier: () -> SafeFuture<RollupResumePoint>,
  private val rollupProofPoller: RollupProofPoller,
  private val l2BlocksFetcher: L2BlocksFetcher,
  private val daCompressorFactory: DaCompressorFactory,
  private val daStreamFactory: DaStreamFactory,
  private val blobTriggers: List<BlobTriggerCalculator>,
  private val rollupTriggers: List<RollupTriggerCalculator>,
  private val blockEncoder: BlockEncoder,
  private val config: Config,
  private val vertx: Vertx,
  private val clock: Clock = Clock.System,
  private val log: Logger = LogManager.getLogger(RollupProofGeneratingCoordinator::class.java),
) : ConflationHandler,
  RollupHandler,
  VertxPeriodicPollingService(
    vertx = vertx,
    name = "RollupProofGeneratingCoordinator",
    pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
    log = log,
    timerSchedule = TimerSchedule.FIXED_DELAY,
  ) {

  data class Config(
    val pollingInterval: Duration,
    val daCodecVersion: DaCodecVersion,
    val dictionaryPath: Path,
    val blobDeadline: Duration? = null,
    val rollupDeadline: Duration? = null,
    val maxConflationsToFeedPerTick: Int = 100,
  )

  private class DaPipeline(
    val compressor: DaCompressor,
    val daStream: DaStream,
    val blobCalculator: GlobalBlobCalculator,
    val rollupCalculator: GlobalRollupCalculator,
  )

  private var pipeline: DaPipeline? = null

  @Volatile
  private var failure: Throwable? = null

  @Volatile
  private var lastFedBlockNumber: ULong = 0UL
  private val pendingRollups = ArrayDeque<Rollup>()
  private val conflationCache = ConcurrentHashMap<Pair<ULong, ULong>, BlocksConflation>()

  /** Number of cached conflations, for tests. */
  internal val cachedConflationsCount: Int get() = conflationCache.size

  /** Caches the conflation's blocks until it is fed. A stage stopped by a DA pipeline error caches nothing. */
  override fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*> {
    if (failure == null && conflation.endBlockNumber > lastFedBlockNumber) {
      conflationCache[conflation.startBlockNumber to conflation.endBlockNumber] = conflation
    }
    return SafeFuture.completedFuture(Unit)
  }

  /** Called by the rollup calculator while [feed] holds the monitor: only enqueues. */
  override fun onRollup(rollup: Rollup) {
    pendingRollups.addLast(rollup)
  }

  override fun action(): SafeFuture<*> {
    failure?.let {
      log.error("rollup stage stopped after a DA pipeline error: errorMessage={}", it.message)
      return SafeFuture.completedFuture(Unit)
    }
    if (!ensureInitialized()) return SafeFuture.completedFuture(Unit)
    return loadNextProvenConflations()
      .thenApply { conflations -> feed(conflations) }
      .thenCompose { submitPendingRollups() }
  }

  override fun stop(): SafeFuture<Unit> {
    return super.stop().thenApply {
      synchronized(this) {
        pipeline?.let {
          it.daStream.close()
          it.compressor.close()
        }
        pipeline = null
      }
    }
  }

  private var pendingResumePoint: SafeFuture<RollupResumePoint>? = null

  /**
   * Whether the pipeline is open. Requests the resume point once and opens the pipeline when it is known; a failed
   * request is retried on the next tick.
   */
  @Synchronized
  private fun ensureInitialized(): Boolean {
    if (pipeline != null) return true
    val resumePoint = pendingResumePoint ?: resumePointSupplier().also { pendingResumePoint = it }
    if (!resumePoint.isDone) return false
    pendingResumePoint = null
    initPipeline(resumePoint.join())
    return true
  }

  private fun initPipeline(resumePoint: RollupResumePoint) {
    if (pipeline != null) return
    val position = resumePoint.daPosition
    val compressor = daCompressorFactory.create(config.daCodecVersion, config.dictionaryPath)
    val daStream = daStreamFactory.open(config.daCodecVersion, position)
    lateinit var blobCalculator: GlobalBlobCalculator
    val rollupCalculator = GlobalRollupCalculator(
      triggers = rollupTriggers,
      openRollup = RollupDaAssembler(position),
      blobCalculator = { blobCalculator },
      rollupDeadline = config.rollupDeadline,
    ).also { it.onRollup(this) }
    blobCalculator = GlobalBlobCalculator(
      compressor = compressor,
      daStream = daStream,
      triggers = blobTriggers,
      blockEncoder = blockEncoder,
      handler = rollupCalculator,
      blobDeadline = config.blobDeadline,
      sharedBlobCarryOverTailBytes = position.sharedBlobCarryOverTailBytes,
    )
    pipeline = DaPipeline(compressor, daStream, blobCalculator, rollupCalculator)
    lastFedBlockNumber = resumePoint.lastConflationEndBlock
    log.info("rollup stage resumed: lastConflationEndBlock={} daPosition={}", lastFedBlockNumber, position)
  }

  /** The next persisted batches after [lastFedBlockNumber], with their blocks, in order and without gaps. */
  private fun loadNextProvenConflations(): SafeFuture<List<BlocksConflation>> {
    val from = lastFedBlockNumber + 1UL
    return batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(from.toLong())
      .thenCompose { highest ->
        if (highest == null) {
          SafeFuture.completedFuture(emptyList())
        } else {
          batchesRepository.findBatchesByBlockRange(from.toLong(), highest).thenCompose { batches ->
            consecutiveBatches(from, batches)
              .take(config.maxConflationsToFeedPerTick)
              .fold(SafeFuture.completedFuture(emptyList<BlocksConflation>())) { acc, batch ->
                acc.thenCompose { loaded -> conflationOf(batch).thenApply { loaded + it } }
              }
          }
        }
      }
  }

  /** The gap-free chain of batches from [from]; where several share a start block, the shortest wins. */
  private fun consecutiveBatches(from: ULong, batches: List<Batch>): List<Batch> {
    val byStart = batches.groupBy { it.startBlockNumber }.mapValues { (_, it) -> it.minBy { it.endBlockNumber } }
    return generateSequence(byStart[from]) { byStart[it.endBlockNumber + 1UL] }.toList()
  }

  private fun conflationOf(batch: Batch): SafeFuture<BlocksConflation> {
    conflationCache[batch.startBlockNumber to batch.endBlockNumber]?.let { return SafeFuture.completedFuture(it) }
    return l2BlocksFetcher.fetchBlocks(batch.startBlockNumber, batch.endBlockNumber).thenApply { blocks ->
      check(blocks.map { it.number } == (batch.startBlockNumber..batch.endBlockNumber).toList()) {
        "fetched blocks=${blocks.map { it.number }} do not match " +
          "batch=${batch.startBlockNumber}..${batch.endBlockNumber}"
      }
      BlocksConflation(
        blocks = blocks,
        conflationResult = ConflationCalculationResult(
          startBlockNumber = batch.startBlockNumber,
          endBlockNumber = batch.endBlockNumber,
          // The trigger and traces are not persisted with a batch; the DA pipeline does not use them.
          conflationTrigger = ConflationTrigger.BLOCKS_LIMIT,
          tracesCounters = TracesCountersV5.EMPTY_TRACES_COUNT,
        ),
      )
    }
  }

  @Synchronized
  private fun feed(conflations: List<BlocksConflation>) {
    val pipeline = this.pipeline ?: return
    try {
      conflations.forEach { conflation ->
        check(conflation.startBlockNumber == lastFedBlockNumber + 1UL) {
          "conflation=${conflation.intervalString()} does not follow lastFedBlockNumber=$lastFedBlockNumber"
        }
        pipeline.blobCalculator.newConflation(conflation)
        lastFedBlockNumber = conflation.endBlockNumber
      }
      val now = clock.now()
      pipeline.blobCalculator.checkDeadline(now)
      pipeline.rollupCalculator.checkDeadline(now)
    } catch (e: Exception) {
      failure = e
      log.error("DA pipeline failed, stopping the rollup stage: errorMessage={}", e.message, e)
      throw e
    } finally {
      if (failure != null) {
        // The stopped stage never feeds again.
        conflationCache.clear()
      } else {
        conflationCache.keys.removeIf { (_, end) -> end <= lastFedBlockNumber }
      }
    }
  }

  private fun submitPendingRollups(): SafeFuture<Unit> {
    val next = synchronized(this) { pendingRollups.firstOrNull() } ?: return SafeFuture.completedFuture(Unit)
    return submitRollup(next)
      .whenException { th ->
        log.warn(
          "rollup proof request failed, will retry: rollup={}..{} errorMessage={}",
          next.startBlockNumber,
          next.endBlockNumber,
          th.message,
        )
      }
      .thenCompose {
        synchronized(this) { check(pendingRollups.removeFirst() === next) }
        submitPendingRollups()
      }
  }

  private fun submitRollup(rollup: Rollup): SafeFuture<Unit> {
    return findL2Executions(rollup).thenCompose { l2Executions ->
      val request = buildRequest(rollup, l2Executions)
      val proofIndex = rollupProverClient.getProofIndex(request)
      // A request rebuilt after a restart is byte-identical, so its proof may already exist.
      rollupProverClient.findProofResponse(proofIndex)
        .thenCompose { existing ->
          if (existing != null) {
            SafeFuture.completedFuture(proofIndex)
          } else {
            rollupProverClient.createProofRequest(request)
          }
        }
        .thenApply { trackProofInProgress(it, rollup) }
    }
  }

  private fun findL2Executions(rollup: Rollup): SafeFuture<List<BlockIntervalProofIndex>> {
    return batchesRepository.findBatchesByBlockRange(
      rollup.startBlockNumber.toLong(),
      rollup.endBlockNumber.toLong(),
    ).thenApply { batches ->
      val batchesByStartAndEnd = batches.associateBy { it.startBlockNumber to it.endBlockNumber }
      rollup.conflations.map { conflation ->
        val batch = batchesByStartAndEnd[conflation.startBlockNumber to conflation.endBlockNumber]
          ?: error("Batch not found for conflation ${conflation.intervalString()}")
        val hash = batch.proofIndexHash
          ?: error("proofIndexHash is null for batch ${conflation.intervalString()}")
        BlockIntervalProofIndex(
          startBlockNumber = conflation.startBlockNumber,
          endBlockNumber = conflation.endBlockNumber,
          startBlockTimestamp = conflation.blocks.first().timestamp.toInstantFromEpochSeconds(),
          hash = hash,
        )
      }
    }
  }

  private fun buildRequest(rollup: Rollup, l2Executions: List<BlockIntervalProofIndex>): RollupProofRequestV1 {
    val da = rollup.da
    return RollupProofRequestV1(
      conflations = rollup.conflations.map { ConflationWitness(it.blocks.map(blockEncoder::encode)) },
      l2Executions = l2Executions,
      chunks = da.blobs.map {
        ChunkWitness(
          chunkHash = it.chunkHash,
          isCalldata = false,
          blobBytes = it.blob,
          calldataBytes = ByteArray(0),
          kzgCommitment = it.kzgCommitment,
          kzgProof = it.kzgProof,
        )
      },
      parentDataRollingHash = da.parentDataRollingHash,
      parentDataTailTakeBytes = da.parentDataTailTakeBytes,
      boundaryPrevDataRollingHash = da.boundaryPrevDataRollingHash,
    )
  }

  private fun trackProofInProgress(proofIndex: BlockIntervalProofIndex, rollup: Rollup) {
    val da = rollup.da
    log.info(
      "rollup proof request created: rollup={}..{} blobs={} tailTake={} tailDiscard={}",
      rollup.startBlockNumber,
      rollup.endBlockNumber,
      da.blobs.size,
      da.parentDataTailTakeBytes,
      da.expectedFinalDataTailDiscardBytes,
    )
    rollupProofPoller.addProofInProgress(
      proofIndex = proofIndex,
      blobsData = da.blobs.map {
        BlobData(
          chunkHash = it.chunkHash,
          blobBytes = it.blob,
          batchesCount = it.segmentEnds.size.toUInt(),
          prevDataRollingHash = it.prevDataRollingHash,
          dataRollingHash = it.dataRollingHash,
        )
      },
      parentDataRollingHash = da.parentDataRollingHash,
      dataRollingHash = da.expectedEndDataRollingHash,
      parentDataTailTakeBytes = da.parentDataTailTakeBytes,
      boundaryPrevDataRollingHash = da.boundaryPrevDataRollingHash,
      finalDataTailDiscardBytes = da.expectedFinalDataTailDiscardBytes,
      startBlockTimestamp = rollup.conflations.first().blocks.first().timestamp.toInstantFromEpochSeconds(),
      endBlockTimestamp = rollup.conflations.last().blocks.last().timestamp.toInstantFromEpochSeconds(),
      totalBatchesCount = rollup.conflations.size,
    )
  }
}
