package linea.test

import io.vertx.core.Vertx
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.CommonDomainFunctions
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

class FetchAndValidationRunner(
  val vertx: Vertx = Vertx.vertx(),
  val rpcUrl: String,
  val log: Logger = LogManager.getLogger(FetchAndValidationRunner::class.java)
) {
  val web3j: Web3j = createWeb3jHttpClient(
    rpcUrl = rpcUrl,
//    executorService = vertx.nettyEventLoopGroup(),
    log = LogManager.getLogger("test.client.web3j"),
    requestResponseLogLevel = Level.DEBUG,
    failuresLogLevel = Level.ERROR
  )
  val validator = BlockEncodingValidator(vertx = vertx, log = log).also { it.start() }
  val blocksFetcher = BlocksFetcher(web3j, log = log)
  val targetEndBlockNumber = AtomicReference<ULong?>()

  fun awaitValidationFinishes(): SafeFuture<Unit> {
    val result = SafeFuture<Unit>()
    vertx.setPeriodic(2000) { timerId ->
      if (targetEndBlockNumber.get() != null &&
        validator.highestValidatedBlockNumber.get() >= targetEndBlockNumber.get()!!
      ) {
        vertx.cancelTimer(timerId)
        validator.stop()
        web3j.shutdown()
        result.complete(Unit)
      }
    }
    return result
  }

  fun fetchAndValidateBlocks(
    startBlockNumber: ULong,
    endBlockNumber: ULong? = null,
    chuckSize: UInt = 100U,
    rlpEncodingDecodingOnly: Boolean = false
  ): SafeFuture<*> {
    targetEndBlockNumber.set(endBlockNumber)
    return blocksFetcher.consumeBlocks(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      chunkSize = chuckSize
    ) { blocks ->
      log.info(
        "got blocks: {}",
        CommonDomainFunctions.blockIntervalString(blocks.first().number, blocks.last().number)
      )
      if (rlpEncodingDecodingOnly) {
        validator.validateRlpEncodingDecoding(blocks)
      } else {
        validator.validateCompression(blocks)
      }
    }
  }
}
