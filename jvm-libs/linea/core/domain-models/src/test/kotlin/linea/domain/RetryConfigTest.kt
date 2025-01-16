package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class RetryConfigTest {
  @Test
  fun `should support no retries`() {
    val notRetryConfig = RetryConfig(maxRetries = 0u)
    assertThat(notRetryConfig.isRetryDisabled).isTrue()
    assertThat(notRetryConfig.isRetryEnabled).isFalse()
    assertThat(notRetryConfig.timeout).isNull()
    assertThat(notRetryConfig.maxRetries).isEqualTo(0u)
    assertThat(notRetryConfig.failuresWarningThreshold).isEqualTo(0u)
    assertThat(RetryConfig.noRetries).isEqualTo(notRetryConfig)
  }

  @Test
  fun `should support retries with timeout only`() {
    val notRetryConfig = RetryConfig(timeout = 2.seconds)
    assertThat(notRetryConfig.isRetryDisabled).isFalse()
    assertThat(notRetryConfig.isRetryEnabled).isTrue()
    assertThat(notRetryConfig.timeout).isEqualTo(2.seconds)
    assertThat(notRetryConfig.maxRetries).isNull()
    assertThat(notRetryConfig.failuresWarningThreshold).isEqualTo(0u)
  }

  @Test
  fun `should support retries with maxRetries only`() {
    val notRetryConfig = RetryConfig(maxRetries = 10u)
    assertThat(notRetryConfig.isRetryDisabled).isFalse()
    assertThat(notRetryConfig.isRetryEnabled).isTrue()
    assertThat(notRetryConfig.timeout).isNull()
    assertThat(notRetryConfig.maxRetries).isEqualTo(10u)
    assertThat(notRetryConfig.failuresWarningThreshold).isEqualTo(0u)
  }

  @Test
  fun `should support retries with timeout and maxRetries`() {
    val notRetryConfig = RetryConfig(maxRetries = 10u, timeout = 20.seconds, backoffDelay = 1500.milliseconds)
    assertThat(notRetryConfig.isRetryDisabled).isFalse()
    assertThat(notRetryConfig.isRetryEnabled).isTrue()
    assertThat(notRetryConfig.timeout).isEqualTo(20.seconds)
    assertThat(notRetryConfig.maxRetries).isEqualTo(10u)
    assertThat(notRetryConfig.backoffDelay).isEqualTo(1500.milliseconds)
    assertThat(notRetryConfig.failuresWarningThreshold).isEqualTo(0u)
  }

  @Test
  fun `should throw exception when timeout is less than 1ms`() {
    assertThatThrownBy {
      RetryConfig(timeout = 0.milliseconds)
    }.isInstanceOf(IllegalArgumentException::class.java)
  }
}
