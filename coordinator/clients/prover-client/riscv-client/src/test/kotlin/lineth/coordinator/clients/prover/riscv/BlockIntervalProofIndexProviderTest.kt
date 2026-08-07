package lineth.coordinator.clients.prover.riscv

import lineth.crypto.HashFunction
import lineth.domain.BlockInterval
import lineth.domain.StartBlockTimestampProvider
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class BlockIntervalProofIndexProviderTest {
  @Test
  fun `builds an index from the request interval and content`() {
    val request = TestRequest(10UL, 20UL, Instant.fromEpochSeconds(1234))
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
    assertThat(index.hash).isEqualTo(expectedHash)
  }

  private data class TestRequest(
    override val startBlockNumber: ULong,
    override val endBlockNumber: ULong,
    override val startBlockTimestamp: Instant,
  ) : BlockInterval, StartBlockTimestampProvider
}
