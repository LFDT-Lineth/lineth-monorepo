package linea.web3j

import build.linea.web3j.domain.toDomain
import build.linea.web3j.domain.toWeb3j
import io.vertx.core.Vertx
import linea.EthLogsSearcher
import linea.SearchDirection
import linea.domain.RetryConfig
import net.consensys.linea.BlockParameter
import net.consensys.linea.BlockParameter.Companion.toBlockParameter
import net.consensys.linea.CommonDomainFunctions
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.async.toSafeFuture
import net.consensys.toULong
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.EthLog
import org.web3j.protocol.core.methods.response.Log
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

private sealed interface SearchResult {
  data class ItemFound(val log: build.linea.domain.EthLog) : SearchResult
  data class KeepSearching(val direction: SearchDirection) : SearchResult
  data object NoResultsInInterval : SearchResult
}

class Web3JLogsSearcher(
  val vertx: Vertx,
  val web3jClient: Web3j,
  val config: Config = Config(),
  val log: Logger = LogManager.getLogger(Web3JLogsSearcher::class.java)
) : EthLogsSearcher {
  data class Config(
    val loopSuccessBackoffDelay: Duration = 1.milliseconds,
    val requestRetryConfig: RetryConfig = RetryConfig()
  )

  override fun findLog(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    chunkSize: Int,
    address: String,
    topics: List<String>,
    shallContinueToSearch: (build.linea.domain.EthLog) -> SearchDirection?
  ): SafeFuture<build.linea.domain.EthLog?> {
    require(chunkSize > 0) { "chunkSize=$chunkSize must be greater than 0" }

    return getAbsoluteBlockNumbers(fromBlock, toBlock)
      .thenCompose { (start, end) ->
        if (start > end) {
          // this is to prevent edge case when fromBlock number is after toBlock=LATEST/FINALIZED
          SafeFuture.failedFuture(
            IllegalStateException("invalid range: fromBlock=$fromBlock is after toBlock=$toBlock ($end)")
          )
        } else {
          findLogLoop(
            start,
            end,
            chunkSize,
            address,
            topics,
            shallContinueToSearch
          )
        }
      }
  }

  private fun findLogLoop(
    fromBlock: ULong,
    toBlock: ULong,
    chunkSize: Int,
    address: String,
    topics: List<String>,
    shallContinueToSearchPredicate: (build.linea.domain.EthLog) -> SearchDirection?
  ): SafeFuture<build.linea.domain.EthLog?> {
    val cursor = SearchCursor(fromBlock, toBlock, chunkSize)
    log.trace("searching between blocks={}", CommonDomainFunctions.blockIntervalString(fromBlock, toBlock))

    val nextChunkToSearchRef: AtomicReference<Pair<ULong, ULong>?> =
      AtomicReference(cursor.next(searchDirection = SearchDirection.FORWARD))
    return AsyncRetryer.retry(
      vertx,
      backoffDelay = config.loopSuccessBackoffDelay,
      stopRetriesPredicate = {
        it is SearchResult.ItemFound || nextChunkToSearchRef.get() == null
      }
    ) {
      log.trace("searching in chunk={}", nextChunkToSearchRef.get())
      val (chunkStart, chunkEnd) = nextChunkToSearchRef.get()!!
      val chunkInterval = CommonDomainFunctions.blockIntervalString(chunkStart, chunkEnd)
      findLogInInterval(chunkStart, chunkEnd, address, topics, shallContinueToSearchPredicate)
        .thenPeek { result ->
          if (result is SearchResult.NoResultsInInterval) {
            nextChunkToSearchRef.set(cursor.next(searchDirection = null))
          } else if (result is SearchResult.KeepSearching) {
            // need to search in the same chunk
            nextChunkToSearchRef.set(cursor.next(searchDirection = result.direction))
          }
          log.trace(
            "search result chunk={} searchResult={} nextChunkToSearch={}",
            chunkInterval,
            result,
            nextChunkToSearchRef.get()
          )
        }
    }.thenApply { either ->
      when (either) {
        is SearchResult.ItemFound -> either.log
        else -> null
      }
    }
  }

  private fun findLogInInterval(
    fromBlock: ULong,
    toBlock: ULong,
    address: String,
    topics: List<String>,
    shallContinueToSearchPredicate: (build.linea.domain.EthLog) -> SearchDirection?
  ): SafeFuture<SearchResult> {
    return getLogs(
      fromBlock = fromBlock.toBlockParameter(),
      toBlock = toBlock.toBlockParameter(),
      address = address,
      topics = topics
    )
      .thenApply { logs ->
        if (logs.isEmpty()) {
          SearchResult.NoResultsInInterval
        } else {
          var nextSearchDirection: SearchDirection? = null
          val item = logs.find {
            nextSearchDirection = shallContinueToSearchPredicate(it)
            nextSearchDirection == null
          }
          if (item != null) {
            SearchResult.ItemFound(item)
          } else {
            SearchResult.KeepSearching(nextSearchDirection!!)
          }
        }
      }
  }

  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<build.linea.domain.EthLog>> {
    return if (config.requestRetryConfig.isRetryEnabled) {
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = config.requestRetryConfig.backoffDelay,
        timeout = config.requestRetryConfig.timeout,
        maxRetries = config.requestRetryConfig.maxRetries?.toInt()
      ) {
        getLogsInternal(fromBlock, toBlock, address, topics)
      }
    } else {
      getLogsInternal(fromBlock, toBlock, address, topics)
    }
  }

  private fun getLogsInternal(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<build.linea.domain.EthLog>> {
    val ethFilter = EthFilter(
      /*fromBlock*/ fromBlock.toWeb3j(),
      /*toBlock*/ toBlock.toWeb3j(),
      /*address*/ address
    ).apply {
      topics.forEach { addSingleTopic(it) }
    }

    return web3jClient
      .ethGetLogs(ethFilter)
      .sendAsync()
      .toSafeFuture()
      .thenCompose {
        if (it.hasError()) {
          SafeFuture.failedFuture(
            RuntimeException(
              "json-rpc error: code=${it.error.code} message=${it.error.message} " +
                "data=${it.error.data}"
            )
          )
        } else {
          val logs = if (it.logs != null) {
            @Suppress("UNCHECKED_CAST")
            (it.logs as List<EthLog.LogResult<Log>>)
              .map { logResult ->
                logResult.get().toDomain()
              }
          } else {
            emptyList()
          }

          SafeFuture.completedFuture(logs)
        }
      }
  }

  private fun getAbsoluteBlockNumbers(
    fromBlock: BlockParameter,
    toBlock: BlockParameter
  ): SafeFuture<Pair<ULong, ULong>> {
    return SafeFuture.collectAll(
      getBlockParameterNumber(fromBlock),
      getBlockParameterNumber(toBlock)
    ).thenApply { (start, end) ->
      start to end
    }
  }

  private fun getBlockParameterNumber(blockParameter: BlockParameter): SafeFuture<ULong> {
    return if (blockParameter is BlockParameter.BlockNumber) {
      SafeFuture.completedFuture(blockParameter.getNumber())
    } else if (blockParameter == BlockParameter.Tag.EARLIEST) {
      SafeFuture.completedFuture(0UL)
    } else {
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = config.requestRetryConfig.backoffDelay,
        stopRetriesPredicate = { response ->
          response?.block?.number != null
        },
        action = {
          web3jClient.ethGetBlockByNumber(blockParameter.toWeb3j(), false).sendAsync().toSafeFuture()
        }
      )
        .thenApply { response ->
          response.block.number.toULong()
        }
    }
  }
}
