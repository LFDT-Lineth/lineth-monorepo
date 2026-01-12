package linea.test

import io.vertx.core.Vertx
import linea.domain.CommonDomainFunctions
import linea.ethapi.EthApiClient
import linea.web3j.ethapi.createEthApiClient
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

class FetchAndValidationRunner(
  val vertx: Vertx = Vertx.vertx(),
  val rpcUrl: String,
  val log: Logger = LogManager.getLogger(FetchAndValidationRunner::class.java),
) {
  val ethApiClient: EthApiClient =
    createEthApiClient(
      rpcUrl = rpcUrl,
//    executorService = vertx.nettyEventLoopGroup(),
      log = LogManager.getLogger("test.client.eth-api"),
      requestResponseLogLevel = Level.DEBUG,
      failuresLogLevel = Level.ERROR,
    )
  val validator = BlockEncodingValidator(vertx = vertx, log = log).also { it.start() }
  val blocksFetcher = BlocksFetcher(ethApiClient, log = log)
  val targetEndBlockNumber = AtomicReference<ULong?>()

  fun awaitValidationFinishes(): SafeFuture<Unit> {
    val result = SafeFuture<Unit>()
    vertx.setPeriodic(2000) { timerId ->
      if (targetEndBlockNumber.get() != null &&
        validator.highestValidatedBlockNumber.get() >= targetEndBlockNumber.get()!!
      ) {
        vertx.cancelTimer(timerId)
        validator.stop()
        result.complete(Unit)
      }
    }
    return result
  }

  fun fetchAndValidateBlocks(
    startBlockNumber: ULong,
    endBlockNumber: ULong? = null,
    chuckSize: UInt = 100U,
    rlpEncodingDecodingOnly: Boolean = false,
  ): SafeFuture<*> {
    targetEndBlockNumber.set(endBlockNumber)
    return blocksFetcher.consumeBlocks(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      chunkSize = chuckSize,
    ) { blocks ->
      log.info(
        "got blocks: {}",
        CommonDomainFunctions.blockIntervalString(blocks.first().number, blocks.last().number),
      )
      if (rlpEncodingDecodingOnly) {
        validator.validateRlpEncodingDecoding(blocks)
      } else {
        validator.validateCompression(blocks)
      }
    }
  }
}
