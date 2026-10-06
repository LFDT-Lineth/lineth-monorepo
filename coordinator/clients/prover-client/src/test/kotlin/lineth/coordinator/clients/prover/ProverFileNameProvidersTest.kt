package lineth.coordinator.clients.prover

import linea.domain.BlockIntervalProofIndex
import linea.kotlin.decodeHex
import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class ProverFileNameProvidersTest {

  @Test
  fun test_l2ExecutionProof_fileNameStartBlockNumber() {
    val fileNameProvider = L2ExecutionProofFileNameProvider
    val fileName = fileNameProvider.getFileName(
      BlockIntervalProofIndex(
        startBlockNumber = 11u,
        endBlockNumber = 17u,
        startBlockTimestamp = Instant.fromEpochSeconds(0),
        hash = "abcd".decodeHex(),
      ),
    )

    Assertions.assertEquals(11L, fileNameProvider.getFileNameStartBlockNumber(fileName))
  }

  @Test
  fun test_rollupProof_fileNameStartBlockNumber() {
    val fileNameProvider = RollupProofFileNameProvider
    val fileName = fileNameProvider.getFileName(
      BlockIntervalProofIndex(
        startBlockNumber = 21u,
        endBlockNumber = 29u,
        startBlockTimestamp = Instant.fromEpochSeconds(0),
        hash = "0abcd123".decodeHex(),
      ),
    )

    Assertions.assertEquals(21L, fileNameProvider.getFileNameStartBlockNumber(fileName))
  }

  @Test
  fun test_rollupAggregationProof_fileNameStartBlockNumber() {
    val fileNameProvider = RollupAggregationProofFileNameProvider
    val fileName = fileNameProvider.getFileName(
      BlockIntervalProofIndex(
        startBlockNumber = 31u,
        endBlockNumber = 39u,
        startBlockTimestamp = Instant.fromEpochSeconds(0),
        hash = "ff00aa".decodeHex(),
      ),
    )

    Assertions.assertEquals(31L, fileNameProvider.getFileNameStartBlockNumber(fileName))
  }
}
