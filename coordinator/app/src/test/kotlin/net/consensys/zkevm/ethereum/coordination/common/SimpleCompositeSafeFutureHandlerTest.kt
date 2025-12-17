package net.consensys.zkevm.ethereum.coordination.common

import net.consensys.zkevm.ethereum.coordination.SimpleCompositeSafeFutureHandler
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture

class SimpleCompositeSafeFutureHandlerTest {
  @Test
  fun `should propagate argument to all the handlers exactly once`() {
    val handler1Calls = mutableListOf<String>()
    val handler2Calls = mutableListOf<String>()
    val handler3Calls = mutableListOf<String>()
    val handlers =
      listOf(
        { value: Long -> SafeFuture.completedFuture(handler1Calls.add("handler1:$value")) },
        { value: Long ->
          handler2Calls.add("handler2:$value")
          SafeFuture.failedFuture(RuntimeException("Handler 2 failed"))
        },
        { value: Long -> SafeFuture.completedFuture(handler3Calls.add("handler3:$value")) },
      )

    SimpleCompositeSafeFutureHandler(handlers).invoke(123)

    assertThat(handler1Calls).containsExactly("handler1:123")
    assertThat(handler2Calls).containsExactly("handler2:123")
    assertThat(handler3Calls).containsExactly("handler3:123")
  }

  @Test
  fun `should be resilient_or_not_to handler failure`() {
    val handler1Calls = mutableListOf<String>()
    val handler2Calls = mutableListOf<String>()
    val handler3Calls = mutableListOf<String>()
    val handlers =
      listOf(
        { value: Long -> SafeFuture.completedFuture(handler1Calls.add("handler1:$value")) },
        { value: Long ->
          handler2Calls.add("handler2:$value")
          throw RuntimeException("Forced error")
        },
        { value: Long -> SafeFuture.completedFuture(handler3Calls.add("handler3:$value")) },
      )

    SimpleCompositeSafeFutureHandler(handlers).invoke(123)

    assertThat(handler1Calls).containsExactly("handler1:123")
    assertThat(handler2Calls).containsExactly("handler2:123")
    assertThat(handler3Calls).containsExactly("handler3:123")
  }
}
