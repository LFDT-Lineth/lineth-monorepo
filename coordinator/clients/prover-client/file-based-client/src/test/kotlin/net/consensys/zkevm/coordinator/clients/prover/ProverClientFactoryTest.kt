package net.consensys.zkevm.coordinator.clients.prover

import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import kotlinx.datetime.Clock
import linea.domain.BlockIntervals
import linea.kotlin.ByteArrayExt
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.domain.ProofsToAggregate
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class ProverClientFactoryTest {
  private fun buildProversConfig(tmpDir: Path, switchBlockNumber: Int? = null): ProversConfig {
    fun buildProverConfig(proverDir: Path): ProverConfig {
      return ProverConfig(
        execution = FileBasedProverConfig(
          requestsDirectory = proverDir.resolve("execution/requests"),
          responsesDirectory = proverDir.resolve("execution/responses"),
          pollingInterval = 100.milliseconds,
          pollingTimeout = 500.milliseconds,
          inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
          inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
        ),
        blobCompression = FileBasedProverConfig(
          requestsDirectory = proverDir.resolve("compression/requests"),
          responsesDirectory = proverDir.resolve("compression/responses"),
          pollingInterval = 100.milliseconds,
          pollingTimeout = 500.milliseconds,
          inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
          inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
        ),
        proofAggregation = FileBasedProverConfig(
          requestsDirectory = proverDir.resolve("aggregation/requests"),
          responsesDirectory = proverDir.resolve("aggregation/responses"),
          pollingInterval = 100.milliseconds,
          pollingTimeout = 500.milliseconds,
          inprogressProvingSuffixPattern = ".*\\.inprogress\\.prover.*",
          inprogressRequestWritingSuffix = ".inprogress_coordinator_writing",
        ),
      )
    }

    return ProversConfig(
      proverA = buildProverConfig(tmpDir.resolve("prover/v2")),
      switchBlockNumberInclusive = switchBlockNumber?.toULong(),
      proverB = switchBlockNumber?.let {
        buildProverConfig(tmpDir.resolve("prover/v3"))
      },
      enableRequestFilesCleanup = false,
    )
  }

  private lateinit var meterRegistry: MeterRegistry
  private lateinit var metricsFacade: MetricsFacade
  private lateinit var proverClientFactory: ProverClientFactory
  private lateinit var vertx: Vertx
  private lateinit var testTmpDir: Path

  private val request1 = ProofsToAggregate(
    compressionProofIndexes = listOf(ProofIndex(startBlockNumber = 1uL, endBlockNumber = 9uL)),
    executionProofs = BlockIntervals(startingBlockNumber = 1uL, listOf(9uL)),
    parentAggregationLastBlockTimestamp = Clock.System.now(),
    parentAggregationLastL1RollingHashMessageNumber = 0uL,
    parentAggregationLastL1RollingHash = ByteArrayExt.random32(),
  )
  private val request2 = ProofsToAggregate(
    compressionProofIndexes = listOf(ProofIndex(startBlockNumber = 10uL, endBlockNumber = 19uL)),
    executionProofs = BlockIntervals(startingBlockNumber = 10uL, listOf(19uL)),
    parentAggregationLastBlockTimestamp = Clock.System.now(),
    parentAggregationLastL1RollingHashMessageNumber = 9uL,
    parentAggregationLastL1RollingHash = ByteArrayExt.random32(),
  )
  private val request3 = ProofsToAggregate(
    compressionProofIndexes = listOf(ProofIndex(startBlockNumber = 300uL, endBlockNumber = 319uL)),
    executionProofs = BlockIntervals(startingBlockNumber = 300uL, listOf(319uL)),
    parentAggregationLastBlockTimestamp = Clock.System.now(),
    parentAggregationLastL1RollingHashMessageNumber = 299uL,
    parentAggregationLastL1RollingHash = ByteArrayExt.random32(),
  )

  @BeforeEach
  fun beforeEach(vertx: Vertx, @TempDir tmpDir: Path) {
    this.vertx = vertx
    this.testTmpDir = tmpDir
    meterRegistry = SimpleMeterRegistry()
    metricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    proverClientFactory =
      ProverClientFactory(vertx, buildProversConfig(testTmpDir, switchBlockNumber = 200), metricsFacade)
  }

  @Test
  fun `should create a prover with routing when switch is defined`() {
    val proverClient = proverClientFactory.proofAggregationProverClient()
    assertThat(proverClient).isInstanceOf(ABProverClientRouter::class.java)

    // swallow timeout exception because responses are not available
    kotlin.runCatching { proverClient.requestProof(request1).get() }
    kotlin.runCatching { proverClient.requestProof(request2).get() }
    kotlin.runCatching { proverClient.requestProof(request3).get() }

    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        Files.list(testTmpDir.resolve("prover/v2/aggregation/requests")).use {
          assertThat(it.count()).isEqualTo(2)
        }
        Files.list(testTmpDir.resolve("prover/v3/aggregation/requests")).use {
          assertThat(it.count()).isEqualTo(1)
        }
      }
  }

  @Test
  fun `should create metrics gauge and aggregate them`() {
    val proverClientI1 = proverClientFactory.proofAggregationProverClient()
    val proverClientI2 = proverClientFactory.proofAggregationProverClient()

    kotlin.runCatching { proverClientI1.requestProof(request1).get() }
    kotlin.runCatching { proverClientI2.requestProof(request2).get() }
    kotlin.runCatching { proverClientI1.requestProof(request3).get() }

    assertThat(meterRegistry.find("linea.batch.prover.waiting").gauge()).isNotNull
    assertThat(meterRegistry.find("linea.blob.prover.waiting").gauge()).isNotNull
    assertThat(meterRegistry.find("linea.aggregation.prover.waiting").gauge()).isNotNull

    assertThat(meterRegistry.find("linea.batch.prover.waiting").gauge()!!.value()).isEqualTo(0.0)
    assertThat(meterRegistry.find("linea.blob.prover.waiting").gauge()!!.value()).isEqualTo(0.0)
    assertThat(meterRegistry.find("linea.aggregation.prover.waiting").gauge()!!.value()).isEqualTo(3.0)
  }
}
