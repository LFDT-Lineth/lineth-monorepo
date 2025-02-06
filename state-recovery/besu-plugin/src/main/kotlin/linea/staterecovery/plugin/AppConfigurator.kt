package linea.staterecovery.plugin

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import build.linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.micrometer.backends.BackendRegistries
import linea.staterecovery.BlockHeaderStaticFields
import linea.staterecovery.ExecutionLayerClient
import linea.staterecovery.StateRecoveryApp
import linea.staterecovery.TransactionDetailsClient
import linea.staterecovery.clients.VertxTransactionDetailsClient
import linea.staterecovery.clients.blobscan.BlobScanClient
import linea.web3j.Web3JLogsSearcher
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.apache.logging.log4j.LogManager
import java.net.URI
import kotlin.time.Duration.Companion.seconds

fun createAppAllInProcess(
  vertx: Vertx = Vertx.vertx(),
  meterRegistry: MeterRegistry = BackendRegistries.getDefaultNow(),
  elClient: ExecutionLayerClient,
  stateManagerClientEndpoint: URI,
  l1RpcEndpoint: URI,
  blobScanEndpoint: URI,
  blockHeaderStaticFields: BlockHeaderStaticFields,
  appConfig: StateRecoveryApp.Config
): StateRecoveryApp {
  return createAppClients(
    vertx = vertx,
    meterRegistry = meterRegistry,
    stateManagerClientEndpoint = stateManagerClientEndpoint,
    l1RpcEndpoint = l1RpcEndpoint,
    blobScanEndpoint = blobScanEndpoint,
    appConfig = appConfig
  ).let { clients ->
    val app = StateRecoveryApp(
      vertx = vertx,
      lineaContractClient = clients.lineaContractClient,
      ethLogsSearcher = clients.ethLogsSearcher,
      blobFetcher = clients.blobScanClient,
      elClient = elClient,
      stateManagerClient = clients.stateManagerClient,
      transactionDetailsClient = clients.transactionDetailsClient,
      blockHeaderStaticFields = blockHeaderStaticFields,
      config = appConfig
    )
    app
  }
}

data class AppClients(
  val lineaContractClient: Web3JLineaRollupSmartContractClientReadOnly,
  val ethLogsSearcher: Web3JLogsSearcher,
  val blobScanClient: BlobScanClient,
  val stateManagerClient: StateManagerClientV1,
  val transactionDetailsClient: TransactionDetailsClient
)

fun createAppClients(
  vertx: Vertx = Vertx.vertx(),
  meterRegistry: MeterRegistry = BackendRegistries.getDefaultNow(),
  l1RpcEndpoint: URI,
  l1RpcRequestRetryConfig: RequestRetryConfig = RequestRetryConfig(backoffDelay = 1.seconds),
  blobScanEndpoint: URI,
  blobScanRequestRetryConfig: RequestRetryConfig = RequestRetryConfig(backoffDelay = 1.seconds),
  stateManagerClientEndpoint: URI,
  stateManagerRequestRetry: RequestRetryConfig = RequestRetryConfig(backoffDelay = 1.seconds),
  zkStateManagerVersion: String = "2.3.0",
  appConfig: StateRecoveryApp.Config
): AppClients {
  val lineaContractClient = Web3JLineaRollupSmartContractClientReadOnly(
    contractAddress = appConfig.smartContractAddress,
    web3j = createWeb3jHttpClient(
      rpcUrl = l1RpcEndpoint.toString(),
      log = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.smart-contract")
    )
  )
  val ethLogsSearcher = run {
    val log = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.logs-searcher")
    Web3JLogsSearcher(
      vertx = vertx,
      web3jClient = createWeb3jHttpClient(
        rpcUrl = l1RpcEndpoint.toString(),
        log = log
      ),
      log = log
    )
  }
  val blobScanClient = BlobScanClient.create(
    vertx = vertx,
    endpoint = blobScanEndpoint,
    requestRetryConfig = blobScanRequestRetryConfig,
    logger = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.blob-scan")
  )
  val jsonRpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, MicrometerMetricsFacade(meterRegistry))
  val stateManagerClient: StateManagerClientV1 = StateManagerV1JsonRpcClient.create(
    rpcClientFactory = jsonRpcClientFactory,
    endpoints = listOf(stateManagerClientEndpoint),
    maxInflightRequestsPerClient = 10u,
    requestRetry = stateManagerRequestRetry,
    zkStateManagerVersion = zkStateManagerVersion,
    logger = LogManager.getLogger("linea.plugin.staterecovery.clients.state-manager")
  )
  val transactionDetailsClient: TransactionDetailsClient = VertxTransactionDetailsClient.create(
    jsonRpcClientFactory = jsonRpcClientFactory,
    endpoint = l1RpcEndpoint,
    retryConfig = l1RpcRequestRetryConfig,
    logger = LogManager.getLogger("linea.plugin.staterecovery.clients.l1.transaction-details")
  )
  return AppClients(
    lineaContractClient = lineaContractClient,
    ethLogsSearcher = ethLogsSearcher,
    blobScanClient = blobScanClient,
    stateManagerClient = stateManagerClient,
    transactionDetailsClient = transactionDetailsClient
  )
}
