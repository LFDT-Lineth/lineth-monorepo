package net.consensys

import java.math.BigInteger
import java.util.HexFormat

fun String.decodeHex(): ByteArray {
  check(length % 2 == 0) { "Must have an even length" }
  return HexFormat.of().parseHex(removePrefix("0x"))
}

fun String.containsAny(strings: List<String>, ignoreCase: Boolean): Boolean {
  return strings.any { this.contains(it, ignoreCase) }
}

fun String.toIntFromHex(): Int = removePrefix("0x").toInt(16)
fun String.toLongFromHex(): Long = removePrefix("0x").toLong(16)
fun String.toULongFromHex(): ULong = BigInteger(removePrefix("0x"), 16).toULong()
fun String.toBigIntegerFromHex(): BigInteger = BigInteger(removePrefix("0x"), 16)
