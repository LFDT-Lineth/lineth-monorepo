package linea.kotlin

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.math.BigInteger

class TypingsExtensionsTest {
  @Test
  fun `BigInteger#ULong`() {
    assertThrows<NumberFormatException> { BigInteger.valueOf(-1).toULong() }
    assertThat(BigInteger.valueOf(0).toULong()).isEqualTo(0u.toULong())
    assertThat(BigInteger.valueOf(123).toULong()).isEqualTo(123u.toULong())
    assertThat(BigInteger(ULong.MAX_VALUE.toString(), 10).toULong()).isEqualTo(ULong.MAX_VALUE)
  }

  @Test
  fun `ULong#BigInteger`() {
    assertThat(0UL.toBigInteger()).isEqualTo(BigInteger.valueOf(0))
    assertThat(123UL.toBigInteger()).isEqualTo(BigInteger.valueOf(123))
    assertThat(ULong.MAX_VALUE.toBigInteger()).isEqualTo(BigInteger(ULong.MAX_VALUE.toString(), 10))
  }

  @Test
  fun `toIntervalString`() {
    assertThat((0..0).toIntervalString()).isEqualTo("[0..0]1")
    assertThat((0..1).toIntervalString()).isEqualTo("[0..1]2")
    assertThat((0..10).toIntervalString()).isEqualTo("[0..10]11")
    assertThat((0.0..10.0).toIntervalString()).isEqualTo("[0.0..10.0]11")

    assertThat((0..-1).toIntervalString()).isEqualTo("[0..-1]2")
    assertThat((0..-10).toIntervalString()).isEqualTo("[0..-10]11")
  }
}
