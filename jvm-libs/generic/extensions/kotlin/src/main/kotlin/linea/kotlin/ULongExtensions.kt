package linea.kotlin

fun List<ULong>.hasSequentialElements(): Boolean {
  if (this.size < 2) return true // A list with less than 2 elements is trivially continuous

  for (i in 1 until this.size) {
    if (this[i] != this[i - 1] + 1UL) {
      return false
    }
  }
  return true
}
