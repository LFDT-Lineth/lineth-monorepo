package linea.kotlin
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ULongExtensionsTest {
  @Test
  fun `hasSequentialElements should return true for an empty list`() {
    val list = emptyList<ULong>()
    assertThat(list.hasSequentialElements()).isTrue()
  }

  @Test
  fun `hasSequentialElements should return true for a list with one element`() {
    val list = listOf(1UL)
    assertThat(list.hasSequentialElements()).isTrue()
  }

  @Test
  fun `hasSequentialElements should return true for a list with sequential elements`() {
    val list = listOf(1UL, 2UL, 3UL, 4UL, 5UL)
    assertThat(list.hasSequentialElements()).isTrue()
  }

  @Test
  fun `hasSequentialElements should return false for a list with non-sequential elements`() {
    val list = listOf(1UL, 3UL, 2UL, 5UL, 4UL)
    assertThat(list.hasSequentialElements()).isFalse()
  }

  @Test
  fun `hasSequentialElements should return false for a list with gaps`() {
    val list = listOf(1UL, 2UL, 4UL, 5UL)
    assertThat(list.hasSequentialElements()).isFalse()
  }
}
