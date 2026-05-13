/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.qbft.toAddress
import maru.core.BeaconBlockHeader
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash

/**
 * Adapter class to convert a BeaconBlockHeader to a QBFT block header
 */
class QbftBlockHeaderAdapter(
  val beaconBlockHeader: BeaconBlockHeader,
) : QbftBlockHeader {
  override fun getNumber(): Long = beaconBlockHeader.number.toLong()

  override fun getTimestamp(): Long = beaconBlockHeader.timestamp.toLong()

  override fun getCoinbase(): Address = beaconBlockHeader.proposer.toAddress()

  override fun getHash(): Hash = Hash.wrap(Bytes32.wrap(beaconBlockHeader.hash()))

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (other !is QbftBlockHeaderAdapter) return false

    if (beaconBlockHeader != other.beaconBlockHeader) return false

    return true
  }

  override fun hashCode(): Int = beaconBlockHeader.hashCode()

  override fun toString(): String = beaconBlockHeader.toString()
}
