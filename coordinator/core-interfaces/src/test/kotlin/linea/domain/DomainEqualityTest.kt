package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class DomainEqualityTest {
  @Test
  fun `BlobRecord equality includes expected shnarf`() {
    val record = createBlobRecord(startBlockNumber = 1UL, endBlockNumber = 1UL)
    val equalRecord = record.copy(
      blobHash = record.blobHash.copyOf(),
      expectedShnarf = record.expectedShnarf.copyOf(),
    )
    val differentRecord = record.copy(expectedShnarf = record.expectedShnarf.withFirstByteFlipped())

    assertThat(record).isEqualTo(equalRecord)
    assertThat(record.hashCode()).isEqualTo(equalRecord.hashCode())
    assertThat(record).isNotEqualTo(differentRecord)
  }

  @Test
  fun `BlobSubmittedEvent equality compares transaction hash contents`() {
    val event = BlobSubmittedEvent(
      endBlockNumber = 2UL,
      endBlockTimestamp = Instant.fromEpochSeconds(10),
      lastShnarf = byteArrayOf(1),
      submissionTimestamp = Instant.fromEpochSeconds(20),
      transactionHash = byteArrayOf(2),
    )
    val equalEvent = event.copy(
      lastShnarf = event.lastShnarf.copyOf(),
      transactionHash = event.transactionHash.copyOf(),
    )
    val differentEvent = event.copy(transactionHash = event.transactionHash.withFirstByteFlipped())

    assertThat(event).isEqualTo(equalEvent)
    assertThat(event.hashCode()).isEqualTo(equalEvent.hashCode())
    assertThat(event).isNotEqualTo(differentEvent)
  }

  @Test
  fun `FinalizationSubmittedEvent equality compares transaction hash contents`() {
    val event = FinalizationSubmittedEvent(
      aggregationProof = createProofToFinalize(firstBlockNumber = 1L, finalBlockNumber = 2L),
      parentShnarf = byteArrayOf(1),
      parentL1RollingHash = byteArrayOf(2),
      parentL1RollingHashMessageNumber = 3L,
      submissionTimestamp = Instant.fromEpochSeconds(20),
      transactionHash = byteArrayOf(4),
    )
    val equalEvent = event.copy(
      parentShnarf = event.parentShnarf.copyOf(),
      parentL1RollingHash = event.parentL1RollingHash.copyOf(),
      transactionHash = event.transactionHash.copyOf(),
    )
    val differentEvent = event.copy(transactionHash = event.transactionHash.withFirstByteFlipped())

    assertThat(event).isEqualTo(equalEvent)
    assertThat(event.hashCode()).isEqualTo(equalEvent.hashCode())
    assertThat(event).isNotEqualTo(differentEvent)
  }

  private fun ByteArray.withFirstByteFlipped(): ByteArray =
    copyOf().also { it[0] = (it[0].toInt() xor 1).toByte() }
}
