/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.qbft.ProposerSelectorImpl
import maru.consensus.qbft.toAddress
import maru.core.ext.DataGenerators
import maru.database.BeaconChain
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.junit.jupiter.api.assertThrows
import org.mockito.Mockito.mock
import org.mockito.kotlin.whenever
import kotlin.test.Test

class ProposerSelectorAdapterTest {
  @Test
  fun `test previous block not found`() {
    val beaconChain = mock<BeaconChain>()
    val missingBlockNumber = 3

    val proposerSelectorAdapter = ProposerSelectorAdapter(beaconChain, ProposerSelectorImpl)
    val consensusRoundIdentifier = ConsensusRoundIdentifier(missingBlockNumber.toLong(), 0)

    val exception =
      assertThrows<Exception> {
        proposerSelectorAdapter.selectProposerForRound(consensusRoundIdentifier)
      }
    assertThat(exception).isInstanceOf(IllegalStateException::class.java)
    assertThat(exception.message).isEqualTo("Parent block not found. parentBlockNumber=${missingBlockNumber - 1}")
  }

  @Test
  fun `passes the state for a given block number to proposer selector`() {
    val beaconChain = mock<BeaconChain>()
    val currentBlockNumber = 3uL
    val previousBlockNumber = currentBlockNumber.dec()
    val expectedState = DataGenerators.randomBeaconState(
      previousBlockNumber,
    )
    whenever(beaconChain.getBeaconState(previousBlockNumber)).thenReturn(expectedState)
    val proposerSelector = ProposerSelectorAdapter(beaconChain, ProposerSelectorImpl)
    val consensusRoundIdentifier = ConsensusRoundIdentifier(currentBlockNumber.toLong(), 0)
    val result = proposerSelector.selectProposerForRound(consensusRoundIdentifier)

    assertThat(result).isIn(expectedState.validators.map { it.toAddress() })
  }
}
