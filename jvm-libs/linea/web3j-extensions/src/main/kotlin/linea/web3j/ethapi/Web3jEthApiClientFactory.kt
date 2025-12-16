package linea.web3j.ethapi

import io.vertx.core.Vertx
import linea.domain.RetryConfig
import linea.ethapi.EthApiClient
import linea.web3j.createWeb3jHttpClient
import linea.web3j.createWeb3jHttpService
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.Web3jService
import org.web3j.utils.Async
import java.util.concurrent.ScheduledExecutorService
import java.util.function.Predicate
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

/**
 * Creates an instance of [EthApiClient] using the provided [Web3j] client.
 *
 * @param web3jClient The [Web3j] client to use for making requests.
 * @param requestRetryConfig The configuration for request retries (default is null, meaning no retries).
 * @param vertx The Vert.x instance to use for asynchronous operations (required if requestRetryConfig is not null and retry enabled).
 * @return An instance of [EthApiClient].
 */
fun createEthApiClient(
  web3jClient: Web3j,
  web3jService: Web3jService,
  requestRetryConfig: RetryConfig? = null,
  vertx: Vertx? = null,
  stopRetriesOnErrorPredicate: Predicate<Throwable> = Predicate { _ -> false },
): EthApiClient {
  if (requestRetryConfig?.isRetryEnabled == true && vertx == null) {
    throw IllegalArgumentException("Vertx instance is required when request retry is enabled")
  }

  val ethApiClient = Web3jEthApiClient(web3jClient, web3jService)
  return if (requestRetryConfig?.isRetryEnabled == true) {
    Web3jEthApiClientWithRetries(
      vertx = vertx!!,
      ethApiClient = ethApiClient,
      requestRetryConfig = requestRetryConfig,
      stopRetriesOnErrorPredicate = stopRetriesOnErrorPredicate,
    )
  } else {
    ethApiClient
  }
}

/**
 * Creates an instance of [EthApiClient] using the provided parameters.
 *
 * @param rpcUrl The RPC URL to connect to.
 * @param log The logger to use for logging (default is a logger for Web3j).
 * @param pollingInterval The polling interval for the client (default is 500 milliseconds).
 * @param executorService The executor service to use for asynchronous operations (default is the default executor service).
 * @param requestResponseLogLevel The log level for request/response logging (default is TRACE).
 * @param failuresLogLevel The log level for failures logging (default is DEBUG).
 * @param requestRetryConfig The configuration for request retries. When null no retries.
 * @param vertx The Vert.x instance to use for asynchronous operations (required if requestRetryConfig is not null and retry enabled).
 * @return An instance of [EthApiClient].
 */
fun createEthApiClient(
  rpcUrl: String,
  log: Logger = LogManager.getLogger(Web3j::class.java),
  pollingInterval: Duration = 500.milliseconds,
  executorService: ScheduledExecutorService = Async.defaultExecutorService(),
  requestResponseLogLevel: Level = Level.TRACE,
  failuresLogLevel: Level = Level.DEBUG,
  requestRetryConfig: RetryConfig? = null,
  vertx: Vertx? = null,
  stopRetriesOnErrorPredicate: Predicate<Throwable> = Predicate { _ -> false },
): EthApiClient {
  val web3jService = createWeb3jHttpService(
    rpcUrl = rpcUrl,
    log = log,
    requestResponseLogLevel = requestResponseLogLevel,
    failuresLogLevel = failuresLogLevel,
  )

  val web3jClient =
    createWeb3jHttpClient(
      httpService = web3jService,
      pollingInterval = pollingInterval,
      executorService = executorService,
    )

  return createEthApiClient(web3jClient, web3jService, requestRetryConfig, vertx, stopRetriesOnErrorPredicate)
}
