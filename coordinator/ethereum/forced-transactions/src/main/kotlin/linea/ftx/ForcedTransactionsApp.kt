package linea.ftx

import build.linea.clients.StateManagerAccountProofClient
import build.linea.clients.StateManagerClientV1
import io.vertx.core.Vertx
import linea.DisabledService
import linea.LongRunningService
import linea.contract.Web3JContractVersionAwaiter
import linea.contract.events.ForcedTransactionAddedEvent
import linea.contract.l1.ContractVersionProvider
import linea.contract.l1.LineaRollupContractVersion
import linea.contract.l1.LineaRollupSmartContractClientReadOnlyFinalizedStateProvider
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.ethapi.EthLogsFilterSubscriptionFactoryPollingBased
import linea.forcedtx.ForcedTransactionInclusionStatus
import linea.forcedtx.ForcedTransactionsClient
import linea.ftx.conflation.AggregationCalculatorByForcedTransaction
import linea.ftx.conflation.ConflationCalculatorByForcedTransaction
import linea.ftx.conflation.ForcedTransactionConflationSafeBlockNumberProvider
import linea.ftx.conflation.ForcedTransactionsInvalidityProofService
import linea.ftx.conflation.ForcedTransactionsSafeBlockNumberManager
import linea.ftx.conflation.InvalidityProofAssembler
import linea.persistence.ftx.ForcedTransactionsDao
import net.consensys.zkevm.coordinator.clients.InvalidityProverClientV1
import net.consensys.zkevm.coordinator.clients.TracesConflationVirtualBlockClientV1
import net.consensys.zkevm.ethereum.coordination.aggregation.AggregationTriggerCalculatorByTargetBlockNumbers
import net.consensys.zkevm.ethereum.coordination.aggregation.SyncAggregationTriggerCalculator
import net.consensys.zkevm.ethereum.coordination.blockcreation.AlwaysSafeBlockNumberProvider
import net.consensys.zkevm.ethereum.coordination.blockcreation.ConflationSafeBlockNumberProvider
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByTargetBlockNumbers
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Queue
import java.util.concurrent.CompletableFuture
import java.util.concurrent.LinkedBlockingQueue
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

/**
 * Wraps a ForcedTransactionAddedEvent with the L1 block timestamp when it was added
 */
internal data class ForcedTransactionWithTimestamp(
  val event: ForcedTransactionAddedEvent,
  val l1BlockTimestamp: Instant,
)

interface ForcedTransactionsApp : LongRunningService {
  val conflationSafeBlockNumberProvider: ConflationSafeBlockNumberProvider
  val conflationCalculator: ConflationCalculator
  val aggregationCalculator: SyncAggregationTriggerCalculator

  data class Config(
    val l1PollingInterval: Duration = 12.seconds,
    val l1ContractAddress: String,
    val l1HighestBlockTag: BlockParameter,
    val l1EventSearchBlockChunk: UInt = 1000u,
    val ftxSequencerSendingInterval: Duration = 12.seconds,
    val maxFtxToSendToSequencer: UInt = 10u,
    val ftxProcessingDelay: Duration = Duration.ZERO,
    val invalidityProofProcessingInterval: Duration = 12.seconds,
  )

  companion object {
    fun createDisabled(): ForcedTransactionsApp = DisabledForcedTransactionsApp()
    fun create(
      config: Config,
      vertx: Vertx,
      l1EthApiClient: EthApiClient,
      l2EthApiClient: EthApiClient,
      finalizedStateProvider: LineaRollupSmartContractClientReadOnlyFinalizedStateProvider,
      contractVersionProvider: ContractVersionProvider<LineaRollupContractVersion>,
      ftxClient: ForcedTransactionsClient,
      ftxDao: ForcedTransactionsDao,
      invalidityProofClient: InvalidityProverClientV1,
      stateManagerClient: StateManagerClientV1,
      accountProofClient: StateManagerAccountProofClient,
      tracesClient: TracesConflationVirtualBlockClientV1,
      clock: Clock,
    ): ForcedTransactionsApp = ForcedTransactionsAppImpl(
      config = config,
      vertx = vertx,
      l1EthApiClient = l1EthApiClient,
      l2EthApiClient = l2EthApiClient,
      finalizedStateProvider = finalizedStateProvider,
      contractVersionProvider = contractVersionProvider,
      ftxClient = ftxClient,
      ftxDao = ftxDao,
      invalidityProofClient = invalidityProofClient,
      stateManagerClient = stateManagerClient,
      accountProofClient = accountProofClient,
      tracesClient = tracesClient,
      clock = clock,
    )
  }
}

internal class DisabledForcedTransactionsApp() : ForcedTransactionsApp,
  DisabledService("forced transactions") {
  private val safeBlockNumberProvider = AlwaysSafeBlockNumberProvider()
  override val conflationSafeBlockNumberProvider: ConflationSafeBlockNumberProvider = safeBlockNumberProvider
  override val conflationCalculator: ConflationCalculator = ConflationCalculatorByTargetBlockNumbers(emptySet())
  override val aggregationCalculator: SyncAggregationTriggerCalculator =
    AggregationTriggerCalculatorByTargetBlockNumbers(
      targetEndBlockNumbers = emptyList(),
    )
}

