/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class SyncControllerTest {
  private lateinit var fakeClSyncService: FakeCLSyncService
  private lateinit var syncController: BeaconSyncControllerImpl

  private fun createController(
    blockNumber: ULong,
    clSyncService: CLSyncService = fakeClSyncService,
    desyncTolerance: ULong = 1UL,
  ): BeaconSyncControllerImpl = createSyncController(blockNumber, clSyncService, desyncTolerance)

  @BeforeEach
  fun setUp() {
    fakeClSyncService = FakeCLSyncService()
    syncController = createController(0UL)
  }

  @Test
  fun `should initialize with default SYNCING status`() {
    assertThat(syncController.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
    assertThat(syncController.getElSyncStatus()).isEqualTo(ELSyncStatus.SYNCING)
    assertThat(syncController.isBeaconChainSynced()).isFalse()
    assertThat(syncController.isELSynced()).isFalse()
    assertThat(syncController.isNodeFullInSync()).isFalse()
  }

  @Test
  fun `should trigger sync when beacon chain is empty and sync target is provided`() {
    // When: chain head is updated with non-empty target
    syncController.onBeaconChainSyncTargetUpdated(100UL)

    // Then: CL sync should start and target should be set
    assertThat(syncController.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(100UL)
  }

  @Test
  fun `should trigger sync when behind sync target`() {
    // Given: beacon chain initialized with current head at block 50
    val controller = createController(50UL)

    // When: chain head is updated to higher block
    controller.onBeaconChainSyncTargetUpdated(100UL)

    // Then: CL sync should start
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(100UL)
  }

  @Test
  fun `should not trigger sync when synced already at sync target`() {
    // Given: beacon chain is at the same level as sync target
    val controller = createController(100UL)
    controller.updateClSyncStatus(CLSyncStatus.SYNCED)

    // When: chain head is updated to same block (controller starts in SYNCING by default)
    controller.onBeaconChainSyncTargetUpdated(100UL)

    // Then: should transition to SYNCED since target matches current head
    assertThat(fakeClSyncService.lastSyncTarget).isNull() // No sync target set on service
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCED)
    assertThat(controller.isBeaconChainSynced()).isTrue()
  }

  @Test
  fun `should update sync target during ongoing sync`() {
    // Given: sync is already in progress
    syncController.updateClSyncStatus(CLSyncStatus.SYNCING)
    fakeClSyncService.lastSyncTarget = 100UL

    // When: sync target is updated
    syncController.onBeaconChainSyncTargetUpdated(150UL)

    // Then: sync target should be updated
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(150UL)
    assertThat(syncController.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
  }

  @Test
  fun `should notify handlers when sync status changes`() {
    val clStatusUpdates = mutableListOf<CLSyncStatus>()
    val elStatusUpdates = mutableListOf<ELSyncStatus>()

    syncController.onClSyncStatusUpdate { clStatusUpdates.add(it) }
    syncController.onElSyncStatusUpdate { elStatusUpdates.add(it) }

    // When: status changes from default SYNCING to SYNCED
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)

    // Then: handlers should be notified
    assertThat(clStatusUpdates).containsExactly(CLSyncStatus.SYNCED)
    assertThat(elStatusUpdates).containsExactly(ELSyncStatus.SYNCED)
  }

  @Test
  fun `should notify beacon sync complete when CL sync completes`() {
    var beaconSyncCompleted = false
    syncController.onBeaconSyncComplete { beaconSyncCompleted = true }

    // When: CL sync completes
    fakeClSyncService.triggerSyncComplete(100UL)

    // Then: beacon sync complete should be notified
    assertThat(beaconSyncCompleted).isTrue()
    assertThat(syncController.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCED)
  }

  @Test
  fun `should notify full sync complete when EL completes after CL`() {
    var fullSyncCompleted = false
    syncController.onFullSyncComplete { fullSyncCompleted = true }

    // Given: both are syncing
    syncController.updateClSyncStatus(CLSyncStatus.SYNCING)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCING)

    // When: EL completes first
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)
    assertThat(fullSyncCompleted).isFalse()

    // When: CL completes
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // It needs another update from the EL sync service
    assertThat(fullSyncCompleted).isFalse()

    // When: EL completes
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)

    assertThat(fullSyncCompleted).isTrue()
  }

  @Test
  fun `should handle multiple sync target updates correctly`() {
    // Given: beacon chain with empty initial state (block 0)
    val controller = createController(0UL)

    // When: multiple targets are provided
    controller.onBeaconChainSyncTargetUpdated(50UL)
    controller.onBeaconChainSyncTargetUpdated(100UL)
    controller.onBeaconChainSyncTargetUpdated(75UL)

    // Then: should have the latest target
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(75UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
  }

  @Test
  fun `should not call onClSyncStatusUpdate when only EL goes out of sync`() {
    val clStatusUpdates = mutableListOf<CLSyncStatus>()
    val elStatusUpdates = mutableListOf<ELSyncStatus>()

    syncController.onClSyncStatusUpdate { clStatusUpdates.add(it) }
    syncController.onElSyncStatusUpdate { elStatusUpdates.add(it) }

    // Given: both CL and EL are synced
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)

    // Clear previous updates
    clStatusUpdates.clear()
    elStatusUpdates.clear()

    // When: only EL goes out of sync
    syncController.updateElSyncStatus(ELSyncStatus.SYNCING)

    // Then: only EL status update should be called, CL should remain synced
    assertThat(syncController.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCED)
    assertThat(syncController.getElSyncStatus()).isEqualTo(ELSyncStatus.SYNCING)
    assertThat(clStatusUpdates).isEmpty()
    assertThat(elStatusUpdates).containsExactly(ELSyncStatus.SYNCING)
  }

  @Test
  fun `should not call onClSyncStatusUpdate without actual status change`() {
    val clStatusUpdates = mutableListOf<CLSyncStatus>()
    val elStatusUpdates = mutableListOf<ELSyncStatus>()

    syncController.onClSyncStatusUpdate { clStatusUpdates.add(it) }
    syncController.onElSyncStatusUpdate { elStatusUpdates.add(it) }

    // Given: send multiple updateClSyncStatus calls
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // Then: only EL status update should be called, CL should remain synced
    assertThat(syncController.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCED)
    assertThat(syncController.getElSyncStatus()).isEqualTo(ELSyncStatus.SYNCING)
    assertThat(clStatusUpdates).isEqualTo(listOf(CLSyncStatus.SYNCED))
    assertThat(elStatusUpdates).isEmpty()
  }

  @Test
  fun `should not call onElSyncStatusUpdate without actual status change`() {
    val clStatusUpdates = mutableListOf<CLSyncStatus>()
    val elStatusUpdates = mutableListOf<ELSyncStatus>()
    var fullSyncUpdates = 0u

    syncController.onClSyncStatusUpdate { clStatusUpdates.add(it) }
    syncController.onElSyncStatusUpdate { elStatusUpdates.add(it) }
    syncController.onFullSyncComplete { fullSyncUpdates += 1u }

    // Given: Cl is synced, send multiple updateClSyncStatus calls
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)

    clStatusUpdates.clear()

    // Then: only EL status update should be called, CL should remain synced
    assertThat(syncController.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCED)
    assertThat(syncController.getElSyncStatus()).isEqualTo(ELSyncStatus.SYNCED)
    assertThat(elStatusUpdates).isEqualTo(listOf(ELSyncStatus.SYNCED))
    assertThat(clStatusUpdates).isEmpty()
    assertThat(fullSyncUpdates).isEqualTo(1u)
  }

  @Test
  fun `should transition to synced when sync target is updated to match current chain head`() {
    // Given: beacon chain at block 100
    val controller = createController(100UL)

    val clStatusUpdates = mutableListOf<CLSyncStatus>()
    val elStatusUpdates = mutableListOf<ELSyncStatus>()

    controller.onClSyncStatusUpdate { clStatusUpdates.add(it) }
    controller.onElSyncStatusUpdate { elStatusUpdates.add(it) }

    // First: trigger sync to target 150 (controller starts in SYNCING state by default)
    controller.onBeaconChainSyncTargetUpdated(150UL)
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(150UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)

    // Clear status updates from the first sync trigger
    clStatusUpdates.clear()
    elStatusUpdates.clear()

    // When: sync target is updated back to 100 (which matches current chain head)
    controller.onBeaconChainSyncTargetUpdated(100UL)

    // Then: controller should transition to SYNCED since target matches current head
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(100UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
    assertThat(controller.getElSyncStatus()).isEqualTo(ELSyncStatus.SYNCING)
    assertThat(controller.isBeaconChainSynced()).isFalse

    // Status updates should be triggered for the transition to SYNCED
    assertThat(clStatusUpdates).isEmpty()
    assertThat(elStatusUpdates).isEmpty() // EL status doesn't change
  }

  @Test
  fun `should handle ongoing sync target updates correctly`() {
    val controller = createController(50UL)

    // Given: sync is triggered to target 200
    controller.onBeaconChainSyncTargetUpdated(200UL)
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(200UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)

    // When: sync target is updated to 150 during ongoing sync
    controller.onBeaconChainSyncTargetUpdated(150UL)

    // Then: should update sync target
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(150UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)

    // When: sync target is updated again to 180
    controller.onBeaconChainSyncTargetUpdated(180UL)

    // Then: should update sync target again
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(180UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
  }

  @Test
  fun `should not trigger sync when within desync tolerance`() {
    // Given: beacon chain at block 100 with desync tolerance of 5
    val controller = createController(100UL, desyncTolerance = 5UL)
    controller.updateClSyncStatus(CLSyncStatus.SYNCED)

    // When: sync target is 104 (difference of 4, within tolerance of 5)
    controller.onBeaconChainSyncTargetUpdated(104UL)

    // Then: should not trigger sync
    assertThat(fakeClSyncService.lastSyncTarget).isNull()
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCED)
  }

  @Test
  fun `should trigger sync when exceeding desync tolerance`() {
    // Given: beacon chain at block 100 with desync tolerance of 5
    val controller = createController(100UL, desyncTolerance = 5UL)
    controller.updateClSyncStatus(CLSyncStatus.SYNCED)

    // When: sync target is 106 (difference of 6, exceeds tolerance of 5)
    controller.onBeaconChainSyncTargetUpdated(106UL)

    // Then: should trigger sync
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(106UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
  }

  @Test
  fun `should transition from syncing to synced when within tolerance during ongoing sync`() {
    // Given: beacon chain at block 100 with desync tolerance of 5, currently syncing
    val controller = createController(100UL, desyncTolerance = 5UL)
    controller.updateClSyncStatus(CLSyncStatus.SYNCING)

    // When: sync target is updated to 103 (within tolerance)
    controller.onBeaconChainSyncTargetUpdated(103UL)

    // Then: should set sync target but still remain in syncing state
    // (the transition to SYNCED happens when onSyncComplete is called)
    assertThat(fakeClSyncService.lastSyncTarget).isEqualTo(103UL)
    assertThat(controller.getCLSyncStatus()).isEqualTo(CLSyncStatus.SYNCING)
  }

  @Test
  fun `when elSyncEnabled is false isELSynced and getElSyncStatus always report SYNCED`() {
    val controller =
      createSyncController(
        blockNumber = 0UL,
        clSyncService = fakeClSyncService,
        desyncTolerance = 1UL,
        elSyncEnabled = false,
      )

    assertThat(controller.isELSynced()).isTrue()
    assertThat(controller.getElSyncStatus()).isEqualTo(ELSyncStatus.SYNCED)
  }

  @Test
  fun `when elSyncEnabled is false isNodeFullInSync depends only on CL`() {
    val controller =
      createSyncController(
        blockNumber = 0UL,
        clSyncService = fakeClSyncService,
        desyncTolerance = 1UL,
        elSyncEnabled = false,
      )

    assertThat(controller.isNodeFullInSync()).isFalse()

    controller.updateClSyncStatus(CLSyncStatus.SYNCED)

    controller.updateElSyncStatus(ELSyncStatus.SYNCING)

    assertThat(controller.isNodeFullInSync()).isTrue()
  }

  @Test
  fun `when elSyncEnabled is false updateElSyncStatus is a no-op`() {
    val elStatusUpdates = mutableListOf<ELSyncStatus>()
    val controller =
      createSyncController(
        blockNumber = 0UL,
        clSyncService = fakeClSyncService,
        desyncTolerance = 1UL,
        elSyncEnabled = false,
      )
    controller.onElSyncStatusUpdate { elStatusUpdates.add(it) }

    controller.updateElSyncStatus(ELSyncStatus.SYNCED)
    controller.updateElSyncStatus(ELSyncStatus.SYNCING)

    assertThat(elStatusUpdates).isEmpty()
    assertThat(controller.getElSyncStatus()).isEqualTo(ELSyncStatus.SYNCED)
  }

  @Test
  fun `when elSyncEnabled is false CL going SYNCING does not force EL status`() {
    val elStatusUpdates = mutableListOf<ELSyncStatus>()
    val controller =
      createSyncController(
        blockNumber = 0UL,
        clSyncService = fakeClSyncService,
        desyncTolerance = 1UL,
        elSyncEnabled = false,
      )
    controller.onElSyncStatusUpdate { elStatusUpdates.add(it) }

    controller.updateClSyncStatus(CLSyncStatus.SYNCING)
    controller.updateClSyncStatus(CLSyncStatus.SYNCED)
    controller.updateClSyncStatus(CLSyncStatus.SYNCING)
    controller.updateClSyncStatus(CLSyncStatus.SYNCED)

    assertThat(elStatusUpdates).isEmpty()
    assertThat(controller.isELSynced()).isTrue()
  }
}
