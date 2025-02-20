package linea.test

import build.linea.web3j.domain.toWeb3j
import io.vertx.core.Vertx
import linea.domain.Block
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.web3j.toDomain
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicLong
import kotlin.time.Duration.Companion.milliseconds

class BlocksFetcher(
  val web3j: Web3j,
  val vertx: Vertx = Vertx.vertx(),
  val pollingChuckSize: UInt = 100U,
  val log: Logger = LogManager.getLogger(BlocksFetcher::class.java)
) {
  fun fetchBlocks(
    startBlockNumber: ULong,
    endBlockNumber: ULong
  ): SafeFuture<List<Block>> {
    return (startBlockNumber..endBlockNumber).toList()
      .map { blockNumber ->
        web3j.ethGetBlockByNumber(blockNumber.toBlockParameter().toWeb3j(), true)
          .sendAsync()
          .toSafeFuture()
          .thenApply {
            if (it.hasError()) {
              log.error("Error fetching block={} errorMessage={}", blockNumber, it.error.message)
            }
            runCatching {
              it.block.toDomain()
            }
              .getOrElse {
                log.error("Error parsing block=$blockNumber", it)
                null
              }
          }
      }
      .let { SafeFuture.collectAll(it.stream()) }
      .thenApply { blocks: List<Block?> ->
        blocks.filterNotNull().sortedBy { it.number }
      }
  }

  fun consumeBlocks(
    startBlockNumber: ULong,
    endBlockNumber: ULong? = null,
    chunkSize: UInt = pollingChuckSize,
    consumer: (List<Block>) -> SafeFuture<*>
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
      }
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
