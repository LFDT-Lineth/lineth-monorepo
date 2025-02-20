package linea.kotlin

import kotlinx.datetime.Instant
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.nanoseconds

class InstantExtensionsTest {

  @Test
  fun `trimToMinutePrecision should trim to the start of the minute`() {
    val instant = Instant.parse("2023-10-01T12:34:56.789Z")
    val expected = Instant.parse("2023-10-01T12:34:00Z")

    val result = instant.trimToMinutePrecision()

    assertThat(result).isEqualTo(expected)
  }

  @Test
  fun `trimToSecondPrecision should trim to the start of the second`() {
    val instant = Instant.parse("2023-10-01T12:34:56.789Z")
    val expected = Instant.parse("2023-10-01T12:34:56Z")

    val result = instant.trimToSecondPrecision()

    assertThat(result).isEqualTo(expected)
  }

  @Test
  fun `trimToMillisecondPrecision should trim to the start of the millisecond`() {
    val instant = Instant.parse("2023-10-01T12:34:56.789Z").plus(5.nanoseconds)
    val expected = Instant.parse("2023-10-01T12:34:56.789Z")

    val result = instant.trimToMillisecondPrecision()

    assertThat(result).isEqualTo(expected)
  }
}
