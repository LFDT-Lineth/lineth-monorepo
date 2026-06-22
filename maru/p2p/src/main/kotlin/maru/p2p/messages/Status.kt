/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import linea.kotlin.encodeHex

data class Status(
  val forkIdHash: ByteArray,
  val latestStateRoot: ByteArray,
  val latestBlockNumber: ULong,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (other !is Status) return false

    if (!forkIdHash.contentEquals(other.forkIdHash)) return false
    if (!latestStateRoot.contentEquals(other.latestStateRoot)) return false
    if (latestBlockNumber != other.latestBlockNumber) return false

    return true
  }

  override fun hashCode(): Int {
    var result = forkIdHash.contentHashCode()
    result = 31 * result + latestStateRoot.contentHashCode()
    result = 31 * result + latestBlockNumber.hashCode()
    return result
  }

  override fun toString(): String =
    "Status(" +
      "latestBlockNumber=$latestBlockNumber, " +
      "forkIdHash=${forkIdHash.encodeHex()}, " +
      "latestStateRoot=${latestStateRoot.encodeHex()})"
}
