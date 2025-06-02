package net.consensys.zkevm.coordination.blob

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import linea.blob.BlobCompressorVersion
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressionException
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobCompressor
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.assertThrows
import java.nio.ByteBuffer
import java.nio.ByteOrder
import kotlin.random.Random

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class GoBackedBlobCompressorTest {
  companion object {
    private const val DATA_LIMIT = 16 * 1024
    private val TEST_DATA = loadTestData()
    private val meterRegistry = SimpleMeterRegistry()
    private val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry, "linea")
    private val compressor = GoBackedBlobCompressor.getInstance(
      BlobCompressorVersion.V1_2,
      DATA_LIMIT.toUInt(),
      metricsFacade,
    )
    private fun loadTestData(): Array<ByteArray> {
      val data = GoBackedBlobCompressorTest::class.java.getResourceAsStream("rlp_blocks.bin")!!.readAllBytes()

      // first 4 bytes are the number of blocks
      val numBlocks = ByteBuffer.wrap(data, 0, 4).order(ByteOrder.LITTLE_ENDIAN).int

      // the rest of the file is the blocks
      // (we repeat them to fill more data)
      val blocks = Array(numBlocks * 2) { ByteArray(0) }

      for (j in 0 until 2) {
        var offset = 4
        for (i in 0 until numBlocks) {
          // first 4 bytes are the length of the block
          val blockLen = ByteBuffer.wrap(data, offset, 4).order(ByteOrder.LITTLE_ENDIAN).int

          // the rest of the block is the block
          blocks[i + j * numBlocks] = ByteArray(blockLen)
          System.arraycopy(data, offset + 4, blocks[i + j * numBlocks], 0, blockLen)
          offset += 4 + blockLen
        }
      }
      return blocks
    }
  }

  @BeforeEach
  fun before() {
    compressor.reset()
  }

  @Test
  fun `test appendBlock with data within limit`() {
    val blocks = TEST_DATA
    val result = compressor.appendBlock(blocks.first())
    assertThat(result.blockAppended).isTrue
    assertThat(result.compressedSizeBefore).isZero()
    assertThat(result.compressedSizeAfter).isGreaterThan(0)
  }

  @Test
  fun `test invalid rlp block`() {
    val block = Random.nextBytes(100)
    assertThrows<BlobCompressionException>("rlp: expected input list for types.extblock") {
      compressor.appendBlock(block)
    }
  }

  @Test
  fun `test compression data limit exceeded`() {
    val blocks = TEST_DATA.iterator()
    var result = compressor.appendBlock(blocks.next())
    while (result.blockAppended && blocks.hasNext()) {
      val blockRlp = blocks.next()
      val canAppend = compressor.canAppendBlock(blockRlp)
      result = compressor.appendBlock(blockRlp)
      // assert consistency between canAppendBlock and appendBlock
      assertThat(canAppend).isEqualTo(result.blockAppended)
    }
    assertThat(result.blockAppended).isFalse()
    assertThat(result.compressedSizeBefore).isGreaterThan(0)
    assertThat(result.compressedSizeAfter).isEqualTo(result.compressedSizeBefore)
  }

  @Test
  fun `test reset`() {
    val blocks = TEST_DATA.iterator()
    assertThat(compressor.goNativeBlobCompressor.Len()).isZero()
    var res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()
    assertThat(res.compressedSizeBefore).isZero()
    assertThat(res.compressedSizeAfter).isGreaterThan(0)
    assertThat(res.compressedSizeAfter).isEqualTo(compressor.goNativeBlobCompressor.Len())

    compressor.reset()

    assertThat(compressor.goNativeBlobCompressor.Len()).isZero()
    res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()
    assertThat(res.compressedSizeBefore).isZero()
    assertThat(res.compressedSizeAfter).isGreaterThan(0)
    assertThat(res.compressedSizeAfter).isEqualTo(compressor.goNativeBlobCompressor.Len())
  }

  @Test
  fun `test batches`() {
    val blocks = TEST_DATA.iterator()
    var res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()

    compressor.startNewBatch()

    res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()
    assertThat(compressor.getCompressedData().size).isGreaterThan(0)
  }
}
