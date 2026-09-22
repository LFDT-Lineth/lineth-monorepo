package lineth.coordinator.clients.prover

import linea.crypto.HashFunction
import linea.domain.BlockInterval
import linea.domain.ProofRequestMetaDataProvider
import linea.domain.StartBlockTimestampProvider
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class BlockIntervalProofIndexProviderTest {
  @Test
  fun `builds an index from the request interval and content`() {
    val request = TestRequest(10UL, 20UL, Instant.fromEpochSeconds(1234), Instant.fromEpochSeconds(5678), 42L, 100L)
    val expectedHash = ByteArray(32) { it.toByte() }
    var hashedContent: ByteArray? = null
    val hashFunction = HashFunction {
      hashedContent = it
      expectedHash
    }

    val index = BlockIntervalProofIndexProvider<TestRequest>(hashFunction)(request)

    assertThat(hashedContent).isEqualTo(request.toString().toByteArray())
    assertThat(index.startBlockNumber).isEqualTo(request.startBlockNumber)
    assertThat(index.endBlockNumber).isEqualTo(request.endBlockNumber)
    assertThat(index.startBlockTimestamp).isEqualTo(request.startBlockTimestamp)
    assertThat(index.endBlockTimestamp).isEqualTo(request.endBlockTimestamp)
    assertThat(index.transactionsCount).isEqualTo(request.transactionsCount)
    assertThat(index.totalGasUsed).isEqualTo(request.totalGasUsed)
    assertThat(index.hash).isEqualTo(expectedHash)
  }

  private data class TestRequest(
    override val startBlockNumber: ULong,
    override val endBlockNumber: ULong,
    override val startBlockTimestamp: Instant,
    override val endBlockTimestamp: Instant,
    override val transactionsCount: Long,
    override val totalGasUsed: Long,
  ) : BlockInterval, StartBlockTimestampProvider, ProofRequestMetaDataProvider
}
