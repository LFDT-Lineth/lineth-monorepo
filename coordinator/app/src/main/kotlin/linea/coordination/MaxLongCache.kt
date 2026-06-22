package linea.coordination

import java.util.concurrent.atomic.AtomicLong
import java.util.function.Consumer

class MaxLongCache(initialValue: Long) : Consumer<Long> {
  private val cachedMax: AtomicLong = AtomicLong(initialValue)

  @Synchronized
  override fun accept(newValue: Long) {
    if (newValue > cachedMax.get()) {
      cachedMax.set(newValue)
    }
  }

  fun get(): Long {
    return cachedMax.get()
  }
}
