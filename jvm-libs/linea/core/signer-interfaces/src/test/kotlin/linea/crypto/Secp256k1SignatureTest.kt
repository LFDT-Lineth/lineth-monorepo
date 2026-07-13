package linea.crypto

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatCode
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.math.BigInteger

class Secp256k1SignatureTest {
  @Test
  fun `accepts canonical signatures up to and including half the curve order`() {
    assertThatCode { Secp256k1Signature(BigInteger.ONE, BigInteger.ONE) }.doesNotThrowAnyException()
    assertThatCode { Secp256k1Signature(Secp256k1.N - BigInteger.ONE, Secp256k1.HALF_N) }.doesNotThrowAnyException()
  }

  @Test
  fun `rejects high-s and out-of-range values`() {
    assertThatThrownBy { Secp256k1Signature(BigInteger.ONE, Secp256k1.HALF_N + BigInteger.ONE) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("low-s")
    assertThatThrownBy { Secp256k1Signature(BigInteger.ZERO, BigInteger.ONE) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { Secp256k1Signature(Secp256k1.N, BigInteger.ONE) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { Secp256k1Signature(BigInteger.ONE, BigInteger.ZERO) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `round-trips through the 64-byte encoding, left-padding short components`() {
    val signature = Secp256k1Signature(BigInteger.ONE, Secp256k1.HALF_N)
    val bytes = signature.toBytes()
    assertThat(bytes).hasSize(Secp256k1Signature.SIZE_BYTES)
    assertThat(bytes.copyOfRange(0, 31)).containsOnly(0)
    assertThat(Secp256k1Signature.fromBytes(bytes)).isEqualTo(signature)
  }

  @Test
  fun `fromBytes rejects wrong lengths and non-canonical encodings`() {
    assertThatThrownBy { Secp256k1Signature.fromBytes(ByteArray(65)) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("64 bytes")
    val highS = Secp256k1Signature(BigInteger.ONE, BigInteger.ONE).toBytes()
    (Secp256k1.HALF_N + BigInteger.ONE).toByteArray().takeLast(32).forEachIndexed { i, b -> highS[32 + i] = b }
    assertThatThrownBy { Secp256k1Signature.fromBytes(highS) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("low-s")
  }
}
