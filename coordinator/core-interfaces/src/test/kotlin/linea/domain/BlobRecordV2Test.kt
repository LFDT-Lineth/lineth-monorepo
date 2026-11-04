package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.random.Random

class BlobRecordV2Test {
  private val parent = Random.nextBytes(32)
  private val end = Random.nextBytes(32)

  @Test
  fun `a fresh-start record chains its blobs from the parent hash`() {
    val middle = Random.nextBytes(32)
    val record = createBlobRecordV2(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      parentDataRollingHash = parent,
      dataRollingHash = end,
      blobsData = listOf(createBlobData(parent, middle), createBlobData(middle, end)),
    )

    assertThat(record.blobsData).hasSize(2)
  }

  @Test
  fun `a record starting inside a shared blob puts that blob between boundaryPrev and parent`() {
    val record = createBlobRecordV2(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      parentDataRollingHash = parent,
      dataRollingHash = end,
      parentDataTailTakeBytes = 100,
    )

    assertThat(record.blobsData.first().dataRollingHash).isEqualTo(parent)
    assertThat(record.blobsData.first().prevDataRollingHash).isEqualTo(record.boundaryPrevDataRollingHash)
  }

  @Test
  fun `a record that lies inside the shared blob has the parent as its end hash`() {
    val boundary = Random.nextBytes(32)
    val record = createBlobRecordV2(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      parentDataRollingHash = parent,
      dataRollingHash = parent,
      parentDataTailTakeBytes = 100,
      boundaryPrevDataRollingHash = boundary,
      finalDataTailDiscardBytes = 50,
      blobsData = listOf(createBlobData(boundary, parent)),
    )

    assertThat(record.blobsData).hasSize(1)
  }

  @Test
  fun `blobs that do not chain by rolling hash are rejected`() {
    assertThatThrownBy {
      createBlobRecordV2(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentDataRollingHash = parent,
        dataRollingHash = end,
        blobsData = listOf(createBlobData(parent, Random.nextBytes(32)), createBlobData(Random.nextBytes(32), end)),
      )
    }.hasMessageContaining("blob 1 does not follow blob 0")
    assertThatThrownBy {
      createBlobRecordV2(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentDataRollingHash = parent,
        dataRollingHash = end,
        blobsData = listOf(createBlobData(Random.nextBytes(32), end)),
      )
    }.hasMessageContaining("first blob must follow parentDataRollingHash")
    assertThatThrownBy {
      createBlobRecordV2(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentDataRollingHash = parent,
        dataRollingHash = end,
        blobsData = listOf(createBlobData(parent, Random.nextBytes(32))),
      )
    }.hasMessageContaining("last blob must end at dataRollingHash")
  }

  @Test
  fun `tails and the boundary hash are validated`() {
    assertThatThrownBy {
      createBlobRecordV2(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentDataTailTakeBytes = 100,
        boundaryPrevDataRollingHash = null,
      )
    }.hasMessageContaining("boundaryPrevDataRollingHash must be set iff parentDataTailTakeBytes > 0")
    assertThatThrownBy {
      createBlobRecordV2(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        boundaryPrevDataRollingHash = Random.nextBytes(32),
      )
    }.hasMessageContaining("boundaryPrevDataRollingHash must be set iff parentDataTailTakeBytes > 0")
    assertThatThrownBy {
      createBlobRecordV2(startBlockNumber = 1UL, endBlockNumber = 10UL, finalDataTailDiscardBytes = 130_047)
    }.hasMessageContaining("finalDataTailDiscardBytes=130047 must be in 0..130046")
    assertThatThrownBy {
      createBlobRecordV2(startBlockNumber = 1UL, endBlockNumber = 10UL, blobsData = emptyList())
    }.hasMessageContaining("blobsData must not be empty")
  }

  @Test
  fun `equality includes the tail fields and per-blob hashes`() {
    val record = createBlobRecordV2(startBlockNumber = 1UL, endBlockNumber = 10UL, parentDataTailTakeBytes = 100)
    val equalRecord = record.copy(
      boundaryPrevDataRollingHash = record.boundaryPrevDataRollingHash!!.copyOf(),
      blobsData = record.blobsData.map { it.copy(dataRollingHash = it.dataRollingHash.copyOf()) },
    )

    assertThat(record).isEqualTo(equalRecord)
    assertThat(record.hashCode()).isEqualTo(equalRecord.hashCode())
    assertThat(record).isNotEqualTo(record.copy(finalDataTailDiscardBytes = 1))
  }
}
