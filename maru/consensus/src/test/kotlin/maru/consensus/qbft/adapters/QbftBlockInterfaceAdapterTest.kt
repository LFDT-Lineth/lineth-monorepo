/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import com.github.michaelbull.result.Ok
import maru.consensus.qbft.toAddress
import maru.consensus.state.StateTransition
import maru.consensus.validation.StateRootValidator
import maru.core.BeaconBlock
import maru.core.BeaconState
import maru.core.Validator
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.SequencedSet

class QbftBlockInterfaceAdapterTest {
  private fun createStateTransition(
    validators: SequencedSet<Validator> = DataGenerators.randomValidators().toSortedSet(),
  ): StateTransition =
    StateTransition { block ->
      SafeFuture.completedFuture(
        BeaconState(
          beaconBlockHeader = block.beaconBlockHeader,
          validators = validators,
        ),
      )
    }

  @Test
  fun `can replace round number in header for commit block`() {
    val beaconBlock =
      BeaconBlock(
        beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(1UL).copy(round = 10u),
        beaconBlockBody = DataGenerators.randomBeaconBlockBody(),
      )
    val qbftBlock = QbftBlockAdapter(beaconBlock)
    val stateTransition = createStateTransition()
    val adapter = QbftBlockInterfaceAdapter(stateTransition)
    val updatedBlock =
      adapter.replaceRoundForCommitBlock(qbftBlock, 20)
    val updatedBeaconBlockHeader = updatedBlock.header.toBeaconBlockHeader()

    assertThat(updatedBeaconBlockHeader.round).isEqualTo(20u)
    assertThat(updatedBeaconBlockHeader.proposer).isEqualTo(beaconBlock.beaconBlockHeader.proposer)
  }

  @Test
  fun `can replace round and proposer in header for proposal block`() {
    val originalProposer = DataGenerators.randomValidator()
    val newProposer = DataGenerators.randomValidator()
    val beaconBlock =
      BeaconBlock(
        beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(1UL).copy(
          round = 10u,
          proposer = originalProposer,
        ),
        beaconBlockBody = DataGenerators.randomBeaconBlockBody(),
      )
    val qbftBlock = QbftBlockAdapter(beaconBlock)
    val stateTransition = createStateTransition()
    val adapter = QbftBlockInterfaceAdapter(stateTransition)
    val updatedBlock = adapter.replaceRoundAndProposerForProposalBlock(qbftBlock, 25, newProposer.toAddress())
    val updatedBeaconBlockHeader = updatedBlock.header.toBeaconBlockHeader()

    assertThat(updatedBeaconBlockHeader.round).isEqualTo(25u)
    assertThat(updatedBeaconBlockHeader.proposer).isEqualTo(newProposer)
  }

  @Test
  fun `updates state root when replacing round number in commit block`() {
    val validators = DataGenerators.randomValidators().toSortedSet()
    val beaconBlock =
      BeaconBlock(
        beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(1UL).copy(round = 10u),
        beaconBlockBody = DataGenerators.randomBeaconBlockBody(),
      )
    val qbftBlock = QbftBlockAdapter(beaconBlock)
    val stateTransition = createStateTransition(validators)
    val adapter = QbftBlockInterfaceAdapter(stateTransition)
    val updatedBlock = adapter.replaceRoundForCommitBlock(qbftBlock, 20)

    val updatedBeaconBlock = updatedBlock.toBeaconBlock()
    val validationResult = StateRootValidator(stateTransition).validateBlock(updatedBeaconBlock).get()
    assertThat(validationResult).isEqualTo(Ok(Unit))
    assertThat(updatedBeaconBlock.beaconBlockHeader.round).isEqualTo(20u)
  }

  @Test
  fun `updates state root when replacing round number and proposal in proposal block`() {
    val originalProposer = DataGenerators.randomValidator()
    val newProposer = DataGenerators.randomValidator()
    val validators = DataGenerators.randomValidators().toSortedSet()
    val beaconBlock =
      BeaconBlock(
        beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(1UL).copy(
          round = 10u,
          proposer = originalProposer,
        ),
        beaconBlockBody = DataGenerators.randomBeaconBlockBody(),
      )
    val qbftBlock = QbftBlockAdapter(beaconBlock)
    val stateTransition = createStateTransition(validators)
    val adapter = QbftBlockInterfaceAdapter(stateTransition)
    val updatedBlock = adapter.replaceRoundAndProposerForProposalBlock(qbftBlock, 25, newProposer.toAddress())

    val updatedBeaconBlock = updatedBlock.toBeaconBlock()
    val validationResult = StateRootValidator(stateTransition).validateBlock(updatedBeaconBlock).get()
    assertThat(validationResult).isEqualTo(Ok(Unit))
    assertThat(updatedBeaconBlock.beaconBlockHeader.round).isEqualTo(25u)
    assertThat(updatedBeaconBlock.beaconBlockHeader.proposer).isEqualTo(newProposer)
  }
}
