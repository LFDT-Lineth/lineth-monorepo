package linea.kotlin

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import kotlin.random.Random

class ByteArrayExtensionsTest {
  @Test
  fun `ByteArray#encodeHex`() {
    assertThat(byteArrayOf().encodeHex()).isEqualTo("0x")
    assertThat(byteArrayOf().encodeHex(false)).isEqualTo("")
    assertThat(byteArrayOf(0).encodeHex()).isEqualTo("0x00")
    assertThat(byteArrayOf(1).encodeHex()).isEqualTo("0x01")
    assertThat(byteArrayOf(0x12, 0x34, 0x56).encodeHex()).isEqualTo("0x123456")
    assertThat(byteArrayOf(0x12, 0x34, 0x56).encodeHex(false)).isEqualTo("123456")
  }

  @Test
  fun `ByteArray#assertSize`() {
    assertThatThrownBy {
      byteArrayOf(1, 2, 3).assertSize(2u, "shortNumber")
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("shortNumber expected to have 2 bytes, but got 3")

    assertThat(byteArrayOf(1, 2, 3).assertSize(3u)).isEqualTo(byteArrayOf(1, 2, 3))
  }

  @Test
  fun `ByteArray#assertIs32Bytes`() {
    assertThatThrownBy {
      byteArrayOf(1, 2, 3).assertIs32Bytes("hash")
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("hash expected to have 32 bytes, but got 3")
    ByteArray(32).assertIs32Bytes()
  }

  @Test
  fun `ByteArray#assertIs20Bytes`() {
    assertThatThrownBy {
      byteArrayOf(1, 2, 3).assertIs20Bytes("address")
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("address expected to have 20 bytes, but got 3")
    ByteArray(20).assertIs20Bytes()
  }

  @Test
  fun `ByteArray#setFirstByteToZero`() {
    assertThat(Random.Default.nextBytes(32).setFirstByteToZero()[0]).isEqualTo(0)
  }

  @Test
  fun `BigInteger#toKWei`() {
    assertThat(1_234_000.toBigInteger().toKWei().toUInt()).isEqualTo(1234u)
    assertThat(1_234_400.toBigInteger().toKWei().toUInt()).isEqualTo(1234u)
    assertThat(1_234_500.toBigInteger().toKWei().toUInt()).isEqualTo(1235u)
    assertThat(1_234_600.toBigInteger().toKWei().toUInt()).isEqualTo(1235u)
  }

  @Test
  fun `ByteArray#sliceOf`() {
    val bytes = Random.Default.nextBytes(64)
    bytes.sliceOf(sliceSize = 5, sliceNumber = 0).also {
      assertThat(it).hasSize(5)
      assertThat(it).isEqualTo(bytes.sliceArray(0..4))
    }
    bytes.sliceOf(sliceSize = 10, sliceNumber = 0).also {
      assertThat(it).hasSize(10)
      assertThat(it).isEqualTo(bytes.sliceArray(0..9))
    }
    bytes.sliceOf(sliceSize = 10, sliceNumber = 2).also {
      assertThat(it).hasSize(10)
      assertThat(it).isEqualTo(bytes.sliceArray(20..29))
    }
    bytes.sliceOf(sliceSize = 1, sliceNumber = 63, allowIncompleteLastSlice = false).also {
      assertThat(it).hasSize(1)
      assertThat(it).isEqualTo(bytes.sliceArray(63..63))
    }
    assertThatThrownBy {
      bytes.sliceOf(sliceSize = 1, sliceNumber = 64, allowIncompleteLastSlice = false)
    }
      .isInstanceOf(AssertionError::class.java)
      .hasMessage("slice 64..64 is out of array size=64")

    bytes.sliceOf(sliceSize = 10, sliceNumber = 6, allowIncompleteLastSlice = true).also {
      assertThat(it).hasSize(4)
      assertThat(it).isEqualTo(bytes.sliceArray(60..63))
    }

    assertThatThrownBy {
      bytes.sliceOf(sliceSize = 10, sliceNumber = 6, allowIncompleteLastSlice = false)
    }
      .isInstanceOf(AssertionError::class.java)
      .hasMessage("slice 60..69 is out of array size=64")

    assertThatThrownBy {
      bytes.sliceOf(sliceSize = 10, sliceNumber = 7)
    }
      .isInstanceOf(AssertionError::class.java)
      .hasMessage("slice 70..79 is out of array size=64")
  }

  @Test
  fun `ByteArray#sliceOf32Bytes`() {
    val bytes = Random.Default.nextBytes(64)
    assertThat(bytes.sliceOf32(0)).isEqualTo(bytes.sliceArray(0..31))
    assertThat(bytes.sliceOf32(1)).isEqualTo(bytes.sliceArray(32..63))
    assertThatThrownBy {
      Random.Default.nextBytes(64 + 16).sliceOf32(sliceNumber = 2)
    }
      .isInstanceOf(AssertionError::class.java)
      .hasMessage("slice 64..95 is out of array size=80")
  }

  @Test
  fun toULongFromLast8Bytes() {
    assertThat(byteArrayOf(0x00).toULongFromLast8Bytes(lenient = true)).isEqualTo(0uL)
    assertThat(byteArrayOf(0x01).toULongFromLast8Bytes(lenient = true)).isEqualTo(1uL)
    val max = ByteArray(32) { 0xff.toByte() }
    assertThat(max.toULongFromLast8Bytes()).isEqualTo(ULong.MAX_VALUE)
    assertThat(max.apply { set(31, 0xfe.toByte()) }.toULongFromLast8Bytes()).isEqualTo(ULong.MAX_VALUE - 1UL)
  }

  @Nested
  inner class PadLeft {
    @Test
    fun `should return the same array if size is already equal to target size`() {
      val original = byteArrayOf(0x1, 0x2, 0x3)
      val result = original.padLeft(3)
      assertThat(result).isEqualTo(original)
    }

    @Test
    fun `should return the same array if size is larger than target size`() {
      val original = byteArrayOf(0x1, 0x2, 0x3)
      val result = original.padLeft(2)
      assertThat(result).isEqualTo(original)
    }

    @Test
    fun `should pad with default value 0x0 when target size is larger`() {
      val original = byteArrayOf(0x1, 0x2, 0x3)
      val result = original.padLeft(6)
      assertThat(result).isEqualTo(byteArrayOf(0x0, 0x0, 0x0, 0x1, 0x2, 0x3))
    }

    @Test
    fun `should pad with custom value when target size is larger`() {
      val original = byteArrayOf(0x1, 0x2, 0x3)
      val result = original.padLeft(6, padding = 0xFF.toByte())
      assertThat(result).isEqualTo(byteArrayOf(0xFF.toByte(), 0xFF.toByte(), 0xFF.toByte(), 0x1, 0x2, 0x3))
    }

    @Test
    fun `should handle empty array and pad to target size`() {
      val original = byteArrayOf()
      val result = original.padLeft(4)
      assertThat(result).isEqualTo(byteArrayOf(0x0, 0x0, 0x0, 0x0))
    }

    @Test
    fun `should handle empty array and pad with custom value`() {
      val original = byteArrayOf()
      val result = original.padLeft(4, padding = 0xAA.toByte())
      assertThat(result).isEqualTo(byteArrayOf(0xAA.toByte(), 0xAA.toByte(), 0xAA.toByte(), 0xAA.toByte()))
    }
  }
}