internal class ForcedTransactionsAppImpl(
  private val config: ForcedTransactionsApp.Config,
  private val vertx: Vertx,
  private val l1EthApiClient: EthApiClient,
  private val l2EthApiClient: EthApiClient,
  private val finalizedStateProvider: LineaRollupSmartContractClientReadOnlyFinalizedStateProvider,
  private val contractVersionProvider: ContractVersionProvider<LineaRollupContractVersion>,
  private val ftxClient: ForcedTransactionsClient,
  private val ftxDao: ForcedTransactionsDao,
  private val invalidityProofClient: InvalidityProverClientV1,
  private val stateManagerClient: StateManagerClientV1,
  private val accountProofClient: StateManagerAccountProofClient,
  private val tracesClient: TracesConflationVirtualBlockClientV1,
  private val clock: Clock,
  safeBlockNumberProvider: ForcedTransactionConflationSafeBlockNumberProvider =
    ForcedTransactionConflationSafeBlockNumberProvider(),
) : ForcedTransactionsApp {
  private val log = LogManager.getLogger(ForcedTransactionsAppImpl::class.java)
  internal val ftxQueue: Queue<ForcedTransactionWithTimestamp> = LinkedBlockingQueue(10_000)
  internal val ftxProcessedQueue: Queue<ForcedTransactionInclusionStatus> = LinkedBlockingQueue(10_000)
  private val ftxResumePointProvider = ForcedTransactionsResumePointProviderImpl(
    finalizedStateProvider = finalizedStateProvider,
    l1HighestBlock = config.l1HighestBlockTag,
    ftxDao = ftxDao,
  )
  private lateinit var ftxStatusUpdater: ForcedTransactionsStatusUpdater
  private lateinit var ftxFetcher: ForcedTransactionsL1EventsFetcher
  private lateinit var ftxSender: ForcedTransactionsSenderForExecution
  private var safeBlockNumberManager = ForcedTransactionsSafeBlockNumberManager(listener = safeBlockNumberProvider)
  override val conflationSafeBlockNumberProvider: ConflationSafeBlockNumberProvider = safeBlockNumberProvider
  override val conflationCalculator: ConflationCalculator = ConflationCalculatorByForcedTransaction(
    processedFtxQueue = ftxProcessedQueue,
  )
  override val aggregationCalculator: SyncAggregationTriggerCalculator = AggregationCalculatorByForcedTransaction(
    processedFtxQueue = ftxProcessedQueue,
  )
  private val ftxInvalidityProofService = ForcedTransactionsInvalidityProofService(
    ftxDao = ftxDao,
    invalidityProofAssembler = InvalidityProofAssembler(
      invalidityProofClient = invalidityProofClient,
      stateManagerClient = stateManagerClient,
      accountProofClient = accountProofClient,
      ethApiLogsClient = l1EthApiClient,
      tracesClient = tracesClient,
      contractAddress = config.l1ContractAddress,
    ),
    vertx = vertx,
    pollingInterval = config.invalidityProofProcessingInterval,
  )

  override fun start(): CompletableFuture<Unit> {
    log.info("starting ForcedTransactionsApp")
    return contractVersionProvider
      .getVersion()
      .thenCompose { contractVersion ->
        if (contractVersion < LineaRollupContractVersion.V8) {
          log.info(
            "contractVersion={} is lower than required {} by forced transactions. " +
              "waiting until the contract is upgraded...",
            contractVersion,
            LineaRollupContractVersion.V8,
          )
          // can release the lock because users cannot send forced transactions until the contract is upgraded
          safeBlockNumberManager.forcedTransactionsUnsupportedYetByL1Contract()
          // wait until V8 is deployed to start
          Web3JContractVersionAwaiter(
            vertx = vertx,
            versionProvider = contractVersionProvider,
            log = log,
          ).awaitVersion(
            minTargetVersion = LineaRollupContractVersion.V8,
            highestBlockTag = config.l1HighestBlockTag,
          )
        } else {
          SafeFuture.completedFuture(contractVersion)
        }
      }
      .thenCompose { ftxResumePointProvider.getLastProcessedForcedTransaction() }
      .thenCompose { (lastProcessedForcedTransactionNumber, ftxRecord) ->
        // check the database for the highest simulatedExecutionBlockNumber of in-flight forced transactions (if any)
        // otherwise, lock safe block number to 0 until we fetch all events from L1 and determine the correct safe block number to conflate to
        if (ftxRecord != null) {
          safeBlockNumberManager.ftxProcessedBySequencer(ftxRecord.ftxNumber, ftxRecord.simulatedExecutionBlockNumber)
        }

        val lastProcessedFtxProvider = ForcedTransactionsResumePointProvider {
          SafeFuture.completedFuture(lastProcessedForcedTransactionNumber)
        }
        this.ftxStatusUpdater = ForcedTransactionsStatusUpdater(
          dao = this.ftxDao,
          ftxClient = this.ftxClient,
          ftxQueue = this.ftxQueue,
          ftxProcessedQueue = this.ftxProcessedQueue,
          lastProcessedFtxNumber = lastProcessedForcedTransactionNumber,
          safeBlockNumberManager = safeBlockNumberManager,
          ftxProcessingDelay = config.ftxProcessingDelay,
          clock = this.clock,
        )
        this.ftxSender = ForcedTransactionsSenderForExecution(
          vertx = vertx,
          ftxClient = this.ftxClient,
          pollingInterval = config.ftxSequencerSendingInterval,
          l2EthApi = l2EthApiClient,
          unprocessedFtxProvider = ftxStatusUpdater::getUnprocessedForcedTransactions,
          txLimitToSendPerTick = config.maxFtxToSendToSequencer.toInt(),
          safeBlockNumberManager = safeBlockNumberManager,
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
          safeBlockNumberManager = safeBlockNumberManager,
          l1EarliestBlock = BlockParameter.Tag.EARLIEST,
          l1HighestBlock = config.l1HighestBlockTag,
          ftxQueue = ftxQueue,
        )

        SafeFuture.allOf(
          ftxSender.start(),
          ftxFetcher.start(),
          ftxInvalidityProofService.start(),
        )
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
