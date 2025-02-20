package linea.kotlin

import java.math.BigInteger
import java.util.HexFormat

fun ByteArray.assertSize(expectedSize: UInt, fieldName: String = ""): ByteArray = apply {
  require(size == expectedSize.toInt()) { "$fieldName expected to have $expectedSize bytes, but got $size" }
}

fun ByteArray.assertIs32Bytes(fieldName: String = ""): ByteArray = assertSize(32u, fieldName)

fun ByteArray.assertIs20Bytes(fieldName: String = ""): ByteArray = assertSize(20u, fieldName)

fun ByteArray.setFirstByteToZero(): ByteArray {
  this[0] = 0
  return this
}

/**
 * Slices the ByteArray into sliceSize bytes chunks and returns the sliceNumber-th chunk.
 */
fun ByteArray.sliceOf(
  sliceSize: Int,
  sliceNumber: Int,
  allowIncompleteLastSlice: Boolean = false
): ByteArray {
  assert(sliceSize > 0) {
    "sliceSize=$sliceSize should be greater than 0"
  }

  val startIndex = sliceNumber * sliceSize
  val endIndex = (sliceNumber * sliceSize + sliceSize - 1)
    .let {
      if (it >= this.size && allowIncompleteLastSlice) {
        this.size - 1
      } else {
        it
      }
    }

  assert(startIndex <= this.size && endIndex < this.size) {
    "slice $startIndex..$endIndex is out of array size=${this.size}"
  }

  return this.sliceArray(startIndex..endIndex)
}

/**
 * Slices the ByteArray into 32 bytes chunks and returns the sliceNumber-th chunk.
 */
fun ByteArray.sliceOf32(sliceNumber: Int): ByteArray {
  return this.sliceOf(sliceSize = 32, sliceNumber)
}

fun ByteArray.encodeHex(prefix: Boolean = true): String {
  val hexStr = HexFormat.of().formatHex(this)
  if (prefix) {
    return "0x$hexStr"
  } else {
    return hexStr
  }
}

fun ByteArray.toULongFromLast8Bytes(lenient: Boolean = false): ULong {
  if (!lenient && size < 8) {
    throw IllegalArgumentException("ByteArray size should be >= 8 to convert to ULong")
  }
  val significantBytes = this.sliceArray((this.size - 8).coerceAtLeast(0) until this.size)
  return BigInteger(1, significantBytes).toULong()
}

/**
 * This a temporary extension to ByteArray.
 * We expect Kotlin to add Companion to ByteArray in the future, like it did for Int and Byte.
 * This extension object ByteArrayE will be removed once that happens
 * and it's function's migrated to ByteArray.Companion.
 */
object ByteArrayExt {
  fun random(size: Int): ByteArray {
    return kotlin.random.Random.nextBytes(size)
  }

  fun random32(): ByteArray = random(32)
}
