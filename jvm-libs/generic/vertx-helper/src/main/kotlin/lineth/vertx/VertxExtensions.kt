package lineth.vertx

import io.vertx.core.Future
import io.vertx.core.Promise
import io.vertx.core.Vertx
import java.util.concurrent.CompletableFuture

fun <T> Vertx.runOnContextAsync2(asyncAction: () -> CompletableFuture<T>): CompletableFuture<T> {
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

fun <T> Vertx.runOnContextAsync(asyncAction: () -> Future<T>): Future<T> {
  val resultFuture = Promise.promise<T>()
  this.runOnContext {
    asyncAction().onComplete { result: T, throwable: Throwable? ->
      if (throwable != null) {
        resultFuture.fail(throwable)
      } else {
        resultFuture.complete(result)
      }
    }
  }
  return resultFuture.future()
}
