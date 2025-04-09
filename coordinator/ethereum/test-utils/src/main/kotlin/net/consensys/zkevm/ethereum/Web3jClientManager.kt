package net.consensys.zkevm.ethereum

import linea.web3j.okhttp.okHttpClientBuilder
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.http.HttpService
import org.web3j.utils.Async

/**
 * Helper Object to create Web3j clients for L1 and L2
 * that allow overriding default log values for testing and debugging easily
 */
object Web3jClientManager {
  val l1RpcUrl: String = System.getProperty("L1_RPC", "http://localhost:8445")
  val l2RpcUrl: String = System.getProperty("L2_RPC", "http://localhost:8545")
  val l1Client: Web3j = buildL1Client()
  val l2Client: Web3j = buildL2Client()

  fun buildL1Client(
    rpcUrl: String = l1RpcUrl,
    log: Logger = LogManager.getLogger("test.clients.l1.web3j-default"),
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG
  ): Web3j {
    return buildWeb3Client(rpcUrl, log, requestResponseLogLevel, failuresLogLevel)
  }

  fun buildL2Client(
    rpcUrl: String = l2RpcUrl,
    log: Logger = LogManager.getLogger("test.clients.l2.web3j-default"),
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG
  ): Web3j {
    return buildWeb3Client(rpcUrl, log, requestResponseLogLevel, failuresLogLevel)
  }

  fun buildWeb3Client(
    rpcUrl: String,
    log: Logger = LogManager.getLogger("test.clients.web3j"),
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
      500,
      Async.defaultExecutorService()
    )
  }
}
