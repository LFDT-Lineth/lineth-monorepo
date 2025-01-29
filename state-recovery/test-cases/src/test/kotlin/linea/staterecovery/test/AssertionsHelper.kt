package linea.staterecovery.test

import build.linea.clients.StateManagerClientV1
import build.linea.domain.BlockInterval
import linea.testing.CommandResult
import linea.testing.Runner
import linea.web3j.createWeb3jHttpClient
import net.consensys.toULong
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

fun execCommandAndAssertSuccess(
  command: String,
  log: Logger
): SafeFuture<CommandResult> {
  return Runner
    .executeCommandFailOnNonZeroExitCode(command, log = log)
    .thenPeek { execResult ->
      log.debug("STDOUT: {}", execResult.stdOutStr)
      log.debug("STDERR: {}", execResult.stdErrStr)
      assertThat(execResult.isSuccess).isTrue()
    }
}

fun assertBesuAndShomeiRecoveredAsExpected(
  web3jElClient: Web3j,
  stateManagerClient: StateManagerClientV1,
  expectedBlockNumber: ULong,
  expectedZkEndStateRootHash: ByteArray,
  timeout: Duration = 60.seconds
) {
  await()
    .pollInterval(1.seconds.toJavaDuration())
    .atMost(timeout.toJavaDuration())
    .untilAsserted {
      assertThat(web3jElClient.ethBlockNumber().send().blockNumber.toULong())
        .isGreaterThanOrEqualTo(expectedBlockNumber)
      val blockInterval = BlockInterval(expectedBlockNumber, expectedBlockNumber)
      assertThat(stateManagerClient.rollupGetStateMerkleProof(blockInterval).get().zkEndStateRootHash)
        .isEqualTo(expectedZkEndStateRootHash)
    }
}

fun waitExecutionLayerToBeUpAndRunning(
  executionLayerUrl: String,
  expectedHeadBlockNumber: ULong = 0UL,
  log: Logger,
  timeout: Duration = 2.minutes
) {
  val web3jElClient = createWeb3jHttpClient(executionLayerUrl)
  await()
    .pollInterval(1.seconds.toJavaDuration())
    .atMost(timeout.toJavaDuration())
    .untilAsserted {
      runCatching {
        assertThat(web3jElClient.ethBlockNumber().send().blockNumber.toULong())
          .isGreaterThanOrEqualTo(expectedHeadBlockNumber)
      }.getOrElse {
        log.info("waiting for Besu to start, trying to connect to $executionLayerUrl")
        throw AssertionError("could not connect to $executionLayerUrl", it)
      }
    }
}
