package linea.ethapi

import io.vertx.core.Vertx
import kotlinx.datetime.Clock
import linea.EthLogsSearcher
import linea.SearchDirection
import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.CommonDomainFunctions
import linea.domain.EthLog
import linea.ethapi.cursor.BinarySearchCursor
import linea.ethapi.cursor.ConsecutiveSearchCursor
import net.consensys.linea.async.AsyncRetryer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

private sealed interface SearchResult {
  data class ItemFound(val log: EthLog) : SearchResult
  data class KeepSearching(val direction: SearchDirection) : SearchResult
  data object NoResultsInInterval : SearchResult
}

class EthLogsSearcherImpl(
  val vertx: Vertx,
  val ethApiClient: EthApiClient,
  val config: Config = Config(),
  val clock: Clock = Clock.System,
  val log: Logger = LogManager.getLogger(EthLogsSearcherImpl::class.java)
) : EthLogsSearcher, EthLogsClient by ethApiClient {
  data class Config(
    val loopSuccessBackoffDelay: Duration = 1.milliseconds
  )

  override fun findLog(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    chunkSize: Int,
    address: String,
    topics: List<String>,
    shallContinueToSearch: (EthLog) -> SearchDirection?
  ): SafeFuture<EthLog?> {
    require(chunkSize > 0) { "chunkSize=$chunkSize must be greater than 0" }

    return getAbsoluteBlockNumbers(fromBlock, toBlock)
      .thenCompose { (start, end) ->
        if (start > end) {
          // this is to prevent edge case when fromBlock number is after toBlock=LATEST/FINALIZED
          SafeFuture.failedFuture(
            IllegalStateException("invalid range: fromBlock=$fromBlock is after toBlock=$toBlock ($end)")
          )
        } else {
          findLogWithBinarySearch(
            fromBlock = start,
            toBlock = end,
            chunkSize = chunkSize,
            address = address,
            topics = topics,
            shallContinueToSearchPredicate = shallContinueToSearch
          )
        }
      }
  }

  override fun getLogsRollingForward(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>,
    chunkSize: UInt,
    searchTimeout: Duration,
    stopAfterTargetLogsCount: UInt?
  ): SafeFuture<EthLogsSearcher.LogSearchResult> {
    require(chunkSize > 0u) { "chunkSize=$chunkSize must be greater than 0" }

    return getAbsoluteBlockNumbers(fromBlock, toBlock)
      .thenCompose { (start, end) ->
        if (start > end) {
          // this is to prevent edge case when fromBlock number is after toBlock=LATEST/FINALIZED
          SafeFuture.failedFuture(
            IllegalStateException("invalid range: fromBlock=$fromBlock is after toBlock=$toBlock ($end)")
          )
        } else {
          getLogsLoopingForward(
            fromBlock = start,
            toBlock = end,
            address = address,
            topics = topics,
            chunkSize = chunkSize,
            searchTimeout = searchTimeout,
            logsSoftLimit = stopAfterTargetLogsCount
          )
        }
      }
  }

  private fun getLogsLoopingForward(
    fromBlock: ULong,
    toBlock: ULong,
    address: String,
    topics: List<String?>,
    chunkSize: UInt,
    searchTimeout: Duration,
    logsSoftLimit: UInt?
  ): SafeFuture<EthLogsSearcher.LogSearchResult> {
    val cursor = ConsecutiveSearchCursor(fromBlock, toBlock, chunkSize.toInt(), SearchDirection.FORWARD)

    val logsCollected: MutableList<EthLog> = CopyOnWriteArrayList()
    val startTime = clock.now()
    val lastSearchedChunk = AtomicReference<ULongRange>(null)

    return AsyncRetryer.retry(
      vertx,
      backoffDelay = config.loopSuccessBackoffDelay,
      stopRetriesPredicate = {
        val enoughLogsCollected = logsCollected.size >= (logsSoftLimit?.toInt() ?: Int.MAX_VALUE)
        val collectionTimeoutElapsed = (clock.now() - startTime) >= searchTimeout
        val noMoreChunksToCollect = !cursor.hasNext()

        enoughLogsCollected || collectionTimeoutElapsed || noMoreChunksToCollect
      }
    ) {
      val chunk = cursor.next()
      val chunkInterval = CommonDomainFunctions.blockIntervalString(chunk.start, chunk.endInclusive)

      log.trace("searching in chunk={}", chunkInterval)

      getLogs(chunk.start.toBlockParameter(), chunk.endInclusive.toBlockParameter(), address, topics)
        .thenPeek { result ->
          lastSearchedChunk.set(chunk)
          logsCollected.addAll(result)
          log.trace(
            "logs collected: chunk={} logsCount={}",
            chunkInterval,
            result.size
          )
        }
    }
      .thenApply {
        val endBlockNumber = lastSearchedChunk.get().endInclusive
        val logs = logsCollected.toList()
        log.debug(
          "getLogsRollingForward: fromBlock={} toBlock={} effectiveEndBlock={} address={} topics={} logsCount={}",
          fromBlock,
          toBlock,
          endBlockNumber,
          address,
          topics.joinToString(", ") { it ?: "null" },
          logs.size
        )
        EthLogsSearcher.LogSearchResult(
          logs = logs,
          startBlockNumber = fromBlock,
          endBlockNumber = endBlockNumber
        )
      }
  }

  private fun findLogWithBinarySearch(
    fromBlock: ULong,
    toBlock: ULong,
    chunkSize: Int,
    address: String,
    topics: List<String>,
    shallContinueToSearchPredicate: (EthLog) -> SearchDirection?
  ): SafeFuture<EthLog?> {
    val cursor = BinarySearchCursor(fromBlock, toBlock, chunkSize)
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
    shallContinueToSearchPredicate: (EthLog) -> SearchDirection?
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
      ethApiClient.getBlockByNumberWithoutTransactionsData(blockParameter)
        .thenApply { block ->
          if (block == null) {
            throw IllegalStateException("block not found for blockParameter=$blockParameter")
          }
          block.number
        }
    }
  }
}
