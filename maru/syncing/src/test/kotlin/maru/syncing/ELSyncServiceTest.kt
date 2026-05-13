/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

import maru.consensus.NewBlockHandler
import maru.core.ext.DataGenerators
import maru.database.InMemoryBeaconChain
import maru.executionlayer.manager.ExecutionPayloadStatus
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import testutils.maru.TestablePeriodicTimerFactory
import kotlin.concurrent.atomics.AtomicInt
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.concurrent.atomics.incrementAndFetch
import kotlin.time.Duration.Companion.seconds

@OptIn(ExperimentalAtomicApi::class)
class ELSyncServiceTest {
  private val switchTimestamp = 3UL

  @Test
  fun `should set sync status to Synced for genesis block`() {
    var elSyncStatus: ELSyncStatus? = null
    val onStatusChangeCount = AtomicInt(0)
    val onStatusChange: (ELSyncStatus) -> Unit = {
      elSyncStatus = it
      onStatusChangeCount.incrementAndFetch()
    }
    val config = ELSyncService.Config(pollingInterval = 1.seconds)
    val beaconChain =
      DataGenerators.genesisState(0uL, emptySet()).let {
        InMemoryBeaconChain(initialBeaconState = it.first, initialBeaconBlock = it.second)
      }
    val blockImportHandler =
      NewBlockHandler<Unit> { SafeFuture.completedFuture(Unit) }
    val blockValidatorHandler =
      NewBlockHandler<ForkChoiceUpdatedResult> {
        SafeFuture.completedFuture(DataGenerators.randomValidForkChoiceUpdatedResult())
      }

    val timerFactory = TestablePeriodicTimerFactory()
    val elSyncService =
      ELSyncService(
        beaconChain = beaconChain,
        eLValidatorBlockImportHandler = blockValidatorHandler,
        followerELBLockImportHandler = blockImportHandler,
        onStatusChange = onStatusChange,
        config = config,
        timerFactory = timerFactory,
      )

    elSyncService.start()
    val timer = timerFactory.getTimer("ELSyncPoller")!!

    assertThat(elSyncStatus).isNull()
    timer.runNextTask()
    assertThat(elSyncStatus).isEqualTo(ELSyncStatus.SYNCED)

    timer.runNextTask()
    timer.runNextTask()
    assertThat(elSyncStatus).isEqualTo(ELSyncStatus.SYNCED)
    elSyncService.stop()
  }

  @Test
  fun `should change el sync status when el is syncing and synced`() {
    var elSyncStatus: ELSyncStatus? = null
    val onStatusChangeCount = AtomicInt(0)
    val onStatusChange: (ELSyncStatus) -> Unit = {
      elSyncStatus = it
      onStatusChangeCount.incrementAndFetch()
    }
    val config = ELSyncService.Config(pollingInterval = 1.seconds)
    val beaconChain =
      DataGenerators.genesisState(0uL, emptySet()).let {
        InMemoryBeaconChain(initialBeaconState = it.first, initialBeaconBlock = it.second)
      }
    var forkChoiceResultToReturn =
      DataGenerators.randomValidForkChoiceUpdatedResult().copy(
        payloadStatus = DataGenerators.randomValidPayloadStatus().copy(status = ExecutionPayloadStatus.SYNCING),
      )
    val blockImportHandler =
      NewBlockHandler<Unit> { SafeFuture.completedFuture(Unit) }
    val blockValidatorHandler =
      NewBlockHandler<ForkChoiceUpdatedResult> {
        SafeFuture.completedFuture(forkChoiceResultToReturn)
      }
    val timerFactory = TestablePeriodicTimerFactory()
    val elSyncService =
      ELSyncService(
        beaconChain = beaconChain,
        eLValidatorBlockImportHandler = blockValidatorHandler,
        followerELBLockImportHandler = blockImportHandler,
        onStatusChange = onStatusChange,
        config = config,
        timerFactory = timerFactory,
      )
    elSyncService.start()
    val timer = timerFactory.getTimer("ELSyncPoller")!!

    assertThat(elSyncStatus).isNull()
    timer.runNextTask()
    assertThat(elSyncStatus).isEqualTo(ELSyncStatus.SYNCED)

    beaconChain
      .newBeaconChainUpdater()
      .putBeaconState(DataGenerators.randomBeaconState(number = 3uL, timestamp = switchTimestamp))
      .putSealedBeaconBlock(DataGenerators.randomSealedBeaconBlock(3UL))
      .commit()

    timer.runNextTask()
    assertThat(elSyncStatus).isEqualTo(ELSyncStatus.SYNCING)

    forkChoiceResultToReturn = DataGenerators.randomValidForkChoiceUpdatedResult()
    timer.runNextTask()
    assertThat(elSyncStatus).isEqualTo(ELSyncStatus.SYNCED)
    assertThat(onStatusChangeCount.load()).isEqualTo(3)

    timer.runNextTask()
    timer.runNextTask()
    assertThat(elSyncStatus).isEqualTo(ELSyncStatus.SYNCED)
    elSyncService.stop()
  }
}
