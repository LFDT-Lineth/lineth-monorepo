package linea.staterecovery

import build.linea.domain.EthLogEvent
import io.vertx.core.Vertx
import net.consensys.encodeHex
import net.consensys.linea.BlockNumberAndHash
import net.consensys.linea.BlockParameter
import net.consensys.linea.CommonDomainFunctions
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class StateSynchronizerService(
  private val vertx: Vertx,
  private val elClient: ExecutionLayerClient,
  private val submissionEventsClient: LineaRollupSubmissionEventsClient,
  private val blobsFetcher: BlobFetcher,
  private val transactionDetailsClient: TransactionDetailsClient,
  private val blobDecompressor: BlobDecompressorAndDeserializer,
  private val blockImporterAndStateVerifier: BlockImporterAndStateVerifier,
  private val pollingInterval: Duration,
  private val log: Logger = LogManager.getLogger(StateSynchronizerService::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  log = log,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds
) {
  private data class DataSubmittedEventAndBlobs(
    val ethLogEvent: EthLogEvent<DataSubmittedV3>,
    val blobs: List<ByteArray>
  )

  var lastSuccessfullyProcessedFinalization: EthLogEvent<DataFinalizedV3>? = null
  var stateRootMismatchFound: Boolean = false
    private set(value) {
      field = value
    }

  private fun findNextFinalization(): SafeFuture<EthLogEvent<DataFinalizedV3>?> {
    return if (lastSuccessfullyProcessedFinalization != null) {
      submissionEventsClient
        .findDataFinalizedEventByStartBlockNumber(
          l2StartBlockNumberInclusive = lastSuccessfullyProcessedFinalization!!.event.endBlockNumber + 1UL
        )
    } else {
      elClient.getBlockNumberAndHash(blockParameter = BlockParameter.Tag.LATEST)
        .thenCompose { headBlock ->
          // 1st, assuming head matches a prev finalization,
          val nextBlockToImport = headBlock.number + 1UL
          submissionEventsClient
            .findDataFinalizedEventByStartBlockNumber(l2StartBlockNumberInclusive = nextBlockToImport)
            .thenCompose { finalizationEvent ->
              if (finalizationEvent != null) {
                SafeFuture.completedFuture(finalizationEvent)
              } else {
                // 2nd: otherwise, local head may be in between, let's find corresponding finalization
                submissionEventsClient
                  .findDataFinalizedEventContainingBlock(l2BlockNumber = nextBlockToImport)
              }
            }
        }
    }
  }

  override fun action(): SafeFuture<Any?> {
    if (stateRootMismatchFound) {
      return SafeFuture.failedFuture(IllegalStateException("state root mismatch found cannot continue"))
    }

    return findNextFinalization()
      .thenCompose { nextFinalization ->
        if (nextFinalization == null) {
          // nothing to do for now
          SafeFuture.completedFuture(null)
        } else {
          submissionEventsClient
            .findDataSubmittedV3EventsUntilNextFinalization(
              l2StartBlockNumberInclusive = nextFinalization.event.startBlockNumber
            )
        }
      }
      .thenCompose { submissionEvents ->
        if (submissionEvents == null) {
          SafeFuture.completedFuture("No new events")
        } else {
          getBlobsForEvents(submissionEvents.dataSubmittedEvents)
            .thenCompose { dataSubmissionsWithBlobs ->
              updateNodeWithBlobsAndVerifyState(dataSubmissionsWithBlobs, submissionEvents.dataFinalizedEvent.event)
            }
            .thenApply {
              lastSuccessfullyProcessedFinalization = submissionEvents.dataFinalizedEvent
            }
        }
      }
  }

  private fun getBlobsForEvents(
    events: List<EthLogEvent<DataSubmittedV3>>
  ): SafeFuture<List<DataSubmittedEventAndBlobs>> {
    return SafeFuture.collectAll(
      events
        .map { dataSubmittedEvent ->
          transactionDetailsClient
            .getBlobVersionedHashesByTransactionHash(dataSubmittedEvent.log.transactionHash)
            .thenCompose(blobsFetcher::fetchBlobsByHash)
            .thenApply { blobs -> DataSubmittedEventAndBlobs(dataSubmittedEvent, blobs) }
        }.stream()
    )
  }

  private fun updateNodeWithBlobsAndVerifyState(
    dataSubmissions: List<DataSubmittedEventAndBlobs>,
    dataFinalizedV3: DataFinalizedV3
  ): SafeFuture<BlockNumberAndHash> {
    return blobDecompressor
      .decompress(
        startBlockNumber = dataFinalizedV3.startBlockNumber,
        blobs = dataSubmissions.flatMap { it.blobs }
      )
      .thenCompose(this::filterOutBlocksAlreadyImported)
      .thenCompose { decompressedBlocks: List<BlockFromL1RecoveredData> ->
        val blockInterval = CommonDomainFunctions.blockIntervalString(
          decompressedBlocks.first().header.blockNumber,
          decompressedBlocks.last().header.blockNumber
        )
        log.debug("importing blocks={} from finalization={}", blockInterval, dataFinalizedV3.intervalString())
        blockImporterAndStateVerifier
          .importBlocks(decompressedBlocks)
          .thenCompose { importResult ->
            log.debug("imported blocks={}", dataFinalizedV3.intervalString())
            assertStateMatches(importResult, dataFinalizedV3)
          }
          .thenApply {
            BlockNumberAndHash(
              number = decompressedBlocks.last().header.blockNumber,
              hash = decompressedBlocks.last().header.blockHash
            )
          }
      }
  }

  private fun filterOutBlocksAlreadyImported(
    blocks: List<BlockFromL1RecoveredData>
  ): SafeFuture<List<BlockFromL1RecoveredData>> {
    return elClient.getBlockNumberAndHash(blockParameter = BlockParameter.Tag.LATEST)
      .thenApply { headBlock ->
        blocks.dropWhile { it.header.blockNumber <= headBlock.number }
      }
  }

  private fun assertStateMatches(
    importResult: ImportResult,
    finalizedV3: DataFinalizedV3
  ): SafeFuture<Unit> {
    return if (importResult.zkStateRootHash.contentEquals(finalizedV3.finalStateRootHash)) {
      log.info(
        "state recovered up to block={} zkStateRootHash={} finalization={}",
        importResult.blockNumber,
        importResult.zkStateRootHash.encodeHex(),
        finalizedV3.intervalString()
      )
      SafeFuture.completedFuture(Unit)
    } else {
      log.error(
        "stopping data recovery from L1, stateRootHash mismatch: " +
          "finalization={} recoveredStateRootHash={} expected block={} to have l1 proven stateRootHash={}",
        finalizedV3.intervalString(),
        finalizedV3.finalStateRootHash.encodeHex(),
        importResult.zkStateRootHash.encodeHex(),
        finalizedV3.endBlockNumber
      )
      stateRootMismatchFound = true
      this.stop()
      SafeFuture.completedFuture(Unit)
    }
  }
}
