package linea.blob

import net.consensys.linea.nativecompressor.CompressorTestData
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.util.stream.Stream
import kotlin.random.Random

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class GoBackedBlobCompressorLegacyTest {
  companion object {
    private const val DATA_LIMIT = 24 * 1024
    private val TEST_DATA = CompressorTestData.blocksRlpEncoded
    private val compressors =
      listOf(BlobCompressorVersion.V2).map { version ->
        BlobCompressorFactory.getInstance(version, DATA_LIMIT)
      }

    @JvmStatic
    fun legacyCompressors(): Stream<Arguments> =
      Stream.of(*compressors.map { c -> Arguments.of(c.version, c) }.toTypedArray())
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("legacyCompressors")
  fun `test appendBlock with data within limit`(version: BlobCompressorVersion, compressor: BlobCompressor) {
    compressor.reset()
    assertThat(compressor.version).isEqualTo(version)
    val blocks = TEST_DATA
    val result = compressor.appendBlock(blocks.first())
    assertThat(result.blockAppended).isTrue
    assertThat(result.compressedSizeBefore).isZero()
    assertThat(result.compressedSizeAfter).isGreaterThan(0)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("legacyCompressors")
  fun `test canAppend does not actually append`(version: BlobCompressorVersion, compressor: BlobCompressor) {
    compressor.reset()
    assertThat(compressor.version).isEqualTo(version)
    val block = TEST_DATA.first()
    val anotherBlock = TEST_DATA[1]

    assertThat(compressor.getCompressedData()).isEmpty()
    compressor.canAppendBlock(block)
    assertThat(compressor.getCompressedData()).isEmpty()
    repeat(3) {
      compressor.canAppendBlock(anotherBlock)
    }
    assertThat(compressor.getCompressedData()).isEmpty()

    val appendResult = compressor.appendBlock(block)
    assertThat(appendResult.blockAppended).isTrue()
    val compressedAfterAppend = compressor.getCompressedData()
    assertThat(compressedAfterAppend).isNotEmpty()

    compressor.canAppendBlock(anotherBlock)
    assertThat(compressor.getCompressedData()).isEqualTo(compressedAfterAppend)
    repeat(5) {
      compressor.canAppendBlock(anotherBlock)
      compressor.canAppendBlock(block)
    }
    assertThat(compressor.getCompressedData()).isEqualTo(compressedAfterAppend)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("legacyCompressors")
  fun `test invalid rlp block`(version: BlobCompressorVersion, compressor: BlobCompressor) {
    compressor.reset()
    assertThat(compressor.version).isEqualTo(version)
    val block = Random.nextBytes(100)
    assertThrows<BlobCompressionException>("rlp: expected input list for types.extblock") {
      compressor.appendBlock(block)
    }
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("legacyCompressors")
  fun `test compression data limit exceeded`(version: BlobCompressorVersion, compressor: BlobCompressor) {
    compressor.reset()
    assertThat(compressor.version).isEqualTo(version)
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

  @ParameterizedTest(name = "{0}")
  @MethodSource("legacyCompressors")
  fun `test reset`(version: BlobCompressorVersion, compressor: BlobCompressor) {
    compressor.reset()
    assertThat(compressor.version).isEqualTo(version)
    val blocks = TEST_DATA.iterator()
    assertThat(compressor.getCompressedData()).isEmpty()
    var res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()
    assertThat(res.compressedSizeBefore).isZero()
    assertThat(res.compressedSizeAfter).isGreaterThan(0)
    assertThat(res.compressedSizeAfter).isEqualTo(compressor.getCompressedData().size)

    compressor.reset()

    assertThat(compressor.getCompressedData()).isEmpty()
    res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()
    assertThat(res.compressedSizeBefore).isZero()
    assertThat(res.compressedSizeAfter).isGreaterThan(0)
    assertThat(res.compressedSizeAfter).isEqualTo(compressor.getCompressedData().size)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("legacyCompressors")
  fun `test batches`(version: BlobCompressorVersion, compressor: BlobCompressor) {
    compressor.reset()
    assertThat(compressor.version).isEqualTo(version)
    val blocks = TEST_DATA.iterator()
    var res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()

    compressor.startNewBatch()

    res = compressor.appendBlock(blocks.next())
    assertThat(res.blockAppended).isTrue()
    assertThat(compressor.getCompressedData().size).isGreaterThan(0)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("legacyCompressors")
  fun `should calculate the compression size of raw data`(version: BlobCompressorVersion, compressor: BlobCompressor) {
    compressor.reset()
    assertThat(compressor.version).isEqualTo(version)
    val data = TEST_DATA.first()
    val compressedSize = compressor.compressedSize(data)
    assertThat(compressedSize).isBetween(1, data.size - 1)
  }
}
