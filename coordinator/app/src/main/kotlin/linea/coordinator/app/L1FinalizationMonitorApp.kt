package linea.coordinator.app

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.contract.l1.FinalizedStateDataProvider
import linea.coordination.HighestULongTracker
import linea.coordination.blockcreation.ForkChoiceUpdaterImpl
import linea.coordinator.clients.ShomeiClient
import linea.coordinator.config.toJsonRpcRetry
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.Type2StateProofManagerConfig
import linea.coordinator.config.v2.isDisabled
import linea.domain.BlockNumberAndHash
import linea.ethapi.EthApiClient
import linea.finalization.FinalizationHandler
import linea.finalization.FinalizationMonitor
import linea.finalization.FinalizationMonitorImpl
import linea.metrics.LineaMetricsCategory
import linea.persistence.AggregationsRepository
import linea.persistence.BatchesRepository
import linea.persistence.BlobsRepository
import linea.persistence.ForcedTransactionsDao
import linea.persistence.conflation.RecordsCleanupFinalizationHandler
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture

/**
 * Responsibilities:
 *   - poll L1 for finalization updates and dispatch them to:
 *     - the conflation app's last proven block number provider
 *     - the finalized records DB cleanup
 *     - the highest accepted finalization metric
 *   - notify type 2 state proof providers (Shomei frontend) of L1 finalizations
 */
class L1FinalizationMonitorApp(
  private val configs: CoordinatorConfig,
  private val vertx: Vertx,
  private val httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
  private val finalizedStateDataProvider: FinalizedStateDataProvider,
  private val lastFinalizedBlock: ULong,
  private val batchesRepository: BatchesRepository,
  private val blobsRepository: BlobsRepository,
  private val aggregationsRepository: AggregationsRepository,
  private val forcedTransactionsDao: ForcedTransactionsDao,
  private val metricsFacade: MetricsFacade,
  private val l1FinalizationUpdateHandler: (blockNumber: Long) -> SafeFuture<Unit>,
  private val l2EthApiClient: EthApiClient = createEthApiClient(
    rpcUrl = configs.l1FinalizationMonitor.l2Endpoint.toString(),
    log = LogManager.getLogger("clients.l2.eth.finalization-monitor"),
    requestRetryConfig = configs.l1FinalizationMonitor.l2RequestRetries,
    vertx = vertx,
  ),
) : LongRunningService {
  private val log = LogManager.getLogger(this::class.java)
  private val l1FinalizationMonitor =
    FinalizationMonitorImpl(
      config =
      FinalizationMonitorImpl.Config(
        pollingInterval = configs.l1FinalizationMonitor.l1PollingInterval,
        l1QueryBlockTag = configs.l1FinalizationMonitor.l1QueryBlockTag,
      ),
      finalizedStateDataProvider = finalizedStateDataProvider,
      l2EthApiClient = l2EthApiClient,
      vertx = vertx,
    )
  private val highestAcceptedFinalizationTracker = HighestULongTracker(lastFinalizedBlock).also {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "highest.accepted.block.number",
      description = "Highest finalized accepted end block number",
      measurementSupplier = it,
    )
  }
  private val shomeiFrontendFinalizationMonitor: LongRunningService = setupL1FinalizationMonitorForShomeiFrontend(
    type2StateProofProviderConfig = configs.type2StateProofProvider,
    httpJsonRpcClientFactory = httpJsonRpcClientFactory,
    finalizedStateDataProvider = finalizedStateDataProvider,
    l2EthApiClient = l2EthApiClient,
    vertx = vertx,
  )

  init {
    mapOf(
      "last_proven_block_provider" to FinalizationHandler { update: FinalizationMonitor.FinalizationUpdate ->
        l1FinalizationUpdateHandler(update.blockNumber.toLong())
      },
      "finalized records cleanup" to RecordsCleanupFinalizationHandler(
        batchesRepository = batchesRepository,
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
        forcedTransactionsDao = forcedTransactionsDao,
      ),
      "highest_accepted_finalization_on_l1" to FinalizationHandler { update: FinalizationMonitor.FinalizationUpdate ->
        highestAcceptedFinalizationTracker(update.blockNumber)
      },
    )
      .forEach { (handlerName, handler) ->
        l1FinalizationMonitor.addFinalizationHandler(handlerName, handler)
      }
  }

  override fun start(): CompletableFuture<Unit> {
    return SafeFuture.allOf(
      l1FinalizationMonitor.start(),
      shomeiFrontendFinalizationMonitor.start(),
    ).thenApply {
      log.info("L1FinalizationMonitorApp started")
    }
  }

  override fun stop(): CompletableFuture<Unit> {
    return SafeFuture.allOf(
      l1FinalizationMonitor.stop(),
      shomeiFrontendFinalizationMonitor.stop(),
    )
      .thenApply { log.info("L1FinalizationMonitorApp Stopped") }
  }

  companion object {
    private fun setupL1FinalizationMonitorForShomeiFrontend(
      type2StateProofProviderConfig: Type2StateProofManagerConfig,
      httpJsonRpcClientFactory: VertxHttpJsonRpcClientFactory,
      finalizedStateDataProvider: FinalizedStateDataProvider,
      l2EthApiClient: EthApiClient,
      vertx: Vertx,
    ): LongRunningService {
      if (type2StateProofProviderConfig.isDisabled()) {
        return DisabledLongRunningService
      }

      val finalizedBlockNotifier = run {
        val log = LogManager.getLogger("clients.ForkChoiceUpdaterShomeiClient")
        val type2StateProofProviderClients = type2StateProofProviderConfig.endpoints.map {
          ShomeiClient(
            vertx = vertx,
            rpcClient = httpJsonRpcClientFactory.create(it, log = log),
            retryConfig = type2StateProofProviderConfig.requestRetries.toJsonRpcRetry(),
            log = log,
          )
        }

        ForkChoiceUpdaterImpl(type2StateProofProviderClients)
      }

      val l1FinalizationMonitor =
        FinalizationMonitorImpl(
          config =
          FinalizationMonitorImpl.Config(
            pollingInterval = type2StateProofProviderConfig.l1PollingInterval,
            l1QueryBlockTag = type2StateProofProviderConfig.l1QueryBlockTag,
          ),
          finalizedStateDataProvider = finalizedStateDataProvider,
          l2EthApiClient = l2EthApiClient,
          vertx = vertx,
        )

      l1FinalizationMonitor.addFinalizationHandler("type 2 state proof provider finalization updates", {
        finalizedBlockNotifier.updateFinalizedBlock(
          BlockNumberAndHash(it.blockNumber, it.blockHash.toArray()),
        )
      })

      return l1FinalizationMonitor
    }
  }
}
