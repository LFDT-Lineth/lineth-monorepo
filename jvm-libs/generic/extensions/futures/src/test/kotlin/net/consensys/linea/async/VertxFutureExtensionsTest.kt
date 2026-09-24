package net.consensys.linea.async

import io.vertx.core.Future
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException

@ExtendWith(VertxExtension::class)
class VertxFutureExtensionsTest {

  @Test
  fun `SafeFuture toVertxFuture propagates a successful result`(vertx: Vertx) {
    val safeFuture = SafeFuture.completedFuture("result")

    val vertxFuture = safeFuture.toVertxFuture()

    assertThat(vertxFuture.get()).isEqualTo("result")
  }

  @Test
  fun `SafeFuture toVertxFuture propagates a failure`(vertx: Vertx) {
    val error = RuntimeException("boom")
    val safeFuture = SafeFuture.failedFuture<String>(error)

    val vertxFuture = safeFuture.toVertxFuture()

    assertThatThrownBy { vertxFuture.get() }
      .isInstanceOf(ExecutionException::class.java)
      .hasCause(error)
  }

  @Test
  fun `SafeFuture toVertxFuture propagates a result that completes later`(vertx: Vertx) {
    val safeFuture = SafeFuture<String>()

    val vertxFuture = safeFuture.toVertxFuture()
    safeFuture.complete("later result")

    assertThat(vertxFuture.get()).isEqualTo("later result")
  }

  @Test
  fun `SafeFuture toVertxFuture propagates a failure that completes later`(vertx: Vertx) {
    val error = IllegalStateException("later boom")
    val safeFuture = SafeFuture<String>()

    val vertxFuture = safeFuture.toVertxFuture()
    safeFuture.completeExceptionally(error)

    assertThatThrownBy { vertxFuture.get() }
      .isInstanceOf(ExecutionException::class.java)
      .hasCause(error)
  }

  @Test
  fun `CompletableFuture toVertxFuture propagates a successful result`(vertx: Vertx) {
    val completableFuture = java.util.concurrent.CompletableFuture.completedFuture("result")

    val vertxFuture = completableFuture.toVertxFuture()

    assertThat(vertxFuture.get()).isEqualTo("result")
  }

  @Test
  fun `CompletableFuture toVertxFuture propagates a failure`(vertx: Vertx) {
    val error = RuntimeException("boom")
    val completableFuture = java.util.concurrent.CompletableFuture<String>()
    completableFuture.completeExceptionally(error)

    val vertxFuture = completableFuture.toVertxFuture()

    assertThatThrownBy { vertxFuture.get() }
      .isInstanceOf(ExecutionException::class.java)
      .hasCause(error)
  }

  @Test
  fun `Future toSafeFuture and back to Vertx Future round-trips a successful result`(vertx: Vertx) {
    val originalFuture: Future<String> = Future.succeededFuture("round-trip")

    val result = originalFuture.toSafeFuture().toVertxFuture()

    assertThat(result.get()).isEqualTo("round-trip")
  }

  @Test
  fun `Future toSafeFuture and back to Vertx Future round-trips a failure`(vertx: Vertx) {
    val error = RuntimeException("round-trip failure")
    val originalFuture: Future<String> = Future.failedFuture(error)

    val result = originalFuture.toSafeFuture().toVertxFuture()

    assertThatThrownBy { result.get() }
      .isInstanceOf(ExecutionException::class.java)
      .hasCause(error)
  }
}
