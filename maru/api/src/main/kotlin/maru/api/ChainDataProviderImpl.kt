/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api

import maru.core.BeaconState
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.extensions.fromHexToByteArray

class ChainDataProviderImpl(
  val beaconChain: BeaconChain,
) : ChainDataProvider {
  override fun getLatestBeaconState(): BeaconState = beaconChain.getLatestBeaconState()

  override fun getBeaconStateByStateRoot(stateRoot: ByteArray): BeaconState =
    beaconChain.getBeaconState(stateRoot) ?: throw BeaconStateNotFoundException()

  override fun getBeaconBlockByNumber(blockNumber: ULong): SealedBeaconBlock =
    beaconChain.getSealedBeaconBlock(blockNumber) ?: throw BlockNotFoundException()

  override fun getLatestBeaconBlock(): SealedBeaconBlock {
    val latestBeaconBlockHeader = beaconChain.getLatestBeaconState().beaconBlockHeader
    return beaconChain.getSealedBeaconBlock(latestBeaconBlockHeader.number)
      ?: throw BlockNotFoundException()
  }

  override fun getBeaconBlockByBlockRoot(blockRoot: String): SealedBeaconBlock {
    val blockRootBytes = blockRoot.fromHexToByteArray()
    return beaconChain.getSealedBeaconBlock(blockRootBytes) ?: throw BlockNotFoundException()
  }
}
