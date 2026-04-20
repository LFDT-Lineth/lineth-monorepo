package net.consensys.zkevm.coordinator.app.conflationbacktesting

import linea.domain.Blob
import linea.domain.BlobRecord
import linea.domain.BlockIntervals
import linea.domain.CompressionProofIndex
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import net.consensys.linea.traces.TracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.time.Instant

class InMemoryConsecutiveProvenBlobsProviderTest {

  private val instant = Instant.DISTANT_PAST
  private val traces = TracesCountersV2.EMPTY_TRACES_COUNT

  @Test
  fun `findConsecutiveProvenBlobs returns empty when nothing was accepted`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    assertThat(provider.findConsecutiveProvenBlobs(0L).get()).isEmpty()
  }

  @Test
  fun `acceptProvenBlobRecord without prior capture does not register a proven blob`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    val record = blobRecord(start = 1uL, end = 2uL)
    provider.acceptProvenBlobRecord(
      CompressionProofIndex(
        startBlockNumber = 1uL,
        endBlockNumber = 2uL,
        hash = Random.nextBytes(32),
        startBlockTimestamp = Instant.fromEpochSeconds(1),
      ),
      record,
    )
    assertThat(provider.findConsecutiveProvenBlobs(1L).get()).isEmpty()
  }

  @Test
  fun `capture without accept does not expose blobs in findConsecutiveProvenBlobs`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    provider.captureBlobExecutionProofs(singleConflationBlob(10uL, 20uL))
    assertThat(provider.findConsecutiveProvenBlobs(10L).get()).isEmpty()
  }

  @Test
  fun `single proven blob returns BlobAndBatchCounters with execution proofs and shnarf`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    val blob = singleConflationBlob(1uL, 5uL)
    provider.captureBlobExecutionProofs(blob)
    val shnarf = Random.nextBytes(32)
    val record = blobRecord(start = 1uL, end = 5uL, batches = 1u)
    provider.acceptProvenBlobRecord(
      CompressionProofIndex(
        startBlockNumber = 1uL,
        endBlockNumber = 5uL,
        hash = shnarf,
        startBlockTimestamp = Instant.fromEpochSeconds(1),
      ),
      record,
    )

    val result = provider.findConsecutiveProvenBlobs(1L).get()
    assertThat(result).hasSize(1)
    assertThat(result[0].executionProofs)
      .isEqualTo(BlockIntervals(startingBlockNumber = 1uL, upperBoundaries = listOf(5uL)))
    assertThat(result[0].blobCounters.startBlockNumber).isEqualTo(1uL)
    assertThat(result[0].blobCounters.endBlockNumber).isEqualTo(5uL)
    assertThat(result[0].blobCounters.numberOfBatches).isEqualTo(1u)
    assertThat(result[0].blobCounters.expectedShnarf).isEqualTo(shnarf)
  }

  @Test
  fun `multi conflation blob stores per batch upper boundaries`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    val blob =
      Blob(
        conflations =
        listOf(
          conflation(1uL, 3uL),
          conflation(4uL, 7uL),
        ),
        compressedData = byteArrayOf(),
        startBlockTime = instant,
        endBlockTime = instant,
      )
    provider.captureBlobExecutionProofs(blob)
    provider.acceptProvenBlobRecord(
      CompressionProofIndex(
        startBlockNumber = 1uL,
        endBlockNumber = 7uL,
        hash = Random.nextBytes(32),
        startBlockTimestamp = Instant.fromEpochSeconds(1),
      ),
      blobRecord(start = 1uL, end = 7uL, batches = 2u),
    )

    assertThat(provider.findConsecutiveProvenBlobs(1L).get().single().executionProofs)
      .isEqualTo(BlockIntervals(startingBlockNumber = 1uL, upperBoundaries = listOf(3uL, 7uL)))
  }

  @Test
  fun `findConsecutiveProvenBlobs returns longest consecutive chain from fromBlockNumber`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    proveBlob(provider, start = 1uL, end = 2uL, batches = 1u)
    proveBlob(provider, start = 3uL, end = 4uL, batches = 1u)

    assertThat(provider.findConsecutiveProvenBlobs(1L).get().map { it.blobCounters.startBlockNumber })
      .containsExactly(1uL, 3uL)
  }

  @Test
  fun `findConsecutiveProvenBlobs stops at first gap in block ranges`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    proveBlob(provider, start = 1uL, end = 2uL, batches = 1u)
    proveBlob(provider, start = 5uL, end = 6uL, batches = 1u)

    assertThat(provider.findConsecutiveProvenBlobs(1L).get()).hasSize(1)
    assertThat(provider.findConsecutiveProvenBlobs(1L).get().single().blobCounters.endBlockNumber).isEqualTo(2uL)
  }

  @Test
  fun `findConsecutiveProvenBlobs can start mid chain`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    proveBlob(provider, start = 1uL, end = 2uL, batches = 1u)
    proveBlob(provider, start = 3uL, end = 4uL, batches = 1u)

    assertThat(provider.findConsecutiveProvenBlobs(3L).get().map { it.blobCounters.startBlockNumber })
      .containsExactly(3uL)
  }

  @Test
  fun `findConsecutiveProvenBlobs returns empty when fromBlockNumber is after all blobs`() {
    val provider = InMemoryConsecutiveProvenBlobsProvider()
    proveBlob(provider, start = 1uL, end = 2uL, batches = 1u)

    assertThat(provider.findConsecutiveProvenBlobs(100L).get()).isEmpty()
  }

  private fun proveBlob(
    provider: InMemoryConsecutiveProvenBlobsProvider,
    start: ULong,
    end: ULong,
    batches: UInt,
  ) {
    provider.captureBlobExecutionProofs(singleConflationBlob(start, end))
    provider.acceptProvenBlobRecord(
      CompressionProofIndex(
        startBlockNumber = start,
        endBlockNumber = end,
        hash = Random.nextBytes(32),
        startBlockTimestamp = Instant.fromEpochSeconds(1),
      ),
      blobRecord(start = start, end = end, batches = batches),
    )
  }

  private fun singleConflationBlob(start: ULong, end: ULong): Blob =
    Blob(
      conflations = listOf(conflation(start, end)),
      compressedData = byteArrayOf(),
      startBlockTime = instant,
      endBlockTime = instant,
    )

  private fun conflation(start: ULong, end: ULong) =
    ConflationCalculationResult(
      startBlockNumber = start,
      endBlockNumber = end,
      conflationTrigger = ConflationTrigger.TARGET_BLOCK_NUMBER,
      tracesCounters = traces,
    )

  private fun blobRecord(
    start: ULong,
    end: ULong,
    batches: UInt = 1u,
  ): BlobRecord {
    val hash = Random.nextBytes(32)
    return BlobRecord(
      startBlockNumber = start,
      endBlockNumber = end,
      blobHash = hash,
      startBlockTime = instant,
      endBlockTime = instant,
      batchesCount = batches,
      expectedShnarf = hash,
    )
  }
}
