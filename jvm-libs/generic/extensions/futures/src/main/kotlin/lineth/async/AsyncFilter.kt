package lineth.async

import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface AsyncFilter<T> {
  fun invoke(items: List<T>): SafeFuture<List<T>>

  fun then(next: AsyncFilter<T>): AsyncFilter<T> = AsyncFilter { items ->
    this.invoke(items).thenCompose(next::invoke)
  }

  companion object {
    fun <T> NoOp(): AsyncFilter<T> = AsyncFilter { SafeFuture.completedFuture(it) }
  }
}
