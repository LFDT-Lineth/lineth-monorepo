package linea.kotlin

internal fun <T : Comparable<T>> isRangeWithin(outer: ClosedRange<T>, inner: ClosedRange<T>): Boolean {
  return inner.start >= outer.start && inner.endInclusive <= outer.endInclusive
}

fun <T : Comparable<T>> ClosedRange<T>.contains(inner: ClosedRange<T>): Boolean = isRangeWithin(
  outer = this,
  inner = inner
)
fun <T : Comparable<T>> ClosedRange<T>.isWithin(outer: ClosedRange<T>): Boolean = isRangeWithin(
  outer = outer,
  inner = this
)
