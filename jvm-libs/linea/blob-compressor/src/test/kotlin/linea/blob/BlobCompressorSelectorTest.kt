package linea.blob

import net.consensys.linea.nativecompressor.CompressorTestData
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotNull
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.CyclicBarrier
import java.util.concurrent.TimeUnit
import kotlin.concurrent.atomics.AtomicReference
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.time.Duration.Companion.days
import kotlin.time.Instant

class BlobCompressorSelectorTest {

  @Test
  fun `returns correct BlobCompressor for matching timestamp`() {
    val v1 = BlobCompressorVersion.V1_2
    val v2 = BlobCompressorVersion.V2
    val t1 = Instant.parse("2025-01-01T00:00:00Z")
    val t2 = Instant.parse("2026-01-01T00:00:00Z")
    val dataLimit = 100
    val selector = BlobCompressorSelectorByTimestamp(
      mapOf(v1 to t1, v2 to t2),
      dataLimit,
    )
    // Should select v2 for t2, v1 for t1 and mid-timestamps
    val compressorV2 = selector.getBlobCompressor(t2)
    val compressorV1 = selector.getBlobCompressor(t1)
    val compressorMid = selector.getBlobCompressor(Instant.parse("2025-06-01T00:00:00Z"))
    assertNotNull(compressorV2)
    assertNotNull(compressorV1)
    assertNotNull(compressorMid)
    assertEquals(compressorV1, compressorMid)
    assertTrue(compressorV2 != compressorV1)
  }

  @Test
  fun `returns the same BlobCompressor object`() {
    val v1 = BlobCompressorVersion.V1_2
    val t1 = Instant.parse("2025-01-01T00:00:00Z")
    val dataLimit = 100
    val selector = BlobCompressorSelectorByTimestamp(
      mapOf(v1 to t1),
      dataLimit,
    )
    // Should select v1 for t2, v1 for t1 and mid-timestamps
    val compressorV1 = selector.getBlobCompressor(t1)
    assertNotNull(compressorV1)
    assertEquals(compressorV1, selector.getBlobCompressor(t1))
    assertEquals(compressorV1, selector.getBlobCompressor(t1.plus(1.days)))
  }

  @Test
  fun `throws if no version matches timestamp`() {
    val v1 = BlobCompressorVersion.V1_2
    val t1 = Instant.parse("2025-01-01T00:00:00Z")
    val dataLimit = 100
    val selector = BlobCompressorSelectorByTimestamp(mapOf(v1 to t1), dataLimit)
    val before = Instant.parse("2024-01-01T00:00:00Z")
    assertThrows<IllegalStateException> {
      selector.getBlobCompressor(before)
    }
  }

  @Test
  fun `compresses data using both versions without exception`() {
    val v2 = BlobCompressorVersion.V2
    val v3 = BlobCompressorVersion.V3
    val t2 = Instant.parse("2025-01-01T00:00:00Z")
    val t3 = Instant.parse("2026-01-01T00:00:00Z")
    val dataLimit = 1000000
    val selector = BlobCompressorSelectorByTimestamp(
      mapOf(v2 to t2, v3 to t3),
      dataLimit,
    )
    val sampleBlock = CompressorTestData.blocksRlpEncoded.first()
    val compressorV2 = selector.getBlobCompressor(t2)
    val compressorV3 = selector.getBlobCompressor(t3)
    assertTrue(compressorV2 != compressorV3)

    compressorV2.startNewBatch()
    compressorV2.appendBlock(sampleBlock)
    val compressedV2 = compressorV2.getCompressedDataAndReset()
    assertNotNull(compressedV2)

    compressorV3.startNewBatch()
    compressorV3.appendBlock(sampleBlock)
    val compressedV3 = compressorV3.getCompressedDataAndReset()
    assertNotNull(compressedV3)

    assertThat(compressedV2).isNotEqualTo(compressedV3) // Different versions should produce different compressed data
  }

  @OptIn(ExperimentalAtomicApi::class)
  @Test
  fun `compresses multiple blocks concurrently with v2 and v3 without exception`() {
    val v2 = BlobCompressorVersion.V2
    val v3 = BlobCompressorVersion.V3
    val t2 = Instant.parse("2025-01-01T00:00:00Z")
    val t3 = Instant.parse("2026-01-01T00:00:00Z")
    val dataLimit = 1_000_000
    val selector = BlobCompressorSelectorByTimestamp(
      mapOf(v2 to t2, v3 to t3),
      dataLimit,
    )

    val compressorV2 = selector.getBlobCompressor(t2)
    val compressorV3 = selector.getBlobCompressor(t3)
    assertTrue(compressorV2 != compressorV3)

    val sampleBlocks = CompressorTestData.blocksRlpEncoded
    assertThat(sampleBlocks).isNotEmpty
    val compressedV2 = compressBlocks(compressorV2, sampleBlocks)
    val compressedV3 = compressBlocks(compressorV3, sampleBlocks)

    assertThat(compressedV2).isEqualTo(compressBlocks(compressorV2, sampleBlocks))
    assertThat(compressedV3).isEqualTo(compressBlocks(compressorV3, sampleBlocks))

    val compressedV2Parallel: AtomicReference<ByteArray> = AtomicReference(ByteArray(0))
    val compressedV3Parallel: AtomicReference<ByteArray> = AtomicReference(ByteArray(0))

    val errors = CopyOnWriteArrayList<Throwable>()
    val startBarrier = CyclicBarrier(3)
    val doneLatch = CountDownLatch(2)

    val v2Thread = Thread {
      try {
        startBarrier.await()
        compressedV2Parallel.store(compressBlocks(compressorV2, sampleBlocks))
      } catch (t: Throwable) {
        errors.add(t)
      } finally {
        doneLatch.countDown()
      }
    }

    val v3Thread = Thread {
      try {
        startBarrier.await()
        compressedV3Parallel.store(compressBlocks(compressorV3, sampleBlocks))
      } catch (t: Throwable) {
        errors.add(t)
      } finally {
        doneLatch.countDown()
      }
    }

    v2Thread.start()
    v3Thread.start()
    startBarrier.await()
    assertTrue(doneLatch.await(10, TimeUnit.SECONDS), "compression threads did not complete in time")

    assertThat(compressedV2Parallel.load()).isEqualTo(compressedV2)
    assertThat(compressedV3Parallel.load()).isEqualTo(compressedV3)
    assertThat(errors).isEmpty()
  }

  fun compressBlocks(compressor: BlobCompressor, blocks: List<ByteArray>): ByteArray {
    blocks.forEach { block ->
      compressor.startNewBatch()
      compressor.appendBlock(block)
    }
    return compressor.getCompressedDataAndReset()
  }
}
