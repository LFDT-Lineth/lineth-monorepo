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

data class BeaconBlockHeader(
  val number: ULong,
  val round: UInt,
  val timestamp: ULong,
  val proposer: Validator,
  val parentRoot: ByteArray,
  val stateRoot: ByteArray,
  val bodyRoot: ByteArray,
  private val headerHashFunction: HeaderHashFunction,
) {
  val hash by lazy { headerHashFunction(this) }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BeaconBlockHeader

    return hash.contentEquals(other.hash)
  }

  override fun hashCode(): Int = hash.contentHashCode()

  fun hash(): ByteArray = hash

  override fun toString(): String =
    "BeaconBlockHeader(number=$number, round=$round, timestamp=$timestamp, proposer=$proposer, " +
      "parentRoot=${parentRoot.encodeHex()}, stateRoot=${stateRoot.encodeHex()}, bodyRoot=${bodyRoot.encodeHex()})"
}
