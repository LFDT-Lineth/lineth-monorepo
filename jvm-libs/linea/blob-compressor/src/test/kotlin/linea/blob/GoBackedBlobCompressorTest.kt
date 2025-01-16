package linea.blob

import net.consensys.linea.blob.BlobCompressorVersion
import net.consensys.linea.nativecompressor.CompressorTestData
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.assertThrows
import kotlin.random.Random

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class GoBackedBlobCompressorTest {
  companion object {
    private const val DATA_LIMIT = 24 * 1024
    private val TEST_DATA = CompressorTestData.blocksRlpEncoded
    private val compressor = GoBackedBlobCompressor.getInstance(BlobCompressorVersion.V0_1_0, DATA_LIMIT.toUInt())
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
    var blocks = TEST_DATA.iterator()
    var result = compressor.appendBlock(blocks.next())
    // at least one block should be appended
    assertThat(result.blockAppended).isTrue()
    while (result.blockAppended) {
      val blockRlp = blocks.next()
      val canAppend = compressor.canAppendBlock(blockRlp)
      result = compressor.appendBlock(blockRlp)
      // assert consistency between canAppendBlock and appendBlock
      assertThat(canAppend).isEqualTo(result.blockAppended)
      if (!blocks.hasNext()) {
        // recompress again, until the limit is reached
        blocks = TEST_DATA.iterator()
      }
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
