/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.ValidatorProvider
import maru.consensus.qbft.adapters.QbftBlockAdapter
import maru.consensus.qbft.adapters.QbftSealedBlockAdapter
import maru.consensus.qbft.adapters.toBeaconBlock
import maru.consensus.qbft.adapters.toBeaconBlockHeader
import maru.core.BeaconBlock
import maru.core.BeaconBlockBody
import maru.core.BeaconBlockHeader
import maru.core.BeaconState
import maru.core.EMPTY_HASH
import maru.core.ExecutionPayload
import maru.core.HashUtil
import maru.core.Seal
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.database.BeaconChain
import maru.executionlayer.manager.ExecutionLayerManager
import maru.serialization.rlp.bodyRoot
import maru.serialization.rlp.headerHash
import maru.serialization.rlp.stateRoot
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.blockcreation.ProposerSelector
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCreator
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.crypto.SECPSignature
import java.util.Optional

/**
 * Responsible for QBFT block creation. As opposed to EagerBlockCreator, Delayed one relies on the fact that FCU was
 * called some time in advance. So at the time of `createBlock` it actually ends the block creation process, not
 * starts it
 */
class DelayedQbftBlockCreator(
  private val manager: ExecutionLayerManager,
  private val proposerSelector: ProposerSelector,
  private val validatorProvider: ValidatorProvider,
  private val beaconChain: BeaconChain,
  private val round: Int,
) : QbftBlockCreator {
  companion object {
    fun createSealedBlock(
      qbftBlock: QbftBlock,
      roundNumber: Int,
      commitSeals: Collection<SECPSignature>,
    ): QbftBlock {
      val seals =
        commitSeals.map { Seal(it.encodedBytes().toArrayUnsafe()) }.toSet()
      val beaconBlock = qbftBlock.toBeaconBlock()
      val beaconBlockHeader = beaconBlock.beaconBlockHeader
      val updatedBlockHeader = beaconBlockHeader.copy(round = roundNumber.toUInt())
      val sealedBlockBody =
        SealedBeaconBlock(
          BeaconBlock(updatedBlockHeader, beaconBlock.beaconBlockBody),
          seals,
        )
      return QbftSealedBlockAdapter(sealedBlockBody)
    }

    // Visible for testing
    fun createBeaconBlock(
      parentSealedBeaconBlock: SealedBeaconBlock,
      executionPayload: ExecutionPayload,
      round: Int,
      timestamp: ULong,
      proposer: ByteArray,
      validators: Set<Validator>,
    ): BeaconBlock {
      val parentBeaconBlockHeader =
        parentSealedBeaconBlock.beaconBlock.beaconBlockHeader
      val beaconBlockBody =
        BeaconBlockBody(parentSealedBeaconBlock.commitSeals, executionPayload)
      val stateRootBlockHeader =
        BeaconBlockHeader(
          number = parentBeaconBlockHeader.number + 1UL,
          round = round.toUInt(),
          timestamp = timestamp,
          proposer = Validator(proposer),
          parentRoot = parentBeaconBlockHeader.hash(),
          stateRoot = EMPTY_HASH, // temporary state root to avoid circular dependency
          bodyRoot = HashUtil.bodyRoot(beaconBlockBody),
          headerHashFunction = HashUtil::headerHash,
        )
      val stateRoot =
        HashUtil.stateRoot(
          BeaconState(stateRootBlockHeader, validators.toSortedSet()),
        )
      val finalBlockHeader = stateRootBlockHeader.copy(stateRoot = stateRoot)
      return BeaconBlock(finalBlockHeader, beaconBlockBody)
    }
  }

  override fun createBlock(
    headerTimeStampSeconds: Long,
    parentHeader: QbftBlockHeader,
  ): QbftBlockCreator.BlockCreationResult {
    val parentBeaconBlockHeader = parentHeader.toBeaconBlockHeader()
    val executionPayload =
      try {
        manager.finishBlockBuilding().get()
      } catch (e: Exception) {
        throw IllegalStateException("Execution payload unavailable, unable to create block", e)
      }
    val parentSealedBeaconBlock =
      beaconChain.getSealedBeaconBlock(parentBeaconBlockHeader.hash())
        ?: throw IllegalStateException("Parent beacon block unavailable, unable to create block")
    val proposer =
      proposerSelector.selectProposerForRound(
        ConsensusRoundIdentifier((parentBeaconBlockHeader.number + 1UL).toLong(), round),
      )
    val validators =
      validatorProvider
        .getValidatorsAfterBlock(
          parentBeaconBlockHeader.number,
        ).get()
    val beaconBlock =
      createBeaconBlock(
        parentSealedBeaconBlock = parentSealedBeaconBlock,
        executionPayload = executionPayload,
        round = round,
        timestamp = headerTimeStampSeconds.toULong(),
        proposer = proposer.bytes.toArrayUnsafe(),
        validators = validators,
      )
    return QbftBlockCreator.BlockCreationResult(QbftBlockAdapter(beaconBlock), Optional.empty())
  }

  override fun createSealedBlock(
    block: QbftBlock,
    roundNumber: Int,
    commitSeals: Collection<SECPSignature>,
  ): QbftBlock = createSealedBlock(qbftBlock = block, roundNumber = roundNumber, commitSeals = commitSeals)
}
