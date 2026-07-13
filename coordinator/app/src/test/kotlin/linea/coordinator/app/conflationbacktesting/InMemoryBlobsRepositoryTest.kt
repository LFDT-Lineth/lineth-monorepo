package linea.coordinator.app.conflationbacktesting

import linea.domain.BlobRecord
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class InMemoryBlobsRepositoryTest {
  private fun blob(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
    startBlockTime: Instant = Instant.fromEpochSeconds(startBlockNumber.toLong()),
    endBlockTime: Instant = Instant.fromEpochSeconds(endBlockNumber.toLong()),
  ): BlobRecord = BlobRecord(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    blobHash = ByteArray(32) { startBlockNumber.toByte() },
    startBlockTime = startBlockTime,
    endBlockTime = endBlockTime,
    batchesCount = 1u,
    expectedShnarf = ByteArray(32) { endBlockNumber.toByte() },
  )

  @Test
  fun `saveNewBlob makes it findable by both start and end block number`() {
    val repository = InMemoryBlobsRepository()
    val blob = blob(startBlockNumber = 1u, endBlockNumber = 10u)

    repository.saveNewBlob(blob).get()

    assertThat(repository.findBlobByStartBlockNumber(1L).get()).isEqualTo(blob)
    assertThat(repository.findBlobByEndBlockNumber(10L).get()).isEqualTo(blob)
  }

  @Test
  fun `findBlobByStartBlockNumber and findBlobByEndBlockNumber return null when not found`() {
    val repository = InMemoryBlobsRepository()

    assertThat(repository.findBlobByStartBlockNumber(1L).get()).isNull()
    assertThat(repository.findBlobByEndBlockNumber(1L).get()).isNull()
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber returns consecutive blobs starting from the given block`() {
    val repository = InMemoryBlobsRepository()
    val blob1 = blob(startBlockNumber = 1u, endBlockNumber = 10u)
    val blob2 = blob(startBlockNumber = 11u, endBlockNumber = 20u)
    val blob3 = blob(startBlockNumber = 21u, endBlockNumber = 30u)
    listOf(blob1, blob2, blob3).forEach { repository.saveNewBlob(it).get() }

    val result = repository.getConsecutiveBlobsFromBlockNumber(
      startingBlockNumberInclusive = 1L,
      endBlockCreatedBefore = Instant.fromEpochSeconds(1000),
    ).get()

    assertThat(result).containsExactly(blob1, blob2, blob3)
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber stops at a gap in block numbers`() {
    val repository = InMemoryBlobsRepository()
    val blob1 = blob(startBlockNumber = 1u, endBlockNumber = 10u)
    // gap: no blob covers blocks 11-20
    val blob3 = blob(startBlockNumber = 21u, endBlockNumber = 30u)
    listOf(blob1, blob3).forEach { repository.saveNewBlob(it).get() }

    val result = repository.getConsecutiveBlobsFromBlockNumber(
      startingBlockNumberInclusive = 1L,
      endBlockCreatedBefore = Instant.fromEpochSeconds(1000),
    ).get()

    assertThat(result).containsExactly(blob1)
  }

  @Test
  fun `getConsecutiveBlobsFromBlockNumber excludes blobs whose endBlockTime is not before the cutoff`() {
    val repository = InMemoryBlobsRepository()
    val blob1 = blob(startBlockNumber = 1u, endBlockNumber = 10u, endBlockTime = Instant.fromEpochSeconds(10))
    val blob2 = blob(startBlockNumber = 11u, endBlockNumber = 20u, endBlockTime = Instant.fromEpochSeconds(20))
    listOf(blob1, blob2).forEach { repository.saveNewBlob(it).get() }

    val result = repository.getConsecutiveBlobsFromBlockNumber(
      startingBlockNumberInclusive = 1L,
      endBlockCreatedBefore = Instant.fromEpochSeconds(15),
    ).get()

    assertThat(result).containsExactly(blob1)
  }

  @Test
  fun `deleteBlobsUpToEndBlockNumber removes matching blobs and returns count`() {
    val repository = InMemoryBlobsRepository()
    val blob1 = blob(startBlockNumber = 1u, endBlockNumber = 10u)
    val blob2 = blob(startBlockNumber = 11u, endBlockNumber = 20u)
    listOf(blob1, blob2).forEach { repository.saveNewBlob(it).get() }

    val deletedCount = repository.deleteBlobsUpToEndBlockNumber(10uL).get()

    assertThat(deletedCount).isEqualTo(1)
    assertThat(repository.findBlobByStartBlockNumber(1L).get()).isNull()
    assertThat(repository.findBlobByEndBlockNumber(10L).get()).isNull()
    assertThat(repository.findBlobByStartBlockNumber(11L).get()).isEqualTo(blob2)
  }

  @Test
  fun `deleteBlobsAfterBlockNumber removes matching blobs and returns count`() {
    val repository = InMemoryBlobsRepository()
    val blob1 = blob(startBlockNumber = 1u, endBlockNumber = 10u)
    val blob2 = blob(startBlockNumber = 11u, endBlockNumber = 20u)
    listOf(blob1, blob2).forEach { repository.saveNewBlob(it).get() }

    val deletedCount = repository.deleteBlobsAfterBlockNumber(11uL).get()

    assertThat(deletedCount).isEqualTo(1)
    assertThat(repository.findBlobByStartBlockNumber(11L).get()).isNull()
    assertThat(repository.findBlobByEndBlockNumber(20L).get()).isNull()
    assertThat(repository.findBlobByStartBlockNumber(1L).get()).isEqualTo(blob1)
  }
}
