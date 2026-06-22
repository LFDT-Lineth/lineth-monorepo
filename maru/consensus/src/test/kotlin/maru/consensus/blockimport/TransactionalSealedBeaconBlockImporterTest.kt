/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.blockimport

import maru.consensus.state.StateTransition
import maru.core.BeaconBlock
import maru.core.BeaconState
import maru.core.ext.DataGenerators
import maru.database.BeaconChain
import maru.database.InMemoryBeaconChain
import maru.executionlayer.manager.ExecutionLayerManager
import maru.p2p.ValidationResult
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito.mock
import org.mockito.Mockito.reset
import org.mockito.kotlin.any
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import maru.executionlayer.manager.ext.DataGenerators as ExecutionLayerDataGenerators

class TransactionalSealedBeaconBlockImporterTest {
  private var executionLayerManager: ExecutionLayerManager = mock()

  private val stateTransition: StateTransition = mock()

  private lateinit var beaconChain: BeaconChain
  private var beaconBlockImporterResponse =
    SafeFuture.completedFuture(ExecutionLayerDataGenerators.randomValidForkChoiceUpdatedResult())

  private lateinit var qbftBlockImporter: TransactionalSealedBeaconBlockImporter
  private lateinit var initialBeaconState: BeaconState

  @BeforeEach
  fun setUp() {
    initialBeaconState =
      DataGenerators.randomBeaconState(
        2UL,
      )
    beaconChain = InMemoryBeaconChain(initialBeaconState)
    qbftBlockImporter = TransactionalSealedBeaconBlockImporter(
      beaconChain = beaconChain,
      stateTransition = stateTransition,
      beaconBlockImporter = {
          _: BeaconState,
          _: BeaconBlock,
        ->
        beaconBlockImporterResponse
      },
    )
  }

  @AfterEach
  fun tearDown() {
    reset(executionLayerManager)
    reset(stateTransition)
  }

  @Test
  fun `importBlock returns success on successful import`() {
    val sealedBeaconBlock =
      DataGenerators.randomSealedBeaconBlock(
        2UL,
      )
    val beaconState =
      DataGenerators.randomBeaconState(
        2UL,
      )

    whenever(stateTransition.processBlock(any())).thenReturn(SafeFuture.completedFuture(beaconState))
    whenever(executionLayerManager.setHead(any(), any(), any())).thenReturn(
      SafeFuture.completedFuture(
        ExecutionLayerDataGenerators
          .randomValidForkChoiceUpdatedResult(),
      ),
    )

    val result = qbftBlockImporter.importBlock(sealedBeaconBlock).get()
    assertThat(result).isEqualTo(ValidationResult.Companion.Valid)
    assertThat(beaconChain.getLatestBeaconState()).isEqualTo(beaconState)
  }

  @Test
  fun `importBlock rolls the DB update back on state transition failure and returns failed future`() {
    val sealedBeaconBlock =
      DataGenerators.randomSealedBeaconBlock(
        2UL,
      )
    val expectedException = RuntimeException("Test exception")
    whenever(stateTransition.processBlock(any())).thenThrow(expectedException)
    val stateBeforeTransition = beaconChain.getLatestBeaconState()

    val result = qbftBlockImporter.importBlock(sealedBeaconBlock)

    assertThat(result.exceptionNow()).isEqualTo(expectedException)
    val stateAfterTransition = beaconChain.getLatestBeaconState()
    assertThat(stateBeforeTransition).isEqualTo(stateAfterTransition)
  }

  @Test
  fun `importBlock succeeds even with block import failure`() {
    val sealedBeaconBlock =
      DataGenerators.randomSealedBeaconBlock(
        2UL,
      )
    val beaconState =
      DataGenerators.randomBeaconState(
        2UL,
      )
    whenever(stateTransition.processBlock(any())).thenReturn(
      SafeFuture.completedFuture(
        beaconState,
      ),
    )
    val expectedException = RuntimeException("Test exception")
    beaconBlockImporterResponse = SafeFuture.failedFuture(expectedException)

    val result = qbftBlockImporter.importBlock(sealedBeaconBlock).get()
    assertThat(result).isEqualTo(ValidationResult.Companion.Valid)
    assertThat(beaconChain.getLatestBeaconState()).isEqualTo(beaconState)
  }
}
