package net.consensys.linea.blob

import net.consensys.decodeHex
import net.consensys.encodeHex
import net.consensys.linea.nativecompressor.CompressorTestData
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import java.util.Base64
import kotlin.random.Random

class GoNativeCompressorAndShnarfCalculatorIntTest {
  private val DATA_LIMIT = 128 * 1024
  private lateinit var compressor: GoNativeBlobCompressor
  private lateinit var shnarfCalculator: GoNativeBlobShnarfCalculator

  private fun encodeBase64(bytes: ByteArray): String {
    return Base64.getEncoder().encodeToString(bytes)
  }

  @Nested
  inner class CompressorV0 {
    @BeforeEach
    fun beforeEach() {
      compressor = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V0_1_0)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }
      shnarfCalculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V0_1_0)
    }

    @Test
    fun `should compress and shnarf with eip4844 disabled`() {
      testsCompressionAndsShnarfCalculationWithEip4844Disabled(compressor, shnarfCalculator)
    }

    @Test
    fun `should compress and shnarf with eip4844 enabled`() {
      testsCompressionAndsShnarfCalculationWithEip4844Enabled(compressor, shnarfCalculator)
    }

    @Test
    fun `compressed size estimation should be consistent with blob maker output`() {
      testCompressedSizeEstimationIsConsistentWithBlobMakerOutput()
    }
  }

  @Nested
  inner class CompressorV1 {
    @BeforeEach
    fun beforeEach() {
      compressor = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V1_0_1)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }
      shnarfCalculator = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V1_0_1)
    }

    @Test
    fun `should compress and shnarf with eip4844 disabled`() {
      testsCompressionAndsShnarfCalculationWithEip4844Disabled(compressor, shnarfCalculator)
    }

    @Test
    fun `should compress and shnarf with eip4844 enabled`() {
      testsCompressionAndsShnarfCalculationWithEip4844Enabled(compressor, shnarfCalculator)
    }

    @Test
    fun `compressed size estimation should be consistent with blob maker output`() {
      testCompressedSizeEstimationIsConsistentWithBlobMakerOutput()
    }
  }

  @Nested
  inner class CompressorSupportsMultipleInstances {
    @Test
    fun `should support multiple instances`() {
      val compressorInstance1 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V0_1_0)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }

      val compressorInstance2 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V1_0_1)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }
      val block0 = CompressorTestData.blocksRlpEncoded[0]
      val block1 = CompressorTestData.blocksRlpEncoded[1]
      assertThat(block0.encodeHex()).isNotEqualTo(block1.encodeHex())

      compressorInstance1.Write(block0, block0.size)
      assertThat(compressorInstance1.Len()).isGreaterThan(0)
      assertThat(compressorInstance2.Len()).isEqualTo(0)

      compressorInstance1.Reset()
      compressorInstance2.Write(block1, block1.size)
      assertThat(compressorInstance1.Len()).isEqualTo(0)
      assertThat(compressorInstance2.Len()).isGreaterThan(0)
    }
  }

  @Nested
  inner class CompressorV0AndV1CoExistence {
    @Test
    fun `should support multiple versions with isolated state`() {
      val compressorV0 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V0_1_0)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }

      val compressorV1 = GoNativeBlobCompressorFactory.getInstance(BlobCompressorVersion.V1_0_1)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }
      val block0 = CompressorTestData.blocksRlpEncoded[0]
      val block1 = CompressorTestData.blocksRlpEncoded[1]
      assertThat(block0.encodeHex()).isNotEqualTo(block1.encodeHex())

      compressorV0.Write(block0, block0.size)
      assertThat(compressorV0.Len()).isGreaterThan(0)
      assertThat(compressorV1.Len()).isEqualTo(0)

      compressorV0.Reset()
      compressorV1.Write(block1, block1.size)
      assertThat(compressorV0.Len()).isEqualTo(0)
      assertThat(compressorV1.Len()).isGreaterThan(0)
    }
  }

  fun testsCompressionAndsShnarfCalculationWithEip4844Disabled(
    compressor: GoNativeBlobCompressor,
    shnarfCalculator: GoNativeBlobShnarfCalculator
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
    shnarfCalculator: GoNativeBlobShnarfCalculator
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
    resultAsserterFn: (CalculateShnarfResult) -> Unit
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
      conflationOrderUpperBoundaries = longArrayOf(10, 20)
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

  fun testCompressedSizeEstimationIsConsistentWithBlobMakerOutput() {
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
