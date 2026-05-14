/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.core.BeaconBlockHeader
import maru.database.BeaconChain
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockchain

class QbftBlockchainAdapter(
  private val beaconChain: BeaconChain,
) : QbftBlockchain {
  override fun getChainHeadHeader(): QbftBlockHeader = QbftBlockHeaderAdapter(getLatestHeader())

  override fun getChainHeadBlockNumber(): Long = getLatestHeader().number.toLong()

  private fun getLatestHeader(): BeaconBlockHeader = beaconChain.getLatestBeaconState().beaconBlockHeader
}
