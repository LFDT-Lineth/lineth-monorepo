package net.consensys.zkevm.coordination.blob

import net.consensys.zkevm.ethereum.coordination.blob.BlobCompressor
import net.consensys.zkevm.ethereum.coordination.blob.FakeBlobCompressor
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.random.Random

class FakeBlobCompressorTest {

  @Test
  fun `test appendBlock with data within limit`() {
    val compressor = FakeBlobCompressor(100, 0.5)
    val block = ByteArray(50)
    val result = compressor.appendBlock(block)
    assertThat(result.blockAppended).isTrue
    assertThat(result.compressedSizeBefore).isEqualTo(0)
    assertThat(result.compressedSizeAfter).isEqualTo(25)
  }

  @Test
  fun `test appendBlock with data exceeding limit`() {
    val compressor = FakeBlobCompressor(100, 0.5)
    val block = ByteArray(1000)
    val result = compressor.appendBlock(block)
    assertThat(result.blockAppended).isFalse
    assertThat(result.compressedSizeBefore).isEqualTo(0)
    assertThat(result.compressedSizeAfter).isEqualTo(500)
  }

  @Test
  fun `test appendBlock with multiple blocks within limit`() {
    val compressor = FakeBlobCompressor(100, 0.5)
    val block1 = ByteArray(30)
    val block2 = ByteArray(40)
    compressor.appendBlock(block1)
    val result = compressor.appendBlock(block2)
    assertThat(result.blockAppended).isTrue
    assertThat(result.compressedSizeBefore).isEqualTo(15)
    assertThat(result.compressedSizeAfter).isEqualTo(35)

    val block3 = ByteArray(150)
    compressor.appendBlock(block3)
    val result2 = compressor.appendBlock(block3)
    assertThat(result2.blockAppended).isFalse
    assertThat(result2.compressedSizeBefore).isEqualTo(35)
    assertThat(result2.compressedSizeAfter).isEqualTo(110)
  }

  @Test
  fun `test appendBlock with multiple blocks within limit - compressor adds overhead`() {
    val compressor = FakeBlobCompressor(100, 1.1)
    val block1 = ByteArray(30)
    val block2 = ByteArray(40)
    assertThat(compressor.appendBlock(block1)).isEqualTo(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 0,
        compressedSizeAfter = 33,
      ),
    )

    assertThat(compressor.appendBlock(block2)).isEqualTo(
      BlobCompressor.AppendResult(
        blockAppended = true,
        compressedSizeBefore = 33,
        compressedSizeAfter = 77,
      ),
    )
    val block3 = ByteArray(31)
    assertThat(compressor.appendBlock(block3)).isEqualTo(
      BlobCompressor.AppendResult(
        blockAppended = false,
        compressedSizeBefore = 77,
        compressedSizeAfter = 111,
      ),
    )
  }

  @Test
  fun `test canAppendBlock returns true until limit is reached`() {
    val compressor = FakeBlobCompressor(100, 0.5)
    val block = ByteArray(50)
    assertThat(compressor.canAppendBlock(block)).isTrue()
    compressor.appendBlock(block) // 25 bytes
    compressor.appendBlock(block) // 50 bytes
    compressor.appendBlock(block) // 75 bytes
    assertThat(compressor.canAppendBlock(block)).isTrue()
    compressor.appendBlock(block) // 100 bytes
    assertThat(compressor.canAppendBlock(ByteArray(1))).isFalse()
  }

  @Test
  fun `test getCompressedData`() {
    val compressor = FakeBlobCompressor(100, 0.5)
    val block1 = Random.nextBytes(50)
    val block2 = Random.nextBytes(50)
    compressor.appendBlock(block1)
    compressor.appendBlock(block2)
    val compressedData = compressor.getCompressedData()
    assertThat(compressedData.size).isEqualTo(50)
  }

  @Test
  fun `test dataLimit less than 1 should throw exception`() {
    assertThatThrownBy { FakeBlobCompressor(0) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("dataLimit must be greater than 0")
  }

  @Test
  fun `test fakeCompressionRatio less than 1 should throw exception`() {
    assertThatThrownBy { FakeBlobCompressor(100, 0.0) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("fakeCompressionRatio must be greater than 0")
  }
}
