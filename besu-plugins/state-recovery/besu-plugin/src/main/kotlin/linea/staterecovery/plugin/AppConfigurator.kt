package linea.staterecovery.plugin

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.micrometer.backends.BackendRegistries
import linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import linea.domain.RetryConfig
import linea.ethapi.EthLogsSearcherImpl
import linea.staterecovery.BlockHeaderStaticFields
import linea.staterecovery.ExecutionLayerClient
import linea.staterecovery.StateRecoveryApp
import linea.staterecovery.TransactionDetailsClient
import linea.staterecovery.clients.VertxTransactionDetailsClient
import linea.staterecovery.clients.blobscan.BlobScanClient
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.apache.logging.log4j.LogManager
import java.net.URI
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

fun createAppAllInProcess(
  vertx: Vertx = Vertx.vertx(),
  meterRegistry: MeterRegistry = BackendRegistries.getDefaultNow(),
  elClient: ExecutionLayerClient,
  stateManagerClientEndpoint: URI,
  l1Endpoint: URI,
  l1SuccessBackoffDelay: Duration,
  l1RequestRetryConfig: RetryConfig,
  blobScanEndpoint: URI,
  blobScanRequestRetryConfig: RetryConfig,
  blobscanRequestRatelimitBackoffDelay: Duration?,
  blockHeaderStaticFields: BlockHeaderStaticFields,
  appConfig: StateRecoveryApp.Config,
): StateRecoveryApp {
  return createAppClients(
    vertx = vertx,
    meterRegistry = meterRegistry,
    stateManagerClientEndpoint = stateManagerClientEndpoint,
    smartContractAddress = appConfig.smartContractAddress,
    l1RpcEndpoint = l1Endpoint,
    l1SuccessBackoffDelay = l1SuccessBackoffDelay,
    l1RequestRetryConfig = l1RequestRetryConfig,
    blobScanEndpoint = blobScanEndpoint,
    blobScanRequestRetryConfig = blobScanRequestRetryConfig,
    blobscanRequestRateLimitBackoffDelay = blobscanRequestRatelimitBackoffDelay,
  ).let { clients ->
    val app =
      StateRecoveryApp(
        vertx = vertx,
        lineaContractClient = clients.lineaContractClient,
        ethLogsSearcher = clients.ethLogsSearcher,
        blobFetcher = clients.blobScanClient,
        elClient = elClient,
        stateManagerClient = clients.stateManagerClient,
        transactionDetailsClient = clients.transactionDetailsClient,
        blockHeaderStaticFields = blockHeaderStaticFields,
        config = appConfig,
      )
    app
  }
}

data class AppClients(
  val lineaContractClient: Web3JLineaRollupSmartContractClientReadOnly,
  val ethLogsSearcher: EthLogsSearcherImpl,
  val blobScanClient: BlobScanClient,
  val stateManagerClient: StateManagerClientV1,
  val transactionDetailsClient: TransactionDetailsClient,
)

fun RetryConfig.toRequestRetryConfig(): RequestRetryConfig {
  return RequestRetryConfig(
    maxRetries = this.maxRetries,
    timeout = this.timeout,
    backoffDelay = this.backoffDelay,
    failuresWarningThreshold = this.failuresWarningThreshold,
  )
}

fun createAppClients(
  vertx: Vertx = Vertx.vertx(),
  meterRegistry: MeterRegistry = BackendRegistries.getDefaultNow(),
  smartContractAddress: String,
  l1RpcEndpoint: URI,
  l1SuccessBackoffDelay: Duration = 1.milliseconds,
  l1RequestRetryConfig: RetryConfig = RetryConfig(backoffDelay = 1.seconds),
  blobScanEndpoint: URI,
  blobScanRequestRetryConfig: RetryConfig = RetryConfig(backoffDelay = 1.seconds),
  stateManagerClientEndpoint: URI,
  blobscanRequestRateLimitBackoffDelay: Duration? = null,
  stateManagerRequestRetry: RetryConfig = RetryConfig(backoffDelay = 1.seconds),
  zkStateManagerVersion: String = "2.3.0",
): AppClients {
  val lineaContractClient =
    Web3JLineaRollupSmartContractClientReadOnly(
      contractAddress = smartContractAddress,
      web3j =
      createWeb3jHttpClient(
        rpcUrl = l1RpcEndpoint.toString(),
        log = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.smart-contract"),
      ),
    )
  val ethLogsSearcher =
    run {
      val log = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.logs-searcher")
      val web3jEthApiClient =
        createEthApiClient(
          vertx = vertx,
          rpcUrl = l1RpcEndpoint.toString(),
          requestRetryConfig = l1RequestRetryConfig,
          log = log,
        )
      EthLogsSearcherImpl(
        vertx = vertx,
        ethApiClient = web3jEthApiClient,
        config =
        EthLogsSearcherImpl.Config(
          loopSuccessBackoffDelay = l1SuccessBackoffDelay,
        ),
        log = log,
      )
    }
  val blobScanClient =
    BlobScanClient.create(
      vertx = vertx,
      endpoint = blobScanEndpoint,
      requestRetryConfig = blobScanRequestRetryConfig,
      logger = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.blob-scan"),
      rateLimitBackoffDelay = blobscanRequestRateLimitBackoffDelay,
    )
  val jsonRpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, MicrometerMetricsFacade(meterRegistry))
  val stateManagerClient: StateManagerClientV1 =
    StateManagerV1JsonRpcClient.create(
      rpcClientFactory = jsonRpcClientFactory,
      endpoints = listOf(stateManagerClientEndpoint),
      maxInflightRequestsPerClient = 10u,
      requestRetry = stateManagerRequestRetry.toRequestRetryConfig(),
      zkStateManagerVersion = zkStateManagerVersion,
      logger = LogManager.getLogger("linea.plugin.staterecovery.clients.state-manager"),
    )
  val transactionDetailsClient: TransactionDetailsClient =
    VertxTransactionDetailsClient.create(
      jsonRpcClientFactory = jsonRpcClientFactory,
      endpoint = l1RpcEndpoint,
      retryConfig = l1RequestRetryConfig.toRequestRetryConfig(),
      logger = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.transaction-details"),
    )
  return AppClients(
    lineaContractClient = lineaContractClient,
    ethLogsSearcher = ethLogsSearcher,
    blobScanClient = blobScanClient,
    stateManagerClient = stateManagerClient,
    transactionDetailsClient = transactionDetailsClient,
  )
}
