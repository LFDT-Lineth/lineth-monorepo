package linea.domain

import linea.kotlin.encodeHex

data class BlockNumberAndHash(
  val number: ULong,
  val hash: ByteArray
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlockNumberAndHash

    if (number != other.number) return false
    if (!hash.contentEquals(other.hash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = number.hashCode()
    result = 31 * result + hash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "BlockNumberAndHash(number=$number, hash=${hash.encodeHex()})"
  }
}
