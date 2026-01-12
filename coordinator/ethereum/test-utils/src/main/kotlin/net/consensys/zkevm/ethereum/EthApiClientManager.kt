package net.consensys.zkevm.ethereum

import linea.ethapi.EthApiClient
import linea.web3j.ethapi.createEthApiClient
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.utils.Async
import kotlin.time.Duration.Companion.milliseconds

object EthApiClientManager {
  val l1RpcUrl: String = System.getProperty("L1_RPC", "http://localhost:8445")
  val l2RpcUrl: String = System.getProperty("L2_RPC", "http://localhost:8545")
  val l1Client: EthApiClient = buildL1Client()
  val l2Client: EthApiClient = buildL2Client()

  fun buildL1Client(
    rpcUrl: String = l1RpcUrl,
    log: Logger = LogManager.getLogger("test.clients.l1.eth-api-default"),
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG,
  ): EthApiClient {
    return buildEthApiClient(rpcUrl, log, requestResponseLogLevel, failuresLogLevel)
  }

  fun buildL2Client(
    rpcUrl: String = l2RpcUrl,
    log: Logger = LogManager.getLogger("test.clients.l2.eth-api-default"),
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG,
  ): EthApiClient {
    return buildEthApiClient(rpcUrl, log, requestResponseLogLevel, failuresLogLevel)
  }

  fun buildEthApiClient(
    rpcUrl: String,
    log: Logger = LogManager.getLogger("test.clients.eth-api"),
    requestResponseLogLevel: Level = Level.TRACE,
    failuresLogLevel: Level = Level.DEBUG,
  ): EthApiClient {
    return createEthApiClient(
      rpcUrl = rpcUrl,
      log = log,
      requestResponseLogLevel = requestResponseLogLevel,
      failuresLogLevel = failuresLogLevel,
      pollingInterval = 500.milliseconds,
      executorService = Async.defaultExecutorService(),
    )
  }
}
