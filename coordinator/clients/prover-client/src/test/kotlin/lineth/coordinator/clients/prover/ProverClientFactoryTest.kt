package lineth.coordinator.clients.prover

import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class ProverClientFactoryTest {
  private fun buildProversConfig(
    tmpDir: Path,
    switchBlockNumber: Int? = null,
    switchBlockTimestamp: Instant? = null,
    withProverB: Boolean = switchBlockNumber != null || switchBlockTimestamp != null,
  ): ProversConfig<ProverConfig> {
    fun buildFileBasedProverConfig(
      proverDir: Path,
    ): FileBasedProverConfig {
      return FileBasedProverConfig(
        requestsDirectory = proverDir.resolve("requests"),
        responsesDirectory = proverDir.resolve("responses"),
        pollingInterval = 100.milliseconds,
        pollingTimeout = 500.milliseconds,
        inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
        inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
      )
    }

    fun buildProverConfig(proverDir: Path): ProverConfig {
      return ProverConfig(
        l2Execution = ProverClientConfig(
          fileBased = buildFileBasedProverConfig(proverDir.resolve("l2-execution")),
          restfulBased = null,
          programId = RiscvProverClientTestFixtures.L2_EXECUTION_PROGRAM_ID,
          provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
          forkName = RiscvProverClientTestFixtures.FORK_NAME,
        ),
        rollup = ProverClientConfig(
          fileBased = buildFileBasedProverConfig(proverDir.resolve("rollup")),
          restfulBased = null,
          programId = RiscvProverClientTestFixtures.ROLLUP_PROGRAM_ID,
          provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
          forkName = RiscvProverClientTestFixtures.FORK_NAME,
        ),
        rollupAggregation = ProverClientConfig(
          fileBased = buildFileBasedProverConfig(proverDir.resolve("rollup-aggregation")),
          restfulBased = null,
          programId = RiscvProverClientTestFixtures.ROLLUP_AGGREGATION_PROGRAM_ID,
          provingSystemVersion = RiscvProverClientTestFixtures.PROVING_SYSTEM_VERSION,
          forkName = RiscvProverClientTestFixtures.FORK_NAME,
        ),
      )
    }

    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = buildProverConfig(tmpDir.resolve("riscv-prover/v1")),
        next = if (withProverB) {
          buildProverConfig(tmpDir.resolve("riscv-prover/v2"))
        } else {
          null
        },
      ),
      switchBlockNumberInclusive = switchBlockNumber?.toULong(),
      switchBlockTimestamp = switchBlockTimestamp,
      enableRequestFilesCleanup = false,
    )
  }

  private lateinit var meterRegistry: MeterRegistry
  private lateinit var metricsFacade: MetricsFacade
  private lateinit var proverClientFactory: ProverClientFactory
  private lateinit var vertx: Vertx
  private lateinit var testTmpDir: Path

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tmpDir: Path) {
    this.vertx = vertx
    this.testTmpDir = tmpDir
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    proverClientFactory =
      ProverClientFactory(
        vertx = vertx,
        config = buildProversConfig(testTmpDir, switchBlockNumber = 200),
        chainId = 59144L,
        metricsFacade = metricsFacade,
      )
  }

  @Test
  fun `l2ExecutionProverClient should build L2 execution prover client when programVk and forkName are set`() {
    val factory = ProverClientFactory(
      vertx = vertx,
      config = buildProversConfig(testTmpDir),
      l2MessageServiceAddress = "0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec",
      chainId = 59144L,
      metricsFacade = metricsFacade,
    )

    val client = factory.l2ExecutionProverClient()
    assertThat(client).isNotNull
  }

  @Test
  fun `l2ExecutionProverClient should fail when l2MessageServiceAddress is not configured`() {
    val factory = ProverClientFactory(
      vertx = vertx,
      config = buildProversConfig(testTmpDir),
      l2MessageServiceAddress = null,
      chainId = 59144L,
      metricsFacade = metricsFacade,
    )

    assertThatThrownBy { factory.l2ExecutionProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("l2MessageServiceAddress must be configured for the RISC-V execution prover")
  }

  @Test
  fun `l2ExecutionProverClient should fail when l2MessageServiceAddress is empty`() {
    val factory = ProverClientFactory(
      vertx = vertx,
      config = buildProversConfig(testTmpDir),
      l2MessageServiceAddress = "",
      chainId = 59144L,
      metricsFacade = metricsFacade,
    )

    assertThatThrownBy { factory.l2ExecutionProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("l2MessageServiceAddress must be configured for the RISC-V execution prover")
  }

  @Test
  fun `should fail with clear error when block number switch has no prover B`() {
    val factory =
      ProverClientFactory(
        vertx = vertx,
        config = buildProversConfig(testTmpDir, switchBlockNumber = 200, withProverB = false),
        chainId = 59144L,
        metricsFacade = metricsFacade,
      )

    assertThatThrownBy { factory.rollupProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("proverBConfig must be provided when switchBlockNumberInclusive is set")
  }

  @Test
  fun `should fail with clear error when timestamp switch has no prover B`() {
    val factory =
      ProverClientFactory(
        vertx = vertx,
        config = buildProversConfig(
          testTmpDir,
          switchBlockTimestamp = Instant.fromEpochSeconds(50),
          withProverB = false,
        ),
        chainId = 59144L,
        metricsFacade = metricsFacade,
      )

    assertThatThrownBy { factory.rollupAggregationProverClient() }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("proverBConfig must be provided when switchBlockTimestamp is set")
  }
}
