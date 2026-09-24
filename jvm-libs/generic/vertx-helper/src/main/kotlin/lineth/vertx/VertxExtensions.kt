package lineth.vertx

import io.vertx.core.Vertx
import java.util.concurrent.CompletableFuture

fun <T> Vertx.runOnContextAsync(asyncAction: () -> CompletableFuture<T>): CompletableFuture<T> {
  val resultFuture = CompletableFuture<T>()
  this.runOnContext {
    asyncAction().whenComplete { result: T, throwable: Throwable? ->
      if (throwable != null) {
        resultFuture.completeExceptionally(throwable)
      } else {
        resultFuture.complete(result)
      }
    }
  }
  return resultFuture
}
