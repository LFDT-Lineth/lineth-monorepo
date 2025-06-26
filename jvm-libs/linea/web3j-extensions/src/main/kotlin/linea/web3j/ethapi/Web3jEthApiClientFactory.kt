package linea.web3j.ethapi

import io.vertx.core.Vertx
import linea.domain.RetryConfig
import linea.ethapi.EthApiClient
import linea.web3j.createWeb3jHttpClient
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
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
  requestRetryConfig: RetryConfig?,
  vertx: Vertx?,
  stopRetriesOnErrorPredicate: Predicate<Throwable> = Predicate { _ -> false },
): EthApiClient {
  if (requestRetryConfig?.isRetryEnabled == true && vertx == null) {
    throw IllegalArgumentException("Vertx instance is required when request retry is enabled")
  }

  val ethApiClient = Web3jEthApiClient(web3jClient)
  return if (requestRetryConfig?.isRetryEnabled == true) {
    Web3jEthApiClientWithRetries(
      vertx = vertx!!,
      ethApiClient = ethApiClient,
      requestRetryConfig = requestRetryConfig,
      stopRetriesOnErrorPredicate = stopRetriesOnErrorPredicate,
    )
  } else {
    Web3jEthApiClient(web3jClient)
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
  log: Logger = org.apache.logging.log4j.LogManager.getLogger(Web3j::class.java),
  pollingInterval: Duration = 500.milliseconds,
  executorService: ScheduledExecutorService = Async.defaultExecutorService(),
  requestResponseLogLevel: Level = Level.TRACE,
  failuresLogLevel: Level = Level.DEBUG,
  requestRetryConfig: RetryConfig?,
  vertx: Vertx?,
  stopRetriesOnErrorPredicate: Predicate<Throwable> = Predicate { _ -> false },
): EthApiClient {
  val web3jClient =
    createWeb3jHttpClient(
      rpcUrl,
      log,
      pollingInterval,
      executorService,
      requestResponseLogLevel,
      failuresLogLevel,
    )

  return createEthApiClient(web3jClient, requestRetryConfig, vertx, stopRetriesOnErrorPredicate)
}
