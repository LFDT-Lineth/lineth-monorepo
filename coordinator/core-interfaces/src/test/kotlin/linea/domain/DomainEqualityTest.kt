package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

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

  private fun ByteArray.withFirstByteFlipped(): ByteArray =
    copyOf().also { it[0] = (it[0].toInt() xor 1).toByte() }
}
