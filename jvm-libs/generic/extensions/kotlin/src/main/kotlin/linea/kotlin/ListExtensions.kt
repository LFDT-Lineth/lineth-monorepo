package linea.kotlin

fun List<ByteArray>.byteArrayListEquals(other: List<ByteArray>): Boolean {
  if (size != other.size) return false
  return zip(other).all { (a, b) -> a.contentEquals(b) }
}

fun List<ByteArray>.byteArrayListHashCode(): Int {
  return fold(1) { acc, ba -> 31 * acc + ba.contentHashCode() }
}
