/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.PrevRandaoProvider
import maru.consensus.ValidatorProvider
import maru.consensus.state.FinalizationProvider
import maru.core.ext.DataGenerators
import maru.database.BeaconChain
import maru.executionlayer.manager.ExecutionLayerManager
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.consensus.common.bft.blockcreation.ProposerSelector
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.whenever
import kotlin.time.Duration.Companion.milliseconds

class QbftBlockCreatorFactoryTest {
  private val executionLayerManager = Mockito.mock(ExecutionLayerManager::class.java)
  private val proposerSelector = Mockito.mock(ProposerSelector::class.java)
  private val validatorProvider = Mockito.mock(ValidatorProvider::class.java)
  private val beaconChain = Mockito.mock(BeaconChain::class.java)
  private val finalizationStateProvider = Mockito.mock(FinalizationProvider::class.java)

  @Suppress("UNCHECKED_CAST")
  private val prevRandaoProvider = Mockito.mock(PrevRandaoProvider::class.java) as PrevRandaoProvider<ULong>

  private fun createFactory(): QbftBlockCreatorFactory {
    whenever(beaconChain.getLatestBeaconState()).thenReturn(
      DataGenerators.randomBeaconState(0u),
    )
    return QbftBlockCreatorFactory(
      manager = executionLayerManager,
      proposerSelector = proposerSelector,
      validatorProvider = validatorProvider,
      beaconChain = beaconChain,
      finalizationStateProvider = finalizationStateProvider,
      prevRandaoProvider = prevRandaoProvider,
      feeRecipient = ByteArray(20),
      eagerQbftBlockCreatorConfig = EagerQbftBlockCreator.Config(100.milliseconds),
    )
  }

  @Test
  fun `uses eager block creator for first block at round 0`() {
    val blockCreator = createFactory().create(0)
    assertThat(blockCreator).isInstanceOf(EagerQbftBlockCreator::class.java)
  }

  @Test
  fun `uses delayed block creator for round 0 after first block`() {
    val factory = createFactory()
    factory.create(0) // first call
    val blockCreator = factory.create(0) // second call
    assertThat(blockCreator).isInstanceOf(DelayedQbftBlockCreator::class.java)
  }

  @Test
  fun `uses eager block creator for round greater than zero`() {
    val factory = createFactory()
    factory.create(0) // first call
    val blockCreator = factory.create(1) // round > 0
    assertThat(blockCreator).isInstanceOf(EagerQbftBlockCreator::class.java)
  }
}
