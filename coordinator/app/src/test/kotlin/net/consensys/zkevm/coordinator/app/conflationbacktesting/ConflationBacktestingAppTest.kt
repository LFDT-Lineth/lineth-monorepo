package net.consensys.zkevm.coordinator.app.conflationbacktesting

import net.consensys.zkevm.coordinator.clients.prover.FileBasedProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProverConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.io.path.Path
import kotlin.time.Duration.Companion.seconds

class ConflationBacktestingAppTest {

  @Test
  fun `path is updated correctly`() {
    val proversConfig = ProverConfig(
      execution = FileBasedProverConfig(
        requestsDirectory = Path("/original/path/to/requests/execution"),
        responsesDirectory = Path("/original/path/to/responses/execution"),
        inprogressProvingSuffixPattern = ".inprogress",
        inprogressRequestWritingSuffix = ".writing",
        pollingInterval = 5.seconds,
        pollingTimeout = 300.seconds,
      ),
      blobCompression = FileBasedProverConfig(
        requestsDirectory = Path("/original/path/to/requests/blobCompression"),
        responsesDirectory = Path("/original/path/to/responses/blobCompression"),
        inprogressProvingSuffixPattern = ".inprogress",
        inprogressRequestWritingSuffix = ".writing",
        pollingInterval = 5.seconds,
        pollingTimeout = 300.seconds,
      ),
      proofAggregation = FileBasedProverConfig(
        requestsDirectory = Path("/original/path/to/requests/proofAggregation"),
        responsesDirectory = Path("/original/path/to/responses/proofAggregation"),
        inprogressProvingSuffixPattern = ".inprogress",
        inprogressRequestWritingSuffix = ".writing",
        pollingInterval = 5.seconds,
        pollingTimeout = 300.seconds,
      ),
    )
    val updatedProversConfig = ConflationBacktestingApp.getUpdatedProverConfig(
      proverConfig = proversConfig,
      backtestingDirectory = Path("/new/backtesting/"),
      conflationBacktestingJobId = "job-123",
    )
    assertThat(updatedProversConfig.execution.requestsDirectory.toString())
      .isEqualTo("/new/backtesting/job-123/execution/requests")
    assertThat(updatedProversConfig.execution.responsesDirectory.toString())
      .isEqualTo("/new/backtesting/job-123/execution/responses")
    assertThat(updatedProversConfig.blobCompression.requestsDirectory.toString())
      .isEqualTo("/new/backtesting/job-123/compression/requests")
    assertThat(updatedProversConfig.blobCompression.responsesDirectory.toString())
      .isEqualTo("/new/backtesting/job-123/compression/responses")
    assertThat(updatedProversConfig.proofAggregation.requestsDirectory.toString())
      .isEqualTo("/new/backtesting/job-123/aggregation/requests")
    assertThat(updatedProversConfig.proofAggregation.responsesDirectory.toString())
      .isEqualTo("/new/backtesting/job-123/aggregation/responses")
  }
}
