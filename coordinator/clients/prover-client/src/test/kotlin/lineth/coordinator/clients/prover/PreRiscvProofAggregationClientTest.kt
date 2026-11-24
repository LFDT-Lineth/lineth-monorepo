package lineth.coordinator.clients.prover

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.ProverFileNameProvider
import linea.crypto.Sha256HashFunction
import linea.domain.BlockIntervals
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.domain.ProofsToAggregate
import lineth.coordinator.clients.prover.RiscvProverClientTestFixtures.fileBasedProverConfig
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class PreRiscvProofAggregationClientTest {
  private val request = ProofsToAggregate(
    compressionProofIndexes = listOf(
      CompressionProofIndex(
        startBlockNumber = 1uL,
        endBlockNumber = 9uL,
        hash = ByteArray(32) { 0x01 },
        startBlockTimestamp = Instant.fromEpochSeconds(1),
      ),
    ),
    executionProofs = BlockIntervals(startingBlockNumber = 1uL, listOf(4uL, 9uL)),
    invalidityProofs = emptyList(),
    parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(0),
    parentAggregationLastL1RollingHashMessageNumber = 0uL,
    parentAggregationLastL1RollingHash = ByteArray(32),
    parentAggregationLastFtxNumber = 0uL,
    parentAggregationLastFtxRollingHash = ByteArray(32),
    startBlockTimestamp = Instant.fromEpochSeconds(1),
  )

  private val customExecutionProofFileNameProvider = object : ProverFileNameProvider<ExecutionProofIndex> {
    override fun getFileName(proofIndex: ExecutionProofIndex): String =
      "custom-${proofIndex.startBlockNumber}-${proofIndex.endBlockNumber}.json"
  }

  @Test
  fun `proof index hash uses the configured response file name providers`(vertx: Vertx, @TempDir tempDir: Path) {
    val hashFunction = Sha256HashFunction()
    val client = PreRiscvProofAggregationClient(
      vertx = vertx,
      config = fileBasedProverConfig(tempDir),
      hashFunction = hashFunction,
      executionProofResponseFileNameProvider = customExecutionProofFileNameProvider,
      log = LogManager.getLogger(PreRiscvProofAggregationClientTest::class.java),
    )

    val expected = PreRiscvProofAggregationClient.createProofIndexProviderFn(
      hashFunction = hashFunction,
      executionProofResponseFileNameProvider = customExecutionProofFileNameProvider,
    )(request)
    val withDefaultProviders = PreRiscvProofAggregationClient.createProofIndexProviderFn(hashFunction)(request)

    val proofIndex = client.getProofIndex(request)
    assertThat(proofIndex.hash).isEqualTo(expected.hash)
    assertThat(proofIndex.hash).isNotEqualTo(withDefaultProviders.hash)
  }
}
