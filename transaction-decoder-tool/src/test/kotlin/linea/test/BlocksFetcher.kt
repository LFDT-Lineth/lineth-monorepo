package linea.test

import io.vertx.core.Vertx
import linea.domain.Block
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.ethapi.EthApiBlockClient
import net.consensys.linea.async.AsyncRetryer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicLong
import kotlin.time.Duration.Companion.milliseconds

class BlocksFetcher(
  val ethApiBlockClient: EthApiBlockClient,
  val vertx: Vertx = Vertx.vertx(),
  val pollingChuckSize: UInt = 100U,
  val log: Logger = LogManager.getLogger(BlocksFetcher::class.java),
) {
  fun fetchBlocks(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
  ): SafeFuture<List<Block>> {
    return (startBlockNumber..endBlockNumber).toList()
      .map { blockNumber -> ethApiBlockClient.ethFindBlockByNumberFullTxs(blockNumber.toBlockParameter()) }
      .let { SafeFuture.collectAll(it.stream()) }
      .thenApply { blocks: List<Block?> ->
        blocks.filterNotNull().sortedBy { it.number }
      }
  }

  fun consumeBlocks(
    startBlockNumber: ULong,
    endBlockNumber: ULong? = null,
    chunkSize: UInt = pollingChuckSize,
    consumer: (List<Block>) -> SafeFuture<*>,
  ): SafeFuture<*> {
    val lastBlockFetched = AtomicLong(startBlockNumber.toLong() - 1)
    return AsyncRetryer.retry(
      vertx,
      backoffDelay = 1000.milliseconds,
      stopRetriesPredicate = {
        endBlockNumber?.let { lastBlockFetched.get().toULong() >= it } ?: false
      },
      stopRetriesOnErrorPredicate = {
        it is Exception
      },
    ) {
      val start = (lastBlockFetched.get() + 1).toULong()
      val end = (start + chunkSize - 1U).coerceAtMost(endBlockNumber ?: ULong.MAX_VALUE)
      fetchBlocks(start, end)
        .thenCompose { blocks ->
          lastBlockFetched.set(blocks.last().number.toLong())
          consumer(blocks)
        }
    }
  }
}
