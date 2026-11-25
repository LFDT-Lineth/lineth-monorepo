package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.random.Random

class RiscvRollupRecordTest {
  private val parent = Random.nextBytes(32)
  private val end = Random.nextBytes(32)

  @Test
  fun `a fresh-start record chains its blobs from the parent hash`() {
    val middle = Random.nextBytes(32)
    val record = createRiscvRollupRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      parentDataRollingHash = parent,
      dataRollingHash = end,
      blobsData = listOf(createBlobData(parent, middle), createBlobData(middle, end)),
    )

    assertThat(record.blobsData).hasSize(2)
  }

  @Test
  fun `a record starting inside a shared blob puts that blob between the parent blob and parent hashes`() {
    val record = createRiscvRollupRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      parentDataRollingHash = parent,
      dataRollingHash = end,
      parentSharedBlobCarryOverTailBytes = 100u,
    )

    assertThat(record.blobsData.first().dataRollingHash).isEqualTo(parent)
    assertThat(record.blobsData.first().prevDataRollingHash).isEqualTo(record.parentBlobDataRollingHash)
  }

  @Test
  fun `a record that lies inside the shared blob has the parent as its end hash`() {
    val parentBlob = Random.nextBytes(32)
    val record = createRiscvRollupRecord(
      startBlockNumber = 1UL,
      endBlockNumber = 10UL,
      parentDataRollingHash = parent,
      dataRollingHash = parent,
      parentSharedBlobCarryOverTailBytes = 100u,
      parentBlobDataRollingHash = parentBlob,
      finalSharedBlobCarryOverTailBytes = 50u,
      blobsData = listOf(createBlobData(parentBlob, parent)),
    )

    assertThat(record.blobsData).hasSize(1)
  }

  @Test
  fun `blobs that do not chain by rolling hash are rejected`() {
    assertThatThrownBy {
      createRiscvRollupRecord(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentDataRollingHash = parent,
        dataRollingHash = end,
        blobsData = listOf(createBlobData(parent, Random.nextBytes(32)), createBlobData(Random.nextBytes(32), end)),
      )
    }.hasMessageContaining("blob 1 does not follow blob 0")
    assertThatThrownBy {
      createRiscvRollupRecord(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentDataRollingHash = parent,
        dataRollingHash = end,
        blobsData = listOf(createBlobData(Random.nextBytes(32), end)),
      )
    }.hasMessageContaining("first blob must follow parentDataRollingHash")
    assertThatThrownBy {
      createRiscvRollupRecord(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentDataRollingHash = parent,
        dataRollingHash = end,
        blobsData = listOf(createBlobData(parent, Random.nextBytes(32))),
      )
    }.hasMessageContaining("last blob must end at dataRollingHash")
  }

  @Test
  fun `tails and the parent blob hash are validated`() {
    assertThatThrownBy {
      createRiscvRollupRecord(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentSharedBlobCarryOverTailBytes = 100u,
        parentBlobDataRollingHash = null,
      )
    }.hasMessageContaining("parentBlobDataRollingHash is required when parentSharedBlobCarryOverTailBytes > 0")
    assertThatThrownBy {
      createRiscvRollupRecord(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        parentBlobDataRollingHash = Random.nextBytes(32),
      )
    }.hasMessageContaining("parentBlobDataRollingHash must be null when the rollup starts on a fresh blob")
    assertThatThrownBy {
      createRiscvRollupRecord(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        finalSharedBlobCarryOverTailBytes = 130_047u,
      )
    }.hasMessageContaining("finalSharedBlobCarryOverTailBytes=130047 must be in 0..130046")
    assertThatThrownBy {
      createRiscvRollupRecord(startBlockNumber = 1UL, endBlockNumber = 10UL, blobsData = emptyList())
    }.hasMessageContaining("blobsData must not be empty")
  }

  @Test
  fun `equality includes the tail fields and per-blob hashes`() {
    val record =
      createRiscvRollupRecord(startBlockNumber = 1UL, endBlockNumber = 10UL, parentSharedBlobCarryOverTailBytes = 100u)
    val equalRecord = record.copy(
      parentBlobDataRollingHash = record.parentBlobDataRollingHash!!.copyOf(),
      blobsData = record.blobsData.map { it.copy(dataRollingHash = it.dataRollingHash.copyOf()) },
    )

    assertThat(record).isEqualTo(equalRecord)
    assertThat(record.hashCode()).isEqualTo(equalRecord.hashCode())
    assertThat(record).isNotEqualTo(record.copy(finalSharedBlobCarryOverTailBytes = 1u))
  }
}
