package linea.kotlin

fun List<ByteArray>.byteArrayListEquals(other: List<ByteArray>): Boolean {
  if (size != other.size) return false
  return zip(other).all { (a, b) -> a.contentEquals(b) }
}

fun List<ByteArray>.byteArrayListHashCode(): Int {
  return fold(1) { acc, ba -> 31 * acc + ba.contentHashCode() }
}

/**
 * Renders a `List<ByteArray>` deterministically (hex per element), unlike the default `List.toString()` which
 * renders each `ByteArray` by identity (`[B@6a3967ba`).
 */
fun List<ByteArray>.byteArrayListToHexString(): String =
  joinToString(prefix = "[", postfix = "]", separator = ", ") { it.encodeHex() }
