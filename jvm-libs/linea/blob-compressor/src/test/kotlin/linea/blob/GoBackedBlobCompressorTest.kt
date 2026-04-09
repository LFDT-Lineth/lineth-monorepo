package linea.blob

import com.sun.jna.ptr.PointerByReference
import linea.blob.BlobCompressorSelectorTest.Companion.compressBlocks
import net.consensys.linea.nativecompressor.CompressorTestData
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.MethodSource
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.CyclicBarrier
import java.util.concurrent.TimeUnit
import kotlin.concurrent.atomics.AtomicReference
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.random.Random

class GoBackedBlobCompressorTest {
  companion object {
    private const val DATA_LIMIT = 24 * 1024
    private val TEST_DATA = CompressorTestData.blocksRlpEncoded

    @JvmStatic
    fun compressorVersions(): List<BlobCompressorVersion> =
      BlobCompressorVersion.entries.filter { it != BlobCompressorVersion.V2 }
  }

  private fun newCompressor(version: BlobCompressorVersion) = BlobCompressorFactory.getInstance(version, DATA_LIMIT)

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `test appendBlock with data within limit`(version: BlobCompressorVersion) {
    newCompressor(version).use { compressor ->
      val result = compressor.appendBlock(TEST_DATA.first())
      assertThat(result.blockAppended).isTrue
      assertThat(result.compressedSizeBefore).isZero()
      assertThat(result.compressedSizeAfter).isGreaterThan(0)
    }
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `test canAppend does not actually append`(version: BlobCompressorVersion) {
    newCompressor(version).use { compressor ->
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
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `test invalid rlp block`(version: BlobCompressorVersion) {
    newCompressor(version).use { compressor ->
      assertThrows<BlobCompressionException>("rlp: expected input list for types.extblock") {
        compressor.appendBlock(Random.nextBytes(100))
      }
    }
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `test reset`(version: BlobCompressorVersion) {
    newCompressor(version).use { compressor ->
      val blocks = TEST_DATA.iterator()
      assertThat(compressor.getCompressedData()).isEmpty()
      var res = compressor.appendBlock(blocks.next())
      assertThat(res.blockAppended).isTrue()
      assertThat(res.compressedSizeAfter).isGreaterThan(0)
      assertThat(res.compressedSizeAfter).isEqualTo(compressor.getCompressedData().size)

      compressor.reset()

      assertThat(compressor.getCompressedData()).isEmpty()
      res = compressor.appendBlock(blocks.next())
      assertThat(res.blockAppended).isTrue()
      assertThat(res.compressedSizeAfter).isEqualTo(compressor.getCompressedData().size)
    }
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `multiple instances have independent state`(version: BlobCompressorVersion) {
    newCompressor(version).use { c1 ->
      newCompressor(version).use { c2 ->
        // append one block to c1 only
        val res = c1.appendBlock(TEST_DATA.first())
        assertThat(res.blockAppended).isTrue()

        // c2 must still be empty
        assertThat(c2.getCompressedData()).isEmpty()

        // append a different block to c2
        val res2 = c2.appendBlock(TEST_DATA[1])
        assertThat(res2.blockAppended).isTrue()

        // c1 must be unaffected by c2's write
        assertThat(c1.getCompressedData().size).isEqualTo(res.compressedSizeAfter)
      }
    }
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `test compression data limit exceeded`(version: BlobCompressorVersion) {
    newCompressor(version).use { compressor ->
      var blocks = TEST_DATA.iterator()
      var result = compressor.appendBlock(blocks.next())
      assertThat(result.blockAppended).isTrue()
      while (result.blockAppended) {
        val blockRlp = blocks.next()
        val canAppend = compressor.canAppendBlock(blockRlp)
        result = compressor.appendBlock(blockRlp)
        assertThat(canAppend).isEqualTo(result.blockAppended)
        if (!blocks.hasNext()) {
          blocks = TEST_DATA.iterator()
        }
      }
      assertThat(result.blockAppended).isFalse()
      assertThat(result.compressedSizeBefore).isGreaterThan(0)
      assertThat(result.compressedSizeAfter).isEqualTo(result.compressedSizeBefore)
    }
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `test batches`(version: BlobCompressorVersion) {
    newCompressor(version).use { compressor ->
      val blocks = TEST_DATA.iterator()
      var res = compressor.appendBlock(blocks.next())
      assertThat(res.blockAppended).isTrue()

      compressor.startNewBatch()

      res = compressor.appendBlock(blocks.next())
      assertThat(res.blockAppended).isTrue()
      assertThat(compressor.getCompressedData().size).isGreaterThan(0)
    }
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `init with bad dictionary path returns native error message`(version: BlobCompressorVersion) {
    val lib = GoNativeBlobCompressorFactory.getInstance(version)
    val errOut = PointerByReference()
    val handle = lib.Init(DATA_LIMIT, "/nonexistent/dictionary.bin", errOut)
    assertThat(handle).isEqualTo(-1)
    assertThat(errOut.value?.getString(0)).isNotBlank()
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `close releases instance and a new one can be created`(version: BlobCompressorVersion) {
    newCompressor(version).use { c1 ->
      c1.appendBlock(TEST_DATA.first())
    }

    // must be able to create a fresh instance after closing
    newCompressor(version).use { c2 ->
      assertThat(c2.getCompressedData()).isEmpty()
    }
  }

  @OptIn(ExperimentalAtomicApi::class)
  @ParameterizedTest(name = "{0}")
  @MethodSource("compressorVersions")
  fun `compresses multiple blocks concurrently with multiple non legacy instances without exception`(
    version: BlobCompressorVersion,
  ) {
    val sampleBlocks = CompressorTestData.blocksRlpEncoded
    assertThat(sampleBlocks).isNotEmpty

    newCompressor(version).use { compressor1 ->
      newCompressor(version).use { compressor2 ->
        assertTrue(compressor1 != compressor2)

        val compressed1 = compressBlocks(compressor1, sampleBlocks)
        val compressed2 = compressBlocks(compressor2, sampleBlocks)

        assertThat(compressed1).isEqualTo(compressBlocks(compressor1, sampleBlocks))
        assertThat(compressed2).isEqualTo(compressBlocks(compressor2, sampleBlocks))

        val compressed1Parallel: AtomicReference<ByteArray> = AtomicReference(ByteArray(0))
        val compressed2Parallel: AtomicReference<ByteArray> = AtomicReference(ByteArray(0))

        val errors = CopyOnWriteArrayList<Throwable>()
        val startBarrier = CyclicBarrier(3)
        val doneLatch = CountDownLatch(2)

        val thread1 = Thread {
          try {
            startBarrier.await()
            compressed1Parallel.store(compressBlocks(compressor1, sampleBlocks))
          } catch (t: Throwable) {
            errors.add(t)
          } finally {
            doneLatch.countDown()
          }
        }

        val thread2 = Thread {
          try {
            startBarrier.await()
            compressed2Parallel.store(compressBlocks(compressor2, sampleBlocks))
          } catch (t: Throwable) {
            errors.add(t)
          } finally {
            doneLatch.countDown()
          }
        }

        thread1.start()
        thread2.start()
        startBarrier.await()
        assertTrue(doneLatch.await(10, TimeUnit.SECONDS), "compression threads did not complete in time")

        assertThat(compressed1Parallel.load()).isEqualTo(compressed1)
        assertThat(compressed2Parallel.load()).isEqualTo(compressed2)
        assertThat(errors).isEmpty()
      }
    }
  }
}
