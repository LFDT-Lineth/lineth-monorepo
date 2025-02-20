package linea.kotlin

inline fun <T, R : Comparable<R>> Iterable<T>.isSortedBy(crossinline selector: (T) -> R?): Boolean {
  if (this is Collection) {
    if (size <= 1) return true
  }
  var previousValue: R? = this.firstOrNull()
    ?.let { selector(it) }
    ?: return true
  for (element in this) {
    val currentValue = selector(element)
    if (previousValue != null && currentValue != null) {
      if (previousValue > currentValue) return false
    }
    previousValue = currentValue
  }

  return true
}
