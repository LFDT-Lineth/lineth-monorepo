/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.core

data class SealedBeaconBlock(
  val beaconBlock: BeaconBlock,
  val commitSeals: Set<Seal>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (other !is SealedBeaconBlock) return false

    if (commitSeals != other.commitSeals) return false
    if (beaconBlock != other.beaconBlock) return false

    return true
  }

  override fun hashCode(): Int {
    var result = commitSeals.hashCode()
    result = 31 * result + beaconBlock.hashCode()
    return result
  }
}
