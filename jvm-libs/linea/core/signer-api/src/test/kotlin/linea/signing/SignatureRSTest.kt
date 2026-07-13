package linea.signing

import org.assertj.core.api.Assertions.assertThatCode
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.math.BigInteger

class SignatureRSTest {
  @Test
  fun `accepts canonical signatures up to and including half the curve order`() {
    assertThatCode { SignatureRS(BigInteger.ONE, BigInteger.ONE) }.doesNotThrowAnyException()
    assertThatCode { SignatureRS(Secp256k1.N - BigInteger.ONE, Secp256k1.HALF_N) }.doesNotThrowAnyException()
  }

  @Test
  fun `rejects high-s and out-of-range values`() {
    assertThatThrownBy { SignatureRS(BigInteger.ONE, Secp256k1.HALF_N + BigInteger.ONE) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("low-s")
    assertThatThrownBy { SignatureRS(BigInteger.ZERO, BigInteger.ONE) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { SignatureRS(Secp256k1.N, BigInteger.ONE) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { SignatureRS(BigInteger.ONE, BigInteger.ZERO) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }
}
