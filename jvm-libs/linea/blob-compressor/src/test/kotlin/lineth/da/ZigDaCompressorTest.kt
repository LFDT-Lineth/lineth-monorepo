package lineth.da

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.Callable
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors

class ZigDaCompressorTest {
  private val conflation by lazy { MainnetWindow.blocks(from = 0, count = 2) }

  @BeforeEach
  fun requireInputs() {
    ZigTestLibrary.assumeAvailable()
    MainnetWindow.assumeAvailable()
  }

  private fun compressor(chainId: ULong = MainnetWindow.chainId): DaCompressor =
    ZigTestLibrary.factory.create(DaCodecVersion.DEV, ZigTestLibrary.dictionary, chainId)

  private fun expectError(type: DaStreamErrorType, message: String, call: () -> Unit) {
    assertThatThrownBy(call)
      .isInstanceOfSatisfying(DaStreamException::class.java) {
        assertThat(it.errorType).isEqualTo(type)
        assertThat(it).hasMessageContaining(message)
      }
  }

  @Test
  fun `compresses a conflation into a frame smaller than its block RLPs`() {
    compressor().use { compressor ->
      val frame = compressor.compress(conflation)

      assertThat(compressor.version).isEqualTo(DaCodecVersion.DEV)
      assertThat(frame).isNotEmpty()
      assertThat(frame.size).isLessThan(conflation.sumOf { it.size })
    }
  }

  @Test
  fun `a frame is the same across calls and across compressors`() {
    compressor().use { first ->
      compressor().use { second ->
        val frame = first.compress(conflation)

        assertThat(first.compress(conflation)).isEqualTo(frame)
        assertThat(second.compress(conflation)).isEqualTo(frame)
        assertThat(first.compress(conflation.take(1))).isNotEqualTo(frame)
      }
    }
  }

  @Test
  fun `a block that cannot be decoded is INVALID_BLOCK_RLP with the message of the library`() {
    compressor().use { compressor ->
      val garbage = ByteArray(40) { it.toByte() }

      expectError(DaStreamErrorType.INVALID_BLOCK_RLP, "block 0 is malformed") {
        compressor.compress(listOf(garbage))
      }
      expectError(DaStreamErrorType.INVALID_BLOCK_RLP, "block 1 is malformed") {
        compressor.compress(listOf(conflation[0], garbage))
      }
    }
  }

  @Test
  fun `a legacy transaction signed for another chain is INVALID_BLOCK_RLP`() {
    compressor(chainId = 1UL).use { compressor ->
      expectError(DaStreamErrorType.INVALID_BLOCK_RLP, "a sender of block 0 cannot be recovered") {
        compressor.compress(listOf(MainnetWindow.blockWithLegacyTransaction()))
      }
    }
  }

  @Test
  fun `a conflation without blocks is INVALID_ARGUMENT`() {
    compressor().use { compressor ->
      expectError(DaStreamErrorType.INVALID_ARGUMENT, "no blocks") { compressor.compress(emptyList()) }
    }
  }

  @Test
  fun `bytes that are not a dictionary are INVALID_DICTIONARY at create`(@TempDir dir: Path) {
    val notADictionary = Files.write(dir.resolve("not-a-dictionary.bin"), ByteArray(1_000) { it.toByte() })

    expectError(DaStreamErrorType.INVALID_DICTIONARY, "not a zstd dictionary") {
      ZigTestLibrary.factory.create(DaCodecVersion.DEV, notADictionary, MainnetWindow.chainId)
    }
  }

  @Test
  fun `threads compressing on one compressor get the frame of a single thread`() {
    val threads = 8
    val pool = Executors.newFixedThreadPool(threads)
    try {
      compressor().use { compressor ->
        val expected = compressor.compress(conflation)

        val frames = pool.invokeAll(List(threads) { Callable { compressor.compress(conflation) } }).map { it.get() }

        assertThat(frames).hasSize(threads).allSatisfy { assertThat(it).isEqualTo(expected) }
      }
    } finally {
      pool.shutdownNow()
    }
  }

  @Test
  fun `calls after close are refused`() {
    val compressor = compressor()
    compressor.close()

    expectError(DaStreamErrorType.INVALID_ARGUMENT, "closed") { compressor.compress(conflation) }
    compressor.close()
  }

  @Test
  fun `threads closing one compressor at once all return`() {
    val threads = 8
    val pool = Executors.newFixedThreadPool(threads)
    try {
      val compressor = compressor()
      val ready = CountDownLatch(threads)
      val go = CountDownLatch(1)
      val closes = List(threads) {
        pool.submit {
          ready.countDown()
          go.await()
          compressor.close()
        }
      }
      ready.await()
      go.countDown()

      closes.forEach { it.get() }

      expectError(DaStreamErrorType.INVALID_ARGUMENT, "closed") { compressor.compress(conflation) }
    } finally {
      pool.shutdownNow()
    }
  }
}
