package linea.domain

import linea.kotlin.encodeHex
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class BlockParameterTest {

  @Test
  fun `parse should parse valid tag`() {
    assertThat(BlockParameter.parse("earliest")).isEqualTo(BlockParameter.Tag.EARLIEST)
    assertThat(BlockParameter.parse("latest")).isEqualTo(BlockParameter.Tag.LATEST)
    assertThat(BlockParameter.parse("lAtEst")).isEqualTo(BlockParameter.Tag.LATEST)
  }

  @Test
  fun `parse should parse valid decimal number`() {
    assertThat(BlockParameter.parse("120")).isEqualTo(BlockParameter.BlockNumber(120UL))
  }

  @Test
  fun `parse should parse valid hexdecimal number`() {
    assertThat(BlockParameter.parse("0x78")).isEqualTo(BlockParameter.BlockNumber(120UL))
  }

  @Test
  fun `parse should parse block hash`() {
    val hashHex = "0x" + "ab".repeat(32)
    val parsed = BlockParameter.parse(hashHex) as BlockParameter.BlockHash
    assertThat(parsed.getHash()).isEqualTo(hashHex)
  }

  @Test
  fun `parse should parse block hash from encoded byte array`() {
    val hashHex = ByteArray(32) { index -> (index + 1).toByte() }.encodeHex(prefix = true)
    val parsed = BlockParameter.parse(hashHex) as BlockParameter.BlockHash
    assertThat(parsed.getHash()).isEqualTo(hashHex)
  }

  @Test
  fun `fromHash should accept bytes and hex string`() {
    val hash = ByteArray(32) { 1 }
    val hashHex = hash.encodeHex(prefix = true)
    assertThat(BlockParameter.fromHash(hash).getHash()).isEqualTo(hashHex)
    assertThat(BlockParameter.fromHash(hashHex).getHash()).isEqualTo(hashHex)
  }

  @Test
  fun `BlockHash should use content-based equality`() {
    val hashBytes = ByteArray(32) { 7 }
    val a = BlockParameter.fromHash(hashBytes)
    val b = BlockParameter.fromHash(hashBytes.copyOf())
    assertThat(a).isEqualTo(b)
    assertThat(a.hashCode()).isEqualTo(b.hashCode())
    assertThat(BlockParameter.fromHash(ByteArray(32) { 8 })).isNotEqualTo(a)
  }

  @Test
  fun `fromHash should reject invalid hash length`() {
    assertThatThrownBy { BlockParameter.fromHash(ByteArray(31)) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("32 bytes")
  }

  @Test
  fun `parse should throw InvalidArgument when invalid`() {
    assertThatThrownBy { BlockParameter.parse("invalid") }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("Invalid BlockParameter: invalid")
  }
}
