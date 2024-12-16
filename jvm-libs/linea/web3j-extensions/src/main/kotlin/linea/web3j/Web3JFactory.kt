package linea.web3j

import net.consensys.linea.web3j.okHttpClientBuilder
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.http.HttpService
import org.web3j.utils.Async
import java.util.concurrent.ScheduledExecutorService
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

fun createWeb3jHttpClient(
  rpcUrl: String,
  log: Logger = org.apache.logging.log4j.LogManager.getLogger(Web3j::class.java),
  pollingInterval: Duration = 500.milliseconds,
  executorService: ScheduledExecutorService = Async.defaultExecutorService(),
  requestResponseLogLevel: Level = Level.TRACE,
  failuresLogLevel: Level = Level.DEBUG
): Web3j {
  return Web3j.build(
    HttpService(
      rpcUrl,
      okHttpClientBuilder(
        logger = log,
        requestResponseLogLevel = requestResponseLogLevel,
        failuresLogLevel = failuresLogLevel
      ).build()
    ),
    pollingInterval.inWholeMilliseconds,
    executorService
  )
}
