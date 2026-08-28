package linea.kotlin
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class UIntExtensionsTest {
  @Test
  fun `minusCoercingUnderflow should return the difference when minuend is greater than subtrahend`() {
    assertThat(10U.minusCoercingUnderflow(5U)).isEqualTo(5U)
  }

  @Test
  fun `minusCoercingUnderflow should return zero when minuend is less than subtrahend`() {
    assertThat(5U.minusCoercingUnderflow(10U)).isEqualTo(0U)
  }

  @Test
  fun `minusCoercingUnderflow should return zero when minuend is equal to subtrahend`() {
    assertThat(5U.minusCoercingUnderflow(5U)).isEqualTo(0U)
  }
}
