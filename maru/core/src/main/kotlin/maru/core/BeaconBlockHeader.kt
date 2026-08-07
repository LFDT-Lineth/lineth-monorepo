/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

import lineth.kotlin.encodeHex

data class BeaconBlockHeader(
  val number: ULong,
  val round: UInt,
  val timestamp: ULong,
  val proposer: Validator,
  val parentRoot: ByteArray,
  val stateRoot: ByteArray,
  val bodyRoot: ByteArray,
  private val beaconBlockIdHashFunction: BeaconBlockIdHashFunction,
) {
  val beaconBlockIdHash by lazy { beaconBlockIdHashFunction(this) }

  // Object equality is field-based (round-sensitive), deliberately EXCLUDING beaconBlockIdHashFunction: two
  // headers with the same fields are equal regardless of which hash function was injected. This is distinct
  // from the chain-identity `beaconBlockIdHash` above, which is fork-aware and round-independent from
  // QBFT_PHASE1 onward. Making equals/hashCode hash-based would collapse headers differing only in
  // round/proposer into a single object under PHASE1, breaking the Besu QBFT engine's round handling.
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BeaconBlockHeader

    if (number != other.number) return false
    if (round != other.round) return false
    if (timestamp != other.timestamp) return false
    if (proposer != other.proposer) return false
    if (!parentRoot.contentEquals(other.parentRoot)) return false
    if (!stateRoot.contentEquals(other.stateRoot)) return false
    if (!bodyRoot.contentEquals(other.bodyRoot)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = number.hashCode()
    result = 31 * result + round.hashCode()
    result = 31 * result + timestamp.hashCode()
    result = 31 * result + proposer.hashCode()
    result = 31 * result + parentRoot.contentHashCode()
    result = 31 * result + stateRoot.contentHashCode()
    result = 31 * result + bodyRoot.contentHashCode()
    return result
  }

  fun beaconBlockIdHash(): ByteArray = beaconBlockIdHash

  override fun toString(): String =
    "BeaconBlockHeader(number=$number, round=$round, timestamp=$timestamp, proposer=$proposer, " +
      "parentRoot=${parentRoot.encodeHex()}, stateRoot=${stateRoot.encodeHex()}, bodyRoot=${bodyRoot.encodeHex()})"
}
