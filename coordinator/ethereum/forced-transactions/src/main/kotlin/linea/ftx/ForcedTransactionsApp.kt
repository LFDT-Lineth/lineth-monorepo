package linea.ftx

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.contract.events.ForcedTransactionAddedEvent
import linea.contract.l1.LineaRollupSmartContractClientReadOnlyFinalizedStateProvider
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.ethapi.EthLogsFilterSubscriptionFactoryPollingBased
import linea.forcedtx.ForcedTransactionsClient
import linea.persistence.ftx.ForcedTransactionsDao
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Queue
import java.util.concurrent.CompletableFuture
import java.util.concurrent.LinkedBlockingQueue
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class ForcedTransactionsApp(
  val config: Config,
  val vertx: Vertx,
  val l1EthApiClient: EthApiClient,
  val finalizedStateProvider: LineaRollupSmartContractClientReadOnlyFinalizedStateProvider,
  val ftxClient: ForcedTransactionsClient,
  val ftxDao: ForcedTransactionsDao,
) : LongRunningService {
  data class Config(
    val l1PollingInterval: Duration = 12.seconds,
    val l1ContractAddress: String,
    val l1HighestBlockTag: BlockParameter,
    val l1EventSearchBlockChunk: UInt = 1000u,
    val ftxSequencerSendingInterval: Duration = 5.seconds,
    val maxFtxToSendToSequencer: UInt = 10u,
  )

  private val log = LogManager.getLogger(ForcedTransactionsApp::class.java)
  private val ftxQueue: Queue<ForcedTransactionAddedEvent> = LinkedBlockingQueue(10_000)
  private val ftxResumePointProvider = ForcedTransactionsResumePointProviderImpl(
    finalizedStateProvider = finalizedStateProvider,
    l1HighestBlock = config.l1HighestBlockTag,
  )
  private lateinit var ftxStatusUpdater: ForcedTransactionsStatusUpdater
  private lateinit var ftxFetcher: ForcedTransactionsL1EventsFetcher
  private lateinit var ftxSender: ForcedTransactionsSenderForExecution

  override fun start(): CompletableFuture<Unit> {
    log.debug("starting ForcedTransactionsApp")
    // getLastProcessedForcedTransactionNumber relies on APi
    // that should have request retries
    return ftxResumePointProvider
      .getLastProcessedForcedTransactionNumber()
      .thenCompose { lastProcessedForcedTransactionNumber ->
        val lastProcessedFtxProvider = ForcedTransactionsResumePointProvider {
          SafeFuture.completedFuture(lastProcessedForcedTransactionNumber)
        }
        this.ftxStatusUpdater = ForcedTransactionsStatusUpdater(
          dao = this.ftxDao,
          ftxClient = this.ftxClient,
          ftxQueue = this.ftxQueue,
          lastProcessedFtxNumber = lastProcessedForcedTransactionNumber,
        )
        this.ftxSender = ForcedTransactionsSenderForExecution(
          vertx = vertx,
          ftxClient = this.ftxClient,
          pollingInterval = config.ftxSequencerSendingInterval,
          unprocessedFtxProvider = ftxStatusUpdater::getUnprocessedForcedTransactions,
          txLimitToSendPerTick = config.maxFtxToSendToSequencer.toInt(),
        )

        this.ftxFetcher = ForcedTransactionsL1EventsFetcher(
          address = config.l1ContractAddress,
          resumePointProvider = lastProcessedFtxProvider,
          ethLogsClient = l1EthApiClient,
          ethLogsFilterSubscriptionFactory = EthLogsFilterSubscriptionFactoryPollingBased(
            vertx = vertx,
            ethApiClient = l1EthApiClient,
            l1FtxLogsPollingInterval = config.l1PollingInterval,
            blockChunkSize = config.l1EventSearchBlockChunk,
          ),
          l1EarliestBlock = BlockParameter.Tag.EARLIEST,
          l1HighestBlock = config.l1HighestBlockTag,
          ftxQueue = ftxQueue,
        )
        ftxSender.start().thenCompose { ftxFetcher.start() }
      }.thenApply {
        log.debug("ForcedTransactionsApp started successfully")
      }
  }

  override fun stop(): CompletableFuture<Unit> {
    val stopFtxSender = if (this::ftxSender.isInitialized) ftxSender.stop() else CompletableFuture.completedFuture(Unit)
    val stopFtxFetcher =
      if (this::ftxFetcher.isInitialized) ftxFetcher.stop() else CompletableFuture.completedFuture(Unit)
    return SafeFuture.allOf(
      stopFtxSender,
      stopFtxFetcher,
    ).thenApply { }
  }
}
