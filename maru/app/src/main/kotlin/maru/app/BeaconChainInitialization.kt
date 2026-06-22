/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.core.BeaconBlock
import maru.core.BeaconBlockBody
import maru.core.BeaconBlockHeader
import maru.core.BeaconState
import maru.core.EMPTY_HASH
import maru.core.GENESIS_EXECUTION_PAYLOAD
import maru.core.HashUtil
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.database.BeaconChain
import maru.serialization.rlp.RLPSerializers
import maru.serialization.rlp.stateRoot

class BeaconChainInitialization(
  private val beaconChain: BeaconChain,
  private val genesisTimestamp: ULong = 0UL,
) {
  private fun initializeDb(validatorSet: Set<Validator>) {
    val genesisExecutionPayload = GENESIS_EXECUTION_PAYLOAD
    val beaconBlockBody = BeaconBlockBody(prevCommitSeals = emptySet(), executionPayload = genesisExecutionPayload)

    val beaconBlockHeader =
      BeaconBlockHeader(
        number = 0u,
        round = 0u,
        timestamp = genesisTimestamp,
        proposer = Validator(genesisExecutionPayload.feeRecipient),
        parentRoot = EMPTY_HASH,
        stateRoot = EMPTY_HASH,
        bodyRoot = EMPTY_HASH,
        headerHashFunction = RLPSerializers.DefaultHeaderHashFunction,
      )

    val sortedValidators = validatorSet.toSortedSet()
    val tmpGenesisStateRoot =
      BeaconState(
        beaconBlockHeader = beaconBlockHeader,
        validators = sortedValidators,
      )
    val stateRootHash = HashUtil.stateRoot(tmpGenesisStateRoot)

    val genesisBlockHeader = beaconBlockHeader.copy(stateRoot = stateRootHash)
    val genesisBlock = BeaconBlock(genesisBlockHeader, beaconBlockBody)
    val genesisState = BeaconState(genesisBlockHeader, sortedValidators)
    beaconChain.newBeaconChainUpdater().run {
      putBeaconState(genesisState)
      putSealedBeaconBlock(SealedBeaconBlock(genesisBlock, emptySet()))
      commit()
    }
  }

  fun ensureDbIsInitialized(validatorSet: Set<Validator>) {
    if (!beaconChain.isInitialized()) {
      initializeDb(validatorSet)
    }
  }
}
