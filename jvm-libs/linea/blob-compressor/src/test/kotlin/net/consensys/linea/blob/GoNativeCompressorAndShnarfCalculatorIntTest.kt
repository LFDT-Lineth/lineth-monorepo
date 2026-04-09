package net.consensys.linea.blob

import com.sun.jna.ptr.PointerByReference
import linea.blob.BlobCompressor
import linea.blob.BlobCompressorFactory
import linea.blob.BlobCompressorVersion
import linea.blob.CalculateShnarfResult
import linea.blob.GoNativeBlobCompressor
import linea.blob.GoNativeBlobCompressorFactory
import linea.blob.GoNativeBlobCompressorV4
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
import org.junit.jupiter.params.provider.ValueSource
import java.util.Base64
import java.util.stream.IntStream.range
import kotlin.random.Random

class GoNativeCompressorAndShnarfCalculatorIntTest {
  private fun encodeBase64(bytes: ByteArray): String {
    return Base64.getEncoder().encodeToString(bytes)
  }

  companion object {
    private const val DATA_LIMIT = 128 * 1024
  }

  @ParameterizedTest()
  @ValueSource(booleans = [false, true])
  fun `should compress and shnarf with eip4844 enabled-disabled`(eip4844Enabled: Boolean) {
    val compressorV3 = BlobCompressorFactory.getInstance(BlobCompressorVersion.V3, DATA_LIMIT)
    val compressorV4 = BlobCompressorFactory.getInstance(BlobCompressorVersion.V4, DATA_LIMIT)

    val shnarfCalculatorV3 = GoNativeShnarfCalculatorFactory.getInstance(ShnarfCalculatorVersion.V3)

    val compressorAndShnarfCalculator = listOf(
      Pair(compressorV3, shnarfCalculatorV3),
      Pair(compressorV4, shnarfCalculatorV3),
    )

    compressorAndShnarfCalculator.forEach { (compressor, shnarfCalculator) ->
      testsCompressionAndsShnarfCalculation(
        compressor = compressor,
        shnarfCalculator = shnarfCalculator,
        eip4844Enabled = eip4844Enabled,
      ) { result ->
        val size = if (eip4844Enabled) 48 else 0
        assertThat(result.commitment.decodeHex()).hasSize(size)
        assertThat(result.kzgProofContract).isNotNull()
        assertThat(result.kzgProofContract.decodeHex()).hasSize(size)
        assertThat(result.kzgProofSideCar).isNotNull()
        assertThat(result.kzgProofSideCar.decodeHex()).hasSize(size)
      }
    }
  }

  @Test
  fun `compressed size estimation should be consistent with blob maker output`() {
    val legacyCompressor = GoNativeBlobCompressorFactory.getLegacyInstance(BlobCompressorVersion.V2)
      .apply {
        this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
        this.Reset()
      }
    testCompressedSizeEstimationIsConsistentWithBlobMakerOutput(legacyCompressor)

    BlobCompressorVersion.entries.filter { it != BlobCompressorVersion.V2 }.forEach { version ->
      val compressor = GoNativeBlobCompressorFactory.getInstance(version)
      val errOut = PointerByReference()
      val handle = compressor.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString(), errOut)
      if (handle == -1) {
        throw InstantiationException(
          errOut.value?.getString(0) ?: "Failed to initialize compressor",
        )
      }
      testCompressedSizeEstimationIsConsistentWithBlobMakerOutput(compressor, handle)
    }
  }

  @Nested
  inner class CompressorSupportsMultipleInstances {
    @Test
    fun `should support multiple instances`() {
      val legacyCompressor = GoNativeBlobCompressorFactory.getLegacyInstance(BlobCompressorVersion.V2)
        .apply {
          this.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString())
          this.Reset()
        }

      val compressorsAndHandles = BlobCompressorVersion.entries
        .filter { it != BlobCompressorVersion.V2 }
        .map { version ->
          val compressor = GoNativeBlobCompressorFactory.getInstance(version)
          val errOut = PointerByReference()
          val handle = compressor.Init(DATA_LIMIT, GoNativeBlobCompressorFactory.dictionaryPath.toString(), errOut)
          if (handle == -1) {
            throw InstantiationException(
              errOut.value?.getString(0) ?: "Failed to initialize compressor",
            )
          }
          Pair(compressor, handle)
        }

      val block0 = CompressorTestData.blocksRlpEncoded[0]
      val block1 = CompressorTestData.blocksRlpEncoded[1]
      val block2 = CompressorTestData.blocksRlpEncoded[2]
      assertThat(block0.encodeHex()).isNotEqualTo(block1.encodeHex())
      assertThat(block0.encodeHex()).isNotEqualTo(block2.encodeHex())
      assertThat(block1.encodeHex()).isNotEqualTo(block2.encodeHex())

      assertThat(legacyCompressor.Len()).isEqualTo(0)
      compressorsAndHandles.forEach { (compressor, handle) ->
        assertThat(compressor.Len(handle)).isEqualTo(0)
      }

      legacyCompressor.Write(block1, block1.size)
      assertThat(legacyCompressor.Len()).isGreaterThan(0)
      compressorsAndHandles.forEach { (compressor, handle) ->
        assertThat(compressor.Len(handle)).isEqualTo(0)
      }

      legacyCompressor.Reset()

      for (i in range(0, compressorsAndHandles.size)) {
        val (compressor, handle) = compressorsAndHandles[i]
        compressor.Write(handle, block2, block2.size)
        assertThat(compressor.Len(handle)).isGreaterThan(0)

        assertThat(legacyCompressor.Len()).isEqualTo(0)
        for (j in range(0, compressorsAndHandles.size)) {
          if (i != j) {
            val (otherCompressor, otherHandle) = compressorsAndHandles[j]
            assertThat(otherCompressor.Len(otherHandle)).isEqualTo(0)
          }
        }
        compressor.Reset(handle)
      }
    }
  }

  fun testsCompressionAndsShnarfCalculation(
    compressor: BlobCompressor,
    shnarfCalculator: GoNativeBlobShnarfCalculator,
    eip4844Enabled: Boolean,
    resultAsserterFn: (CalculateShnarfResult) -> Unit,
  ) {
    val appendResult = compressor.appendBlock(CompressorTestData.blocksRlpEncoded.first())
    assertTrue(appendResult.blockAppended)
    val compressedData = compressor.getCompressedData()

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

  fun testCompressedSizeEstimationIsConsistentWithBlobMakerOutput(compressor: GoNativeBlobCompressorV4, handle: Int) {
    val block = CompressorTestData.blocksRlpEncoded.first()

    // Write the block to the blob maker and get the effective compressed size
    assertTrue(compressor.Write(handle, block, block.size))
    assertThat(compressor.Error(handle)).isNullOrEmpty()
    val compressedSizeWithHeader = compressor.Len(handle)
    assertTrue(compressedSizeWithHeader > 0)
    compressor.Reset(handle)

    // Perform the same operation using the WorstCompressedBlockSize function
    val compressedSize = compressor.WorstCompressedBlockSize(handle, block, block.size)
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
