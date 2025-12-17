package linea.kotlin

import linea.OneEth
import linea.OneGWei
import java.math.BigInteger

fun ULong.toKWeiUInt(): UInt = this.toDouble().tokWeiUInt()
inline val ULong.gwei: ULong get() = this.multiplyExact(OneGWei.toULong())
inline val ULong.eth: ULong get() = this.multiplyExact(OneEth.toULong())
fun ULong.toGWei(): Double = this.toDouble().toGWei()

/**
 * Parses an hexadecimal string as [ULong] number and returns the result.
 * @throws NumberFormatException if the string is not a valid hexadecimal representation of a number.
 */
fun ULong.Companion.fromHexString(value: String): ULong = value.removePrefix("0x").toULong(16)
fun ULong.toBigInteger(): BigInteger = BigInteger(this.toString())
fun ULong.toHexString(hexPrefix: Boolean = true): String = this.toString(16).let {
  if (hexPrefix) {
    "0x$it"
  } else {
    "$it"
  }
}

fun ULong.toHexStringPaddedToBitSize(targetBitSize: Int, hexPrefix: Boolean = true): String {
  require(targetBitSize % 4 == 0) { "targetBitSize=$targetBitSize should be a multiple of 4" }
  val targetNumberOfHexDigits = targetBitSize / 4
  val rawHex = this.toString(16)
  require(rawHex.length <= targetNumberOfHexDigits) {
    val requiredBits = rawHex.length * 4
    "Number $this needs ${rawHex.length} hex digits ($requiredBits bits), targetBitSize=$targetBitSize"
  }
  val hex = rawHex.padStart(targetNumberOfHexDigits, '0')

  return if (hexPrefix) {
    "0x$hex"
  } else {
    hex
  }
}

fun ULong.toHexStringPaddedToByteSize(targetByteSize: Int, hexPrefix: Boolean = true): String =
  this.toHexStringPaddedToBitSize(targetByteSize * 8, hexPrefix)

fun ULong.toHexStringUInt256(hexPrefix: Boolean = true): String = this.toHexStringPaddedToBitSize(256, hexPrefix)

fun List<ULong>.hasSequentialElements(): Boolean {
  if (this.size < 2) return true // A list with less than 2 elements is trivially continuous

  for (i in 1 until this.size) {
    if (this[i] != this[i - 1] + 1UL) {
      return false
    }
  }
  return true
}

fun ULong.minusCoercingUnderflow(other: ULong): ULong {
  return if (this > other) {
    this - other
  } else {
    0UL
  }
}

fun ULongRange.intersection(other: ULongRange): ULongRange {
  val start = maxOf(this.first, other.first)
  val end = minOf(this.last, other.last)
  return if (start <= end) {
    start..end
  } else {
    ULongRange.EMPTY
  }
}
