package linea.web3j.ethapi

import io.vertx.core.Vertx
import linea.domain.RetryConfig
import linea.ethapi.EthApiClient
import linea.kotlin.decodeHex
import linea.web3j.EthBlockExtended
import linea.web3j.createWeb3jHttpClient
import linea.web3j.createWeb3jHttpService
import linea.web3j.getWeb3jService
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.datatypes.Hash
import org.web3j.protocol.Web3j
import org.web3j.protocol.Web3jService
import org.web3j.utils.Async
import java.math.BigInteger
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
  web3jService: Web3jService = web3jClient.getWeb3jService(),
  requestRetryConfig: RetryConfig? = null,
  vertx: Vertx? = null,
  stopRetriesOnErrorPredicate: Predicate<Throwable> = Predicate { _ -> false },
  blockValidator: (EthBlockExtended.Block) -> Unit = {},
): EthApiClient {
  if (requestRetryConfig?.isRetryEnabled == true && vertx == null) {
    throw IllegalArgumentException("Vertx instance is required when request retry is enabled")
  }

  val ethApiClient = Web3jEthApiClient(web3jClient, web3jService, blockValidator = blockValidator)
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
  blockValidator: (EthBlockExtended.Block) -> Unit = {},
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

  return createEthApiClient(
    web3jClient,
    web3jService,
    requestRetryConfig,
    vertx,
    stopRetriesOnErrorPredicate,
    blockValidator,
  )
}

/** Validate L2 policy before dropping unsupported Ethereum fields from the shared block model. */
fun validateLinethBlock(block: EthBlockExtended.Block) {
  require(block.withdrawals.isNullOrEmpty()) { "Withdrawals are not supported: block=${block.number}" }
  require(
    block.withdrawalsRoot == null ||
      block.withdrawalsRoot.decodeHex().contentEquals(Hash.EMPTY_TRIE_HASH.bytes.toArray()),
  ) {
    "Nonempty withdrawals root: block=${block.number}"
  }
  require(
    block.requestsHash == null ||
      block.requestsHash!!.decodeHex().contentEquals(Hash.EMPTY_REQUESTS_HASH.bytes.toArray()),
  ) {
    "Execution requests are not supported: block=${block.number}"
  }
  require(block.blobGasUsed == BigInteger.ZERO && block.excessBlobGas == BigInteger.ZERO) {
    "Blob gas is not supported: block=${block.number}"
  }
  require(
    block.parentBeaconBlockRoot == null ||
      block.parentBeaconBlockRoot.decodeHex().contentEquals(ByteArray(32)),
  ) {
    "Nonzero parent beacon block root: block=${block.number}"
  }
  if (block.slotNumber != null) {
    require(
      block.withdrawalsRoot != null && block.requestsHash != null &&
        block.parentBeaconBlockRoot != null && block.blobGasUsedRaw != "0" && block.excessBlobGasRaw != "0",
    ) {
      "Missing Amsterdam header fields: block=${block.number}"
    }
  }
}
