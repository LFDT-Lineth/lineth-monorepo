package linea.web3j

import linea.web3j.okhttp.okHttpClientBuilder
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.JsonRpc2_0Web3j
import org.web3j.protocol.http.HttpService
import org.web3j.utils.Async
import java.util.concurrent.ScheduledExecutorService
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

internal fun Web3j.getWeb3jService(): Web3jService {
  return when (this) {
    is JsonRpc2_0Web3j -> {
      // Access the protected web3jService field using reflection
      val field = JsonRpc2_0Web3j::class.java.getDeclaredField("web3jService")
      field.isAccessible = true
      field.get(this) as Web3jService
    }
    else -> {
      throw IllegalStateException("Web3j of type ${this::class.simpleName} is not supported yet")
    }
  }
}

fun createWeb3jHttpClient(
  rpcUrl: String,
  log: Logger = org.apache.logging.log4j.LogManager.getLogger(Web3j::class.java),
  pollingInterval: Duration = 1.seconds,
  executorService: ScheduledExecutorService = Async.defaultExecutorService(),
  requestResponseLogLevel: Level = Level.TRACE,
  failuresLogLevel: Level = Level.DEBUG,
): Web3j {
  val httpService = createWeb3jHttpService(rpcUrl, log, requestResponseLogLevel, failuresLogLevel)
  return createWeb3jHttpClient(
    httpService,
    pollingInterval,
    executorService,
  )
}

fun createWeb3jHttpClient(
  httpService: HttpService,
  pollingInterval: Duration = 1.seconds,
  executorService: ScheduledExecutorService = Async.defaultExecutorService(),
): Web3j {
  return Web3j.build(
    /* web3jService = */
    httpService,
    // used for Web3jRx to poll for new Blocks and TransactionsReceipts
    /* pollingInterval = */
    pollingInterval.inWholeMilliseconds,
    /* scheduledExecutorService = */
    executorService,
  )
}

fun createWeb3jHttpService(
  rpcUrl: String,
  log: Logger = org.apache.logging.log4j.LogManager.getLogger(Web3j::class.java),
  requestResponseLogLevel: Level = Level.TRACE,
  failuresLogLevel: Level = Level.DEBUG,
): HttpService {
  return HttpService(
    /* url = */
    rpcUrl,
    /* httpClient = */
    okHttpClientBuilder(
      logger = log,
      requestResponseLogLevel = requestResponseLogLevel,
      failuresLogLevel = failuresLogLevel,
    ).build(),
  )
}
