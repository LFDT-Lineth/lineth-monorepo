package net.consensys.linea.blob

import linea.blob.BlobCompressorVersion
import linea.blob.CalculateShnarfResult
import linea.blob.GoNativeBlobCompressor
import linea.blob.GoNativeBlobCompressorFactory
import linea.blob.GoNativeBlobShnarfCalculator
import linea.blob.GoNativeShnarfCalculatorFactory
import linea.blob.ShnarfCalculatorVersion
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import net.consensys.linea.nativecompressor.CompressorTestData
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.util.Base64
import java.util.stream.Stream
import kotlin.random.Random

class GoNativeCompressorAndShnarfCalculatorIntTest {
  private fun encodeBase64(bytes: ByteArray): String {
    return Base64.getEncoder().encodeToString(bytes)
  }

  companion object {
    private const val DATA_LIMIT = 128 * 1024

    @JvmStatic
    fun enumerateValidBlobCompressorAndShnarfCalculatorPair(): Stream<Arguments> {
      val compressorV1 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V1_2)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }
      val compressorV2 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V2)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }
      val compressorV3 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V3)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }
      val shnarfCalculatorV1 = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V1_2)
      val shnarfCalculatorV3 = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V3)

      return Stream.of(
        Arguments.of(compressorV1, shnarfCalculatorV1),
        Arguments.of(compressorV2, shnarfCalculatorV1),
        Arguments.of(compressorV3, shnarfCalculatorV3),
      )
    }
  }

  @ParameterizedTest
  @MethodSource("enumerateValidBlobCompressorAndShnarfCalculatorPair")
  fun `should compress and shnarf with eip4844 disabled`(
    compressor: GoNativeBlobCompressor,
    shnarfCalculator: GoNativeBlobShnarfCalculator,
  ) {
    testsCompressionAndsShnarfCalculationWithEip4844Disabled(compressor, shnarfCalculator)
  }

  @ParameterizedTest
  @MethodSource("enumerateValidBlobCompressorAndShnarfCalculatorPair")
  fun `should compress and shnarf with eip4844 enabled`(
    compressor: GoNativeBlobCompressor,
    shnarfCalculator: GoNativeBlobShnarfCalculator,
  ) {
    testsCompressionAndsShnarfCalculationWithEip4844Enabled(compressor, shnarfCalculator)
  }

  @ParameterizedTest
  @MethodSource("enumerateValidBlobCompressorAndShnarfCalculatorPair")
  fun `compressed size estimation should be consistent with blob maker output`(
    compressor: GoNativeBlobCompressor,
  ) {
    testCompressedSizeEstimationIsConsistentWithBlobMakerOutput(compressor)
  }

  @Nested
  inner class CompressorSupportsMultipleInstances {
    @Test
    fun `should support multiple instances`() {
      val compressorInstance1 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V1_2)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }

      val compressorInstance2 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V2)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }

      val compressorInstance3 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V3)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }

      val block0 = CompressorTestData.blocksRlpEncoded[0]
      val block1 = CompressorTestData.blocksRlpEncoded[1]
      val block2 = CompressorTestData.blocksRlpEncoded[2]
      assertThat(block0.encodeHex()).isNotEqualTo(block1.encodeHex())
      assertThat(block0.encodeHex()).isNotEqualTo(block2.encodeHex())
      assertThat(block1.encodeHex()).isNotEqualTo(block2.encodeHex())

      compressorInstance1.Write(block0, block0.size)
      assertThat(compressorInstance1.Len()).isGreaterThan(0)
      assertThat(compressorInstance2.Len()).isEqualTo(0)
      assertThat(compressorInstance3.Len()).isEqualTo(0)

      compressorInstance1.Reset()
      compressorInstance2.Write(block1, block1.size)
      assertThat(compressorInstance1.Len()).isEqualTo(0)
      assertThat(compressorInstance2.Len()).isGreaterThan(0)
      assertThat(compressorInstance3.Len()).isEqualTo(0)

      compressorInstance2.Reset()
      compressorInstance3.Write(block1, block1.size)
      assertThat(compressorInstance1.Len()).isEqualTo(0)
      assertThat(compressorInstance2.Len()).isEqualTo(0)
      assertThat(compressorInstance3.Len()).isGreaterThan(0)
    }
  }

  fun testsCompressionAndsShnarfCalculationWithEip4844Disabled(
    compressor: GoNativeBlobCompressor,
    shnarfCalculator: GoNativeBlobShnarfCalculator,
  ) {
    testsCompressionAndsShnarfCalculation(compressor, shnarfCalculator, false) { result ->
      assertThat(result.commitment.decodeHex()).hasSize(0)
      assertThat(result.kzgProofContract).isNotNull()
      assertThat(result.kzgProofContract.decodeHex()).hasSize(0)
      assertThat(result.kzgProofSideCar).isNotNull()
      assertThat(result.kzgProofSideCar.decodeHex()).hasSize(0)
    }
  }

  fun testsCompressionAndsShnarfCalculationWithEip4844Enabled(
    compressor: GoNativeBlobCompressor,
    shnarfCalculator: GoNativeBlobShnarfCalculator,
  ) {
    testsCompressionAndsShnarfCalculation(compressor, shnarfCalculator, true) { result ->
      assertThat(result.commitment.decodeHex()).hasSize(48)
      assertThat(result.kzgProofContract).isNotNull()
      assertThat(result.kzgProofContract.decodeHex()).hasSize(48)
      assertThat(result.kzgProofSideCar).isNotNull()
      assertThat(result.kzgProofSideCar.decodeHex()).hasSize(48)
    }
  }

  fun testsCompressionAndsShnarfCalculation(
    compressor: GoNativeBlobCompressor,
    shnarfCalculator: GoNativeBlobShnarfCalculator,
    eip4844Enabled: Boolean,
    resultAsserterFn: (CalculateShnarfResult) -> Unit,
  ) {
    val block = CompressorTestData.blocksRlpEncoded.first()
    assertTrue(compressor.Write(block, block.size))
    assertThat(compressor.Error()).isNullOrEmpty()

    val compressedData = ByteArray(compressor.Len())
    compressor.Bytes(compressedData)

    val result: CalculateShnarfResult = shnarfCalculator.CalculateShnarf(
      eip4844Enabled = eip4844Enabled,
      compressedData = encodeBase64(compressedData),
      parentStateRootHash = Random.nextBytes(32).encodeHex(),
      finalStateRootHash = Random.nextBytes(32).encodeHex(),
      prevShnarf = Random.nextBytes(32).encodeHex(),
      conflationOrderStartingBlockNumber = 1,
      conflationOrderUpperBoundariesLen = 2,
      conflationOrderUpperBoundaries = longArrayOf(10, 20),
    )
    assertThat(result).isNotNull
    assertThat(result.errorMessage).isEmpty()
    assertThat(result.commitment).isNotNull()
    assertThat(result.dataHash).isNotNull
    assertThat(result.dataHash.decodeHex()).hasSize(32)
    assertThat(result.snarkHash).isNotNull
    assertThat(result.snarkHash.decodeHex()).hasSize(32)
    assertThat(result.expectedX).isNotNull()
    assertThat(result.expectedX.decodeHex()).hasSize(32)
    assertThat(result.expectedY).isNotNull()
    assertThat(result.expectedY.decodeHex()).hasSize(32)
    assertThat(result.expectedShnarf).isNotNull
    assertThat(result.expectedShnarf.decodeHex()).hasSize(32)
    resultAsserterFn(result)
  }

  fun testCompressedSizeEstimationIsConsistentWithBlobMakerOutput(compressor: GoNativeBlobCompressor) {
    val block = CompressorTestData.blocksRlpEncoded.first()

    // Write the block to the blob maker and get the effective compressed size
    assertTrue(compressor.Write(block, block.size))
    assertThat(compressor.Error()).isNullOrEmpty()
    val compressedSizeWithHeader = compressor.Len()
    assertTrue(compressedSizeWithHeader > 0)
    compressor.Reset()

    // Perform the same operation using the WorstCompressedBlockSize function
    val compressedSize = compressor.WorstCompressedBlockSize(block, block.size)
    assertTrue(compressedSize < block.size)
    assertTrue(compressedSize > 0)

    assertTrue(compressedSizeWithHeader > compressedSize)

    // sanity checks, these may not be "future proof" tests
    // but they are useful to ensure that the blob maker is behaving as expected
    // and that the code is correctly interfacing with it
    val estimatedHeaderSizePacked = 64 // technically, unpacked it's like ~40 bytes

    // min compressed size should always be strictly bigger than the size returned
    // by the blob maker minus the header size
    assertThat(compressedSizeWithHeader - estimatedHeaderSizePacked).isLessThanOrEqualTo(compressedSize)
  }
}
