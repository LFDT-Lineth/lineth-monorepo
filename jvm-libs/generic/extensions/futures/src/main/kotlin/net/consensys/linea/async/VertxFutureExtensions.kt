package net.consensys.linea.async

import io.vertx.core.Future
import io.vertx.core.Promise
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture

fun <T> Future<T>.get(): T = this.toCompletionStage().toCompletableFuture().get()

fun <T> Future<T>.toSafeFuture(): SafeFuture<T> = SafeFuture.of(this.toCompletionStage())

// Kotlin 2.4+ strictly propagates Vert.x's @Nullable annotation on executeBlocking's return type,
// yielding Future<T?> even when the Callable never returns null. This overload re-asserts non-null.
@Suppress("UNCHECKED_CAST")
fun <T : Any> Future<T?>.toSafeFutureNonNull(): SafeFuture<T> =
  SafeFuture.of(this.toCompletionStage()) as SafeFuture<T>

fun <T> SafeFuture<T>.toVertxFuture(): Future<T> {
  val result = Promise.promise<T>()
  this.thenAccept(result::complete)
  this.handleException(result::fail)
  return result.future()
}

fun <T> Future<T>.toCompletableFuture(): CompletableFuture<T> = this.toSafeFuture().toCompletableFuture()

fun <T> CompletableFuture<T>.toVertxFuture(): Future<T> = this.toSafeFuture().toVertxFuture()
