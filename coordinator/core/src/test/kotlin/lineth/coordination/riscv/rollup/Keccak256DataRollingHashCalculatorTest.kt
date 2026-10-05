package lineth.coordination.riscv.rollup

import linea.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class Keccak256DataRollingHashCalculatorTest {
  private val calculator = Keccak256DataRollingHashCalculator
  private val genesisDataRollingHash = ByteArray(32)
  private val chunkHashA = ByteArray(32) { 0x11 }
  private val chunkHashB = ByteArray(32) { 0x22 }

  @Test
  fun `fold computes keccak256 of parent concatenated with chunk hash`() {
    // keccak256 of 64 zero bytes
    assertThat(calculator.fold(ByteArray(32), ByteArray(32)))
      .isEqualTo("0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5".decodeHex())
  }

  @Test
  fun `fold returns 32 bytes`() {
    assertThat(calculator.fold(genesisDataRollingHash, chunkHashA)).hasSize(32)
  }

  @Test
  fun `fold is order sensitive`() {
    assertThat(calculator.fold(chunkHashA, chunkHashB))
      .isNotEqualTo(calculator.fold(chunkHashB, chunkHashA))
  }

  @Test
  fun `sequential folds are deterministic`() {
    fun foldAll() = listOf(chunkHashA, chunkHashB, chunkHashA)
      .fold(genesisDataRollingHash) { drh, chunkHash -> calculator.fold(drh, chunkHash) }

    val first = foldAll()
    assertThat(foldAll()).isEqualTo(first)
    assertThat(first).isNotEqualTo(calculator.fold(genesisDataRollingHash, chunkHashA))
  }

  @Test
  fun `fold rejects inputs that are not 32 bytes`() {
    assertThatThrownBy { calculator.fold(ByteArray(31), chunkHashA) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("parentDataRollingHash must be 32 bytes")
    assertThatThrownBy { calculator.fold(genesisDataRollingHash, ByteArray(33)) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("chunkHash must be 32 bytes")
  }
}
