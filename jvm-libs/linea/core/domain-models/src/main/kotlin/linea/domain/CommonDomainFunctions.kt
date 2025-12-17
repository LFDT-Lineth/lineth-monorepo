package linea.domain

import linea.kotlin.assertIs20Bytes
import linea.kotlin.decodeHex
import java.math.BigInteger

object CommonDomainFunctions {
  fun blockIntervalString(startBlockNumber: ULong, endBlockNumber: ULong): String {
    return "[$startBlockNumber..$endBlockNumber]${endBlockNumber - startBlockNumber + 1uL}"
  }
}

fun String.assertIsValidAddress(fieldName: String = ""): String = apply {
  this.decodeHex().assertIs20Bytes(fieldName)
}

fun String.bigIntFromPrefixedHex(): BigInteger {
  return BigInteger(this.removePrefix("0x"), 16)
}

fun String.uLongFromPrefixedHex(): ULong {
  return this.removePrefix("0x").toULong(16)
}
