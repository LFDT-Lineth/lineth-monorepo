package linea.web3j

import io.vertx.core.Vertx
import net.consensys.linea.async.AsyncRetryer
import org.web3j.abi.EventEncoder
import org.web3j.abi.datatypes.Event
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.EthLog
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class Web3JLogsClient(
  val vertx: Vertx,
  val web3jClient: Web3j,
  val config: Config = Config()
) {
  data class Config(
    val timeout: Duration = 30.seconds,
    val backoffDelay: Duration = 100.milliseconds,
    val lookBackRange: Int = 100
  ) {
    init {
      require(lookBackRange > 0) { "lookBackRange must be greater than 0" }
    }
  }

  private val logsPoller = AsyncRetryer.retryer<List<Log>>(
    vertx,
    backoffDelay = config.backoffDelay,
    timeout = config.timeout
  )

  fun getLogs(
    ethFilter: EthFilter
  ): SafeFuture<List<Log>> {
    return SafeFuture
      .of(web3jClient.ethGetLogs(ethFilter).sendAsync())
      .thenCompose {
        if (it.hasError()) {
          SafeFuture.failedFuture<List<Log>>(
            RuntimeException(
              "json-rpc error: code=${it.error.code} message=${it.error.message} " +
                "data=${it.error.data}"
            )
          )
        } else {
          val logs = if (it.logs != null) {
            @Suppress("UNCHECKED_CAST")
            (it.logs as List<EthLog.LogResult<Log>>).map { logResult -> logResult.get() }
          } else {
            emptyList()
          }
          SafeFuture.completedFuture(logs)
        }
      }
  }

  fun findLastLog(
    upToBlockNumberInclusive: Long,
    address: String,
    lookbackBlockNumberLimitInclusive: Long = upToBlockNumberInclusive,
    eventsFilter: List<Event>
  ): SafeFuture<Log?> {
    require(lookbackBlockNumberLimitInclusive <= upToBlockNumberInclusive) {
      "lookBackBlockNumberLimit must be less than or equal to upToBlockNumberInclusive"
    }
    val topics = eventsFilter.map { EventEncoder.encode(it) }.toTypedArray()
    var toBlock = upToBlockNumberInclusive
    var fromBlock = (toBlock - config.lookBackRange).coerceAtLeast(lookbackBlockNumberLimitInclusive)

    var ethFilter =
      EthFilter(
        /*fromBlock*/ DefaultBlockParameter.valueOf(fromBlock.toBigInteger()),
        /*toBlock*/ DefaultBlockParameter.valueOf(upToBlockNumberInclusive.toBigInteger()),
        address
      ).apply {
        addOptionalTopics(*topics)
      }

    return (
      logsPoller.retry(
        stopRetriesPredicate = { logs ->
          if (logs.isNotEmpty() || fromBlock <= lookbackBlockNumberLimitInclusive) {
            true
          } else {
            toBlock = fromBlock - 1
            fromBlock = (toBlock - config.lookBackRange + 1).coerceAtLeast(lookbackBlockNumberLimitInclusive)
            ethFilter = EthFilter(
              /*fromBlock*/ DefaultBlockParameter.valueOf(fromBlock.toBigInteger()),
              /*toBlock*/ DefaultBlockParameter.valueOf(toBlock.toBigInteger()),
              address
            ).apply {
              addOptionalTopics(*topics)
            }
            false
          }
        }
      ) {
        getLogs(ethFilter)
      }
      )
      .thenApply { logs ->
        logs.maxByOrNull { it.blockNumber }
      }
  }
}
