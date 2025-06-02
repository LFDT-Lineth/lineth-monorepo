package linea.staterecovery

import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerV1JsonRpcClient
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import kotlinx.datetime.Clock
import linea.domain.EthLogEvent
import linea.domain.RetryConfig
import linea.ethapi.EthLogsSearcherImpl
import linea.kotlin.gwei
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.log4j.configureLoggers
import linea.staterecovery.test.assertBesuAndShomeiRecoveredAsExpected
import linea.staterecovery.test.execCommandAndAssertSuccess
import linea.staterecovery.test.getFinalizationsOnL1
import linea.staterecovery.test.getLastFinalizationOnL1
import linea.staterecovery.test.waitExecutionLayerToBeUpAndRunning
import linea.testing.Runner
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import linea.web3j.waitForTxReceipt
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.testing.filesystem.getPathTo
import net.consensys.zkevm.ethereum.L2AccountManager
import net.consensys.zkevm.ethereum.Web3jClientManager
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.net.URI
import java.nio.file.Files
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class StateRecoveryE2ETest {
  private val log = LogManager.getLogger("test.case.StateRecoverAppWithLocalStackIntTest")
  private lateinit var stateManagerClient: StateManagerClientV1
  private val executionLayerUrl = "http://localhost:9145"
  private val stateManagerUrl = "http://localhost:8890"

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    val jsonRpcFactory = VertxHttpJsonRpcClientFactory(
      vertx = vertx,
      metricsFacade = MicrometerMetricsFacade(SimpleMeterRegistry()),
    )

    stateManagerClient = StateManagerV1JsonRpcClient.create(
      rpcClientFactory = jsonRpcFactory,
      endpoints = listOf(URI(stateManagerUrl)),
      maxInflightRequestsPerClient = 1U,
      requestRetry = RequestRetryConfig(
        backoffDelay = 10.milliseconds,
        timeout = 2.seconds,
      ),
      zkStateManagerVersion = "2.3.0",
      logger = LogManager.getLogger("test.clients.l1.state-manager"),
    )

    configureLoggers(
      rootLevel = Level.INFO,
      log.name to Level.DEBUG,
      "net.consensys.linea.contract.Web3JContractAsyncHelper" to Level.WARN,
      "test.clients.l1.executionlayer" to Level.INFO,
      "test.clients.l1.web3j-default" to Level.INFO,
      "test.clients.l1.state-manager" to Level.DEBUG,
      "test.clients.l1.transaction-details" to Level.INFO,
      "test.clients.l1.linea-contract" to Level.INFO,
      "test.clients.l1.events-fetcher" to Level.INFO,
      "test.clients.l1.blobscan" to Level.INFO,
      "net.consensys.linea.contract.l1" to Level.INFO,
    )
  }

  private fun freshStartOfStack() {
    log.debug("restarting the stack")
    Runner.executeCommandFailOnNonZeroExitCode(
      command = "make start-env-with-staterecovery",
      envVars = mapOf(
        "L1_GENESIS_TIME" to Clock.System.now().plus(5.seconds).epochSeconds.toString(),
      ),
      timeout = 2.minutes,
    ).get()
    log.debug("stack restarted")
  }

  @Test
  fun `should recover from middle of chain and be resilient to node restarts`(
    vertx: Vertx,
  ) {
    // Part A:
    // we shall have multiple finalizations on L1
    // restart FRESH (empty state) Besu & Shomei with recovery block somewhere in the middle of those finalizations
    // will partially sync through P2P network
    // then will trigger recovery mode and sync the remaining blocks
    // assert Besu and Shomei are in sync
    // Part B:
    // send some txs to L2 to trigger coordinator to finalize on L1
    // wait for at least 1 more finalization on L1
    // assert Besu and Shomei are in sync
    // Part C:
    // restart zkbesu-node only
    // send some txs to L2 to trigger coordinator to finalize on L1
    // wait for at least 1 more finalization on L1
    // assert Besu and Shomei are in sync

    freshStartOfStack()
    // No Errors should be logged in Besu
    assertThat(getBesuErrorLogs()).isEmpty()

    val localStackL1ContractAddress = "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9"
    val logsSearcher = EthLogsSearcherImpl(
      vertx = vertx,
      ethApiClient = createEthApiClient(
        web3jClient =
        Web3jClientManager.buildL1Client(
          log = LogManager.getLogger("test.clients.l1.events-fetcher"),
          requestResponseLogLevel = Level.TRACE,
          failuresLogLevel = Level.WARN,
        ),
        requestRetryConfig = RetryConfig.noRetries,
        vertx = null,
      ),
      EthLogsSearcherImpl.Config(
        loopSuccessBackoffDelay = 1.milliseconds,
      ),
      log = LogManager.getLogger("test.clients.l1.events-fetcher"),
    )
    val web3jElClient = createWeb3jHttpClient(executionLayerUrl)
    log.info("starting test flow: besu staterecovery block={}", web3jElClient.ethBlockNumber().send().blockNumber)
    // generate some activity on L2 for coordinator to finalize on L1
    val keepSendingTxToL2 = AtomicBoolean(true)
    sendTxToL2(keepSendingTxToL2::get)

    // A
    // await for 3 finalizations to happen on L1
    val lastFinalizationA = run {
      var finalizationLogs: List<EthLogEvent<DataFinalizedV3>> = emptyList()
      await()
        .atMost(2.minutes.toJavaDuration())
        .untilAsserted {
          finalizationLogs = getFinalizationsOnL1(logsSearcher, localStackL1ContractAddress)
          assertThat(finalizationLogs.size).isGreaterThan(2)
        }
      finalizationLogs.last()
    }

    log.info("lastFinalizationA={}", lastFinalizationA.event.intervalString())

    // await for coordinator to finalize on L1 at least once
    val stateRecoveryStartBlockNumber = lastFinalizationA.event.startBlockNumber - 2UL

    log.info("restarting Besu+Shomei for recovery of state pushed to L1 by coordinator")
    execCommandAndAssertSuccess(
      command = "make staterecovery-replay-from-block " +
        "L1_ROLLUP_CONTRACT_ADDRESS=$localStackL1ContractAddress " +
        "STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER=$stateRecoveryStartBlockNumber",
      log = log,
    ).get()
    // No Errors should be logged in Besu
    assertThat(getBesuErrorLogs()).isEmpty()
    // wait for Besu to be up and running
    waitExecutionLayerToBeUpAndRunning(executionLayerUrl, log = log)
    // assert besu and shomei could sync through P2P network
    assertBesuAndShomeiRecoveredAsExpected(
      web3jElClient,
      stateManagerClient,
      lastFinalizationA.event.endBlockNumber,
      lastFinalizationA.event.finalStateRootHash,
    )
    // No Errors should be logged in Besu
    assertThat(getBesuErrorLogs()).isEmpty()

    // B
    // await for coordinator to finalize on L1 again
    await()
      .atMost(2.minutes.toJavaDuration())
      .untilAsserted {
        getLastFinalizationOnL1(logsSearcher, localStackL1ContractAddress)
          .also { finalization ->
            assertThat(finalization.event.startBlockNumber)
              .isGreaterThan(lastFinalizationA.event.endBlockNumber)
          }
      }
    val lastFinalizationB = getLastFinalizationOnL1(logsSearcher, localStackL1ContractAddress)
    log.info("lastFinalizationB={}", lastFinalizationB.event.intervalString())
    assertBesuAndShomeiRecoveredAsExpected(
      web3jElClient,
      stateManagerClient,
      lastFinalizationB.event.endBlockNumber,
      lastFinalizationB.event.finalStateRootHash,
    )
    // No Errors should be logged in Besu
    assertThat(getBesuErrorLogs()).isEmpty()

    // C.
    // restart besu node with non-graceful shutdown
    log.info("Restarting zkbesu-shomei node")
    execCommandAndAssertSuccess(
      command = "docker restart -s 9 zkbesu-shomei-sr",
      log = log,
    ).get()
    // No Errors should be logged in Besu
    assertThat(getBesuErrorLogs()).isEmpty()

    waitExecutionLayerToBeUpAndRunning(executionLayerUrl, log = log)
    val lastRecoveredBlock = web3jElClient.ethBlockNumber().send().blockNumber.toULong()
    // await coordinator to finalize on L1, beyond what's already in sync
    await()
      .atMost(2.minutes.toJavaDuration())
      .untilAsserted {
        getLastFinalizationOnL1(logsSearcher, localStackL1ContractAddress)
          .also { lastFinalizationC ->
            assertThat(lastFinalizationC.event.startBlockNumber)
              .isGreaterThan(lastFinalizationB.event.endBlockNumber)
            assertThat(lastFinalizationC.event.endBlockNumber).isGreaterThan(lastRecoveredBlock)
          }
      }
    keepSendingTxToL2.set(false)

    val lastFinalizationC = getLastFinalizationOnL1(logsSearcher, localStackL1ContractAddress)
    log.info("lastFinalizationC={}", lastFinalizationC.event.intervalString())

    assertBesuAndShomeiRecoveredAsExpected(
      web3jElClient,
      stateManagerClient,
      lastFinalizationC.event.endBlockNumber,
      lastFinalizationC.event.finalStateRootHash,
    )
    // No Errors should be logged in Besu
    assertThat(getBesuErrorLogs()).isEmpty()
  }

  private fun sendTxToL2(
    keepSendingPredicate: () -> Boolean,
  ) {
    val account = L2AccountManager.generateAccount()
    val txManager = L2AccountManager.getTransactionManager(account)
    Thread {
      while (keepSendingPredicate()) {
        val txHash = txManager.sendTransaction(
          /*gasPrice*/
          150UL.gwei.toBigInteger(),
          /*gasLimit*/
          25_000UL.toBigInteger(),
          /*to*/
          account.address,
          /*data*/
          "",
          /*value*/
          1UL.toBigInteger(),
        ).transactionHash
        log.trace("sent tx to L2, txHash={}", txHash)
        Web3jClientManager.l2Client.waitForTxReceipt(
          txHash = txHash,
          timeout = 5.seconds,
          pollingInterval = 500.milliseconds,
        )
      }
    }.start()
  }

  private fun getBesuErrorLogs(): List<String> {
    val tmpLogDir = getPathTo("tmp/local").resolve("test-logs")
    if (!Files.exists(tmpLogDir)) {
      Files.createDirectory(tmpLogDir)
    }
    val tmpFile = tmpLogDir.resolve("zkbesu-shomei-sr-e2e-test.logs")
    // We need this workaround because the Java native implementation hangs if STDOUT is too long
    Runner
      .executeCommandFailOnNonZeroExitCode("docker logs zkbesu-shomei-sr > ${tmpFile.toAbsolutePath()}")
      .get()

    val errorLogs = Files.readAllLines(tmpFile).filter { it.contains("ERROR", ignoreCase = true) }

    Files.delete(tmpFile)

    return errorLogs
  }
}
