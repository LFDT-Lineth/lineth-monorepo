/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

import maru.extensions.encodeHex
import org.apache.tuweni.bytes.Bytes

data class Validator(
  val address: ByteArray,
) : Comparable<Validator> {
  init {
    require(address.size == 20) {
      "Addresses should be 20 bytes long"
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as Validator

    return address.contentEquals(other.address)
  }

  override fun hashCode(): Int = address.contentHashCode()

  override fun toString(): String = "Validator(address=${address.encodeHex()})"

  override fun compareTo(other: Validator): Int = Bytes.wrap(address).compareTo(Bytes.wrap(other.address))
}
