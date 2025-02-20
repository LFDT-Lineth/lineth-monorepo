package linea.staterecovery

import io.vertx.core.Vertx
import linea.domain.BlockParameter
import linea.domain.CommonDomainFunctions
import linea.kotlin.encodeHex
import linea.staterecovery.datafetching.SubmissionsFetchingTask
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class StateSynchronizerService(
  private val vertx: Vertx,
  private val l1EarliestBlockWithFinalizationThatSupportRecovery: BlockParameter,
  private val elClient: ExecutionLayerClient,
  private val submissionEventsClient: LineaRollupSubmissionEventsClient,
  private val blobsFetcher: BlobFetcher,
  private val transactionDetailsClient: TransactionDetailsClient,
  private val blobDecompressor: BlobDecompressorAndDeserializer,
  private val blockImporterAndStateVerifier: BlockImporterAndStateVerifier,
  private val pollingInterval: Duration,
  private val debugForceSyncStopBlockNumber: ULong?,
  private val log: Logger = LogManager.getLogger(StateSynchronizerService::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  log = log,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds
) {
  var stateRootMismatchFound: Boolean = false
    private set(value) {
      field = value
    }
  lateinit var blobsFetcherTask: SubmissionsFetchingTask

  @get:Synchronized
  @set:Synchronized
  private var lookbackHashesInitalized = false

  override fun start(): SafeFuture<Unit> {
    log.debug("starting L1 -> ExecutionLayer state importer service")
    return this.elClient
      .lineaGetStateRecoveryStatus()
      .thenCompose { status ->
        val l2StartBlockNumberToFetchInclusive = startBlockToFetchFromL1(
          headBlockNumber = status.headBlockNumber,
          recoveryStartBlockNumber = status.stateRecoverStartBlockNumber,
          lookbackWindow = 256UL
        )

        this.blobsFetcherTask = SubmissionsFetchingTask(
          vertx = vertx,
          l1EarliestBlockWithFinalizationThatSupportRecovery = l1EarliestBlockWithFinalizationThatSupportRecovery,
          l1PollingInterval = pollingInterval,
          l2StartBlockNumberToFetchInclusive = l2StartBlockNumberToFetchInclusive,
          submissionEventsClient = submissionEventsClient,
          blobsFetcher = blobsFetcher,
          transactionDetailsClient = transactionDetailsClient,
          blobDecompressor = blobDecompressor,
          submissionEventsQueueLimit = 10,
          compressedBlobsQueueLimit = 10,
          targetDecompressedBlobsQueueLimit = 10,
          debugForceSyncStopBlockNumber = debugForceSyncStopBlockNumber
        )
        blobsFetcherTask.start()
      }
      .thenCompose { initLookbackHashes() }
      .thenCompose { super.start() }
      .whenException {
        log.error("failed to start L1 -> ExecutionLayer state importer service", it)
      }.whenSuccess {
        log.debug("started L1 -> ExecutionLayer state importer service")
      }
  }

  override fun action(): SafeFuture<*> {
    if (stateRootMismatchFound) {
      return SafeFuture.failedFuture<Unit>(IllegalStateException("state root mismatch found cannot continue"))
    }

    return importNextFinalizationAvailable()
  }

  fun initLookbackHashes(): SafeFuture<Unit> {
    if (lookbackHashesInitalized) {
      return SafeFuture.completedFuture(Unit)
    }
    val loobackHasheFetcher = LookBackBlockHashesFetcher(
      vertx = vertx,
      elClient = elClient,
      submissionsFetcher = blobsFetcherTask
    )

    return this.elClient
      .lineaGetStateRecoveryStatus()
      .thenCompose(loobackHasheFetcher::getLookBackHashes)
      .thenApply { lookbackHashes ->
        elClient.addLookbackHashes(lookbackHashes)
        lookbackHashesInitalized = true
      }
  }

  private fun importNextFinalizationAvailable(): SafeFuture<*> {
    val nexFinalization = blobsFetcherTask
      .peekNextFinalizationReadyToImport()
      ?: run {
        log.trace("no finalization ready to import")
        return SafeFuture.completedFuture(Unit)
      }

    return filterOutBlocksAlreadyImportedAndBeyondStopSync(nexFinalization.data)
      .thenCompose { blocksToImport ->
        if (blocksToImport.isEmpty()) {
          log.debug(
            "no blocks to import for finalization={}",
            nexFinalization.submissionEvents.dataFinalizedEvent.event
          )
          return@thenCompose SafeFuture.completedFuture(Unit)
        }

        importBlocksAndAssertStateroot(
          decompressedBlocksToImport = blocksToImport,
          dataFinalizedV3 = nexFinalization.submissionEvents.dataFinalizedEvent.event
        )
      }
      .thenPeek {
        blobsFetcherTask.pruneQueueForElementsUpToInclusive(
          nexFinalization.submissionEvents.dataFinalizedEvent.event.endBlockNumber
        )
      }
  }

  private fun importBlocksAndAssertStateroot(
    decompressedBlocksToImport: List<BlockFromL1RecoveredData>,
    dataFinalizedV3: DataFinalizedV3
  ): SafeFuture<Unit> {
    val blockInterval = CommonDomainFunctions.blockIntervalString(
      decompressedBlocksToImport.first().header.blockNumber,
      decompressedBlocksToImport.last().header.blockNumber
    )
    log.debug("importing blocks={} from finalization={}", blockInterval, dataFinalizedV3.intervalString())
    return blockImporterAndStateVerifier
      .importBlocks(decompressedBlocksToImport)
      .thenCompose { importResult ->
        log.debug("imported blocks={}", dataFinalizedV3.intervalString())
        assertStateMatches(importResult, dataFinalizedV3)
      }
  }

  private fun filterOutBlocksAlreadyImportedAndBeyondStopSync(
    blocks: List<BlockFromL1RecoveredData>
  ): SafeFuture<List<BlockFromL1RecoveredData>> {
    return elClient.getBlockNumberAndHash(blockParameter = BlockParameter.Tag.LATEST)
      .thenApply { headBlock ->
        var filteredBlocks = blocks.dropWhile { it.header.blockNumber <= headBlock.number }
        if (debugForceSyncStopBlockNumber != null) {
          filteredBlocks = filteredBlocks.takeWhile { it.header.blockNumber <= debugForceSyncStopBlockNumber }
        }
        filteredBlocks
      }
  }

  private fun assertStateMatches(
    importResult: ImportResult,
    finalizedV3: DataFinalizedV3
  ): SafeFuture<Unit> {
    if (importResult.blockNumber != finalizedV3.endBlockNumber) {
      log.info(
        "cannot compare stateroot: last imported block={} finalization={} debugForceSyncStopBlockNumber={}",
        importResult.blockNumber,
        finalizedV3.intervalString(),
        debugForceSyncStopBlockNumber
      )
      if (importResult.blockNumber == debugForceSyncStopBlockNumber) {
        // this means debugForceSyncStopBlockNumber was set and we stopped before reaching the target block
        // so just stop the service
        this.stop()
      }
    } else if (importResult.zkStateRootHash.contentEquals(finalizedV3.finalStateRootHash)) {
      log.info(
        "state recovered up to finalization={} zkStateRootHash={}",
        finalizedV3.intervalString(),
        importResult.zkStateRootHash.encodeHex()
      )
    } else {
      log.error(
        "stopping data recovery from L1, stateRootHash mismatch: " +
          "finalization={} recovered block={} yielded recoveredStateRootHash={} expected to have " +
          "l1 proven stateRootHash={}",
        finalizedV3.intervalString(),
        importResult.blockNumber,
        importResult.zkStateRootHash.encodeHex(),
        finalizedV3.finalStateRootHash.encodeHex()
      )
      stateRootMismatchFound = true
      this.stop()
    }
    return SafeFuture.completedFuture(Unit)
  }
}
