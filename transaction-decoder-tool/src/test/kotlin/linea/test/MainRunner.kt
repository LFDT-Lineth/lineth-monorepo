package linea.test

import io.vertx.core.Vertx
import net.consensys.linea.async.get
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.core.config.Configurator
import java.util.concurrent.TimeUnit

fun configureLoggers(loggerConfigs: List<Pair<String, Level>>) {
  loggerConfigs.forEach { (loggerName, level) ->
    Configurator.setLevel(loggerName, level)
  }
}

fun main() {
  val rpcUrl = run {
    "https://linea-sepolia.infura.io/v3/${System.getenv("INFURA_PROJECT_ID")}"
//    "https://linea-mainnet.infura.io/v3/${System.getenv("INFURA_PROJECT_ID")}"
  }
  val vertx = Vertx.vertx()
  vertx.exceptionHandler { error ->
    println("Unhandled exception: message=${error.message}")
    LogManager.getLogger("vertx").error("Unhandled exception: message={}", error.message, error)
  }
  val fetcherAndValidate =
    FetchAndValidationRunner(
      rpcUrl = rpcUrl,
      vertx = vertx,
      log = LogManager.getLogger("test.validator"),
    )
  configureLoggers(
    listOf(
      "linea.rlp" to Level.TRACE,
      "test.client.web3j" to Level.TRACE,
      "test.validator" to Level.INFO,
    ),
  )

  // Sepolia Blocks
  val startBlockNumber = 7_236_338UL
//  val startBlockNumber = 5_099_599UL
  // Mainnet Blocks
//  val startBlockNumber = 10_000_308UL
  runCatching {
    fetcherAndValidate.fetchAndValidateBlocks(
      startBlockNumber = startBlockNumber,
      endBlockNumber = startBlockNumber + 1U,
//      endBlockNumber = startBlockNumber + 0u,
      chuckSize = 1_000U,
      rlpEncodingDecodingOnly = false,
    ).get(2, TimeUnit.MINUTES)
  }.onFailure { error ->
    fetcherAndValidate.log.error("Error fetching and validating blocks", error)
  }
  fetcherAndValidate.awaitValidationFinishes().get()
  println("waited validation finishes")
//  fetcherAndValidate.validator.stop()
  vertx.close().get()
  println("closed vertx")
}
