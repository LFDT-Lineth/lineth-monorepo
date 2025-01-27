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
  val lineaContractClient = Web3JLineaRollupSmartContractClientReadOnly(
    contractAddress = appConfig.smartContractAddress,
    web3j = createWeb3jHttpClient(
      rpcUrl = l1RpcEndpoint.toString(),
      log = LogManager.getLogger("linea.plugin.staterecover.clients.l1.smart-contract")
    )
  )
  val ethLogsSearcher = run {
    val log = LogManager.getLogger("linea.plugin.staterecover.clients.l1.logs-searcher")
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
    requestRetryConfig = RequestRetryConfig(
      backoffDelay = 1.seconds
    ),
    logger = LogManager.getLogger("linea.plugin.staterecover.clients.l1.blob-scan")
  )
  val jsonRpcClientFactory = VertxHttpJsonRpcClientFactory(vertx, MicrometerMetricsFacade(meterRegistry))
  val stateManagerClient: StateManagerClientV1 = StateManagerV1JsonRpcClient.create(
    rpcClientFactory = jsonRpcClientFactory,
    endpoints = listOf(stateManagerClientEndpoint),
    maxInflightRequestsPerClient = 10u,
    requestRetry = RequestRetryConfig(
      backoffDelay = 1.seconds
    ),
    zkStateManagerVersion = "2.3.0",
    logger = LogManager.getLogger("linea.plugin.staterecover.clients.state-manager")
  )

  val transactionDetailsClient: TransactionDetailsClient = VertxTransactionDetailsClient.create(
    jsonRpcClientFactory = jsonRpcClientFactory,
    endpoint = l1RpcEndpoint,
    retryConfig = RequestRetryConfig(
      backoffDelay = 1.seconds
    ),
    logger = LogManager.getLogger("linea.plugin.staterecover.clients.l1.transaction-details")
  )

  val app = StateRecoveryApp(
    vertx = vertx,
    lineaContractClient = lineaContractClient,
    ethLogsSearcher = ethLogsSearcher,
    blobFetcher = blobScanClient,
    elClient = elClient,
    stateManagerClient = stateManagerClient,
    transactionDetailsClient = transactionDetailsClient,
    blockHeaderStaticFields = blockHeaderStaticFields,
    config = appConfig
  )

  return app
}
