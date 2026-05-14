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

data class Seal(
  val signature: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as Seal

    return signature.contentEquals(other.signature)
  }

  override fun hashCode(): Int = signature.contentHashCode()

  override fun toString(): String = signature.encodeHex()
}
