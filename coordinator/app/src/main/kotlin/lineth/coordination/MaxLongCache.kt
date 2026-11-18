package lineth.coordination

import java.util.concurrent.atomic.AtomicLong
import java.util.function.Consumer
import kotlin.math.max

class MaxLongCache(initialValue: Long) : Consumer<Long> {
  private val cachedMax: AtomicLong = AtomicLong(initialValue)

  override fun accept(newValue: Long) {
    cachedMax.accumulateAndGet(newValue) { current, new -> max(current, new) }
  }

  fun get(): Long {
    return cachedMax.get()
  }
}
