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

class SyncControllerSubscriptionsTest {
  private lateinit var fakeClSyncService: FakeCLSyncService
  private lateinit var syncController: BeaconSyncControllerImpl

  private fun createController(
    blockNumber: ULong,
    clSyncService: CLSyncService = fakeClSyncService,
  ): BeaconSyncControllerImpl =
    createSyncController(
      blockNumber = blockNumber,
      clSyncService = clSyncService,
      desyncTolerance = 1UL,
    )

  @BeforeEach
  fun setUp() {
    fakeClSyncService = FakeCLSyncService()
    syncController = createController(0UL)
  }

  @Test
  fun `should support multiple CL sync status handlers and notify all in order`() {
    val handler1Updates = mutableListOf<CLSyncStatus>()
    val handler2Updates = mutableListOf<CLSyncStatus>()
    val handler3Updates = mutableListOf<CLSyncStatus>()

    // Register multiple handlers
    syncController.onClSyncStatusUpdate { handler1Updates.add(it) }
    syncController.onClSyncStatusUpdate { handler2Updates.add(it) }
    syncController.onClSyncStatusUpdate { handler3Updates.add(it) }

    // When: CL status changes
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCING)

    // Then: all handlers should be notified in order
    assertThat(handler1Updates).containsExactly(CLSyncStatus.SYNCED, CLSyncStatus.SYNCING)
    assertThat(handler2Updates).containsExactly(CLSyncStatus.SYNCED, CLSyncStatus.SYNCING)
    assertThat(handler3Updates).containsExactly(CLSyncStatus.SYNCED, CLSyncStatus.SYNCING)
  }

  @Test
  fun `should support multiple EL sync status handlers and notify all in order`() {
    val handler1Updates = mutableListOf<ELSyncStatus>()
    val handler2Updates = mutableListOf<ELSyncStatus>()
    val handler3Updates = mutableListOf<ELSyncStatus>()

    // Register multiple handlers
    syncController.onElSyncStatusUpdate { handler1Updates.add(it) }
    syncController.onElSyncStatusUpdate { handler2Updates.add(it) }
    syncController.onElSyncStatusUpdate { handler3Updates.add(it) }

    // Given: CL is already synced (so EL can transition)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // When: EL status changes
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCING)

    // Then: all handlers should be notified in order
    assertThat(handler1Updates).containsExactly(ELSyncStatus.SYNCED, ELSyncStatus.SYNCING)
    assertThat(handler2Updates).containsExactly(ELSyncStatus.SYNCED, ELSyncStatus.SYNCING)
    assertThat(handler3Updates).containsExactly(ELSyncStatus.SYNCED, ELSyncStatus.SYNCING)
  }

  @Test
  fun `should support multiple beacon sync complete handlers and notify all in order`() {
    val handler1Calls = mutableListOf<String>()
    val handler2Calls = mutableListOf<String>()
    val handler3Calls = mutableListOf<String>()

    // Register multiple handlers
    syncController.onBeaconSyncComplete { handler1Calls.add("handler1") }
    syncController.onBeaconSyncComplete { handler2Calls.add("handler2") }
    syncController.onBeaconSyncComplete { handler3Calls.add("handler3") }

    // When: CL sync completes (transition from SYNCING to SYNCED)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // Then: all handlers should be called once
    assertThat(handler1Calls).containsExactly("handler1")
    assertThat(handler2Calls).containsExactly("handler2")
    assertThat(handler3Calls).containsExactly("handler3")

    // When: beacon sync completes again (after going back to syncing)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCING)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // Then: all handlers should be called again
    assertThat(handler1Calls).containsExactly("handler1", "handler1")
    assertThat(handler2Calls).containsExactly("handler2", "handler2")
    assertThat(handler3Calls).containsExactly("handler3", "handler3")
  }

  @Test
  fun `should support multiple full sync complete handlers and notify all in order`() {
    val handler1Calls = mutableListOf<String>()
    val handler2Calls = mutableListOf<String>()
    val handler3Calls = mutableListOf<String>()

    // Register multiple handlers
    syncController.onFullSyncComplete { handler1Calls.add("handler1") }
    syncController.onFullSyncComplete { handler2Calls.add("handler2") }
    syncController.onFullSyncComplete { handler3Calls.add("handler3") }

    // Given: both are syncing
    syncController.updateClSyncStatus(CLSyncStatus.SYNCING)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCING)

    // When: CL completes first, then EL completes
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)

    // Then: all handlers should be called once (full sync complete)
    assertThat(handler1Calls).containsExactly("handler1")
    assertThat(handler2Calls).containsExactly("handler2")
    assertThat(handler3Calls).containsExactly("handler3")

    // When: full sync completes again (after going back to syncing)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCING)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCING)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)

    // Then: all handlers should be called again
    assertThat(handler1Calls).containsExactly("handler1", "handler1")
    assertThat(handler2Calls).containsExactly("handler2", "handler2")
    assertThat(handler3Calls).containsExactly("handler3", "handler3")
  }

  @Test
  fun `should notify multiple handlers for complex sync state transitions`() {
    val clStatusUpdates = mutableListOf<CLSyncStatus>()
    val elStatusUpdates = mutableListOf<ELSyncStatus>()
    val beaconSyncCompleteCalls = mutableListOf<String>()
    val fullSyncCompleteCalls = mutableListOf<String>()

    // Register multiple handlers for each event type
    syncController.onClSyncStatusUpdate { clStatusUpdates.add(it) }
    syncController.onElSyncStatusUpdate { elStatusUpdates.add(it) }
    syncController.onBeaconSyncComplete { beaconSyncCompleteCalls.add("beacon") }
    syncController.onFullSyncComplete { fullSyncCompleteCalls.add("full") }

    // Add second set of handlers
    val clStatusUpdates2 = mutableListOf<CLSyncStatus>()
    val elStatusUpdates2 = mutableListOf<ELSyncStatus>()
    syncController.onClSyncStatusUpdate { clStatusUpdates2.add(it) }
    syncController.onElSyncStatusUpdate { elStatusUpdates2.add(it) }
    syncController.onBeaconSyncComplete { beaconSyncCompleteCalls.add("beacon2") }
    syncController.onFullSyncComplete { fullSyncCompleteCalls.add("full2") }

    // Scenario: Complete sync cycle
    // 1. CL completes sync (SYNCING -> SYNCED)
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // 2. EL completes sync (SYNCING -> SYNCED)
    syncController.updateElSyncStatus(ELSyncStatus.SYNCED)

    // Verify all handlers were called correctly
    assertThat(clStatusUpdates).containsExactly(CLSyncStatus.SYNCED)
    assertThat(clStatusUpdates2).containsExactly(CLSyncStatus.SYNCED)
    assertThat(elStatusUpdates).containsExactly(ELSyncStatus.SYNCED)
    assertThat(elStatusUpdates2).containsExactly(ELSyncStatus.SYNCED)
    assertThat(beaconSyncCompleteCalls).containsExactly("beacon", "beacon2")
    assertThat(fullSyncCompleteCalls).containsExactly("full", "full2")
  }

  @Test
  fun `should handle handler exceptions gracefully and continue notifying other handlers`() {
    val successfulHandler1Calls = mutableListOf<CLSyncStatus>()
    val successfulHandler2Calls = mutableListOf<CLSyncStatus>()

    // Register handlers with one that throws exception
    syncController.onClSyncStatusUpdate { successfulHandler1Calls.add(it) }
    syncController.onClSyncStatusUpdate { throw RuntimeException("Test exception") }
    syncController.onClSyncStatusUpdate { successfulHandler2Calls.add(it) }

    // When: CL status changes
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // Then: successful handlers should still be called despite exception in middle handler
    assertThat(successfulHandler1Calls).containsExactly(CLSyncStatus.SYNCED)
    assertThat(successfulHandler2Calls).containsExactly(CLSyncStatus.SYNCED)
  }

  @Test
  fun `should support registration of handlers after sync events have occurred`() {
    val lateHandler1Calls = mutableListOf<CLSyncStatus>()
    val lateHandler2Calls = mutableListOf<CLSyncStatus>()

    // When: sync status changes before handlers are registered
    syncController.updateClSyncStatus(CLSyncStatus.SYNCED)

    // Then: register handlers after the event
    syncController.onClSyncStatusUpdate { lateHandler1Calls.add(it) }
    syncController.onClSyncStatusUpdate { lateHandler2Calls.add(it) }

    // When: another sync status change occurs
    syncController.updateClSyncStatus(CLSyncStatus.SYNCING)

    // Then: late handlers should be notified of new events
    assertThat(lateHandler1Calls).containsExactly(CLSyncStatus.SYNCING)
    assertThat(lateHandler2Calls).containsExactly(CLSyncStatus.SYNCING)
  }
}
