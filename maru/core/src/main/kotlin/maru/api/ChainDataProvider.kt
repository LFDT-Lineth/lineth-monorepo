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

interface ChainDataProvider {
  fun getGenesisBeaconState(): BeaconState =
    getBeaconStateByStateRoot(getGenesisBeaconBlock().beaconBlock.beaconBlockHeader.stateRoot)

  fun getLatestBeaconState(): BeaconState

  fun getBeaconStateByStateRoot(stateRoot: ByteArray): BeaconState

  fun getGenesisBeaconBlock(): SealedBeaconBlock = getBeaconBlockByNumber(0u)

  fun getBeaconBlockByNumber(blockNumber: ULong): SealedBeaconBlock

  fun getLatestBeaconBlock(): SealedBeaconBlock

  fun getBeaconBlockByBlockRoot(blockRoot: String): SealedBeaconBlock
}

class BlockNotFoundException : Exception()

class BeaconStateNotFoundException : Exception()
