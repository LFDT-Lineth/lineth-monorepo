/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.extensions
import java.util.HexFormat
import kotlin.experimental.xor

fun ByteArray.encodeHex(prefix: Boolean = true): String {
  val hexStr = HexFormat.of().formatHex(this)
  return if (prefix) {
    "0x$hexStr"
  } else {
    hexStr
  }
}

fun String.fromHexToByteArray(): ByteArray = HexFormat.of().parseHex(removePrefix("0x"))

fun ByteArray.xor(other: ByteArray): ByteArray {
  require(this.size == other.size) { "ByteArrays must have the same length" }
  return ByteArray(this.size) { i -> this[i].xor(other[i]) }
}
