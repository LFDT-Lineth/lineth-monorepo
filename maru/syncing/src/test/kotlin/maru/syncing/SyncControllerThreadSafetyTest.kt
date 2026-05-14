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
import org.junit.jupiter.api.RepeatedTest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.parallel.Execution
import org.junit.jupiter.api.parallel.ExecutionMode
import java.util.concurrent.CountDownLatch
import java.util.concurrent.CyclicBarrier
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlin.random.Random

// These tests employ multiple threads on their own, so they could use some more CPU space than usual
@Execution(ExecutionMode.SAME_THREAD)
class SyncControllerThreadSafetyTest {
  private lateinit var syncController: BeaconSyncControllerImpl

  private fun createController(
    blockNumber: ULong,
    clSyncService: CLSyncService = FakeCLSyncService(),
  ): BeaconSyncControllerImpl =
    createSyncController(
      blockNumber = blockNumber,
      clSyncService = clSyncService,
      desyncTolerance = 1UL,
    )

  @BeforeEach
  fun setUp() {
    syncController = createController(50UL)
  }

  @RepeatedTest(100)
  fun `should handle concurrent status updates without race conditions`() {
    val executor = Executors.newFixedThreadPool(3)
    val barrier = CyclicBarrier(3)
    val iterations = 10

    val clStatusUpdates = mutableListOf<CLSyncStatus>()
    val elStatusUpdates = mutableListOf<ELSyncStatus>()
    val beaconSyncCompletions = AtomicInteger(0)
    val fullSyncCompletions = AtomicInteger(0)

    // Set handlers once during initialization (as per real usage)
    syncController.onClSyncStatusUpdate { synchronized(clStatusUpdates) { clStatusUpdates.add(it) } }
    syncController.onElSyncStatusUpdate { synchronized(elStatusUpdates) { elStatusUpdates.add(it) } }
    syncController.onBeaconSyncComplete { beaconSyncCompletions.incrementAndGet() }
    syncController.onFullSyncComplete { fullSyncCompletions.incrementAndGet() }

    try {
      // Thread 1: Updates CL status
      executor.submit {
        barrier.await()
        repeat(iterations) { i ->
          val status = if (i % 2 == 0) CLSyncStatus.SYNCING else CLSyncStatus.SYNCED
          syncController.updateClSyncStatus(status)
          Thread.sleep(5) // Small delay to encourage race conditions
        }
      }

      // Thread 2: Updates EL status
      executor.submit {
        barrier.await()
        repeat(iterations) { i ->
          val status = if (i % 2 == 0) ELSyncStatus.SYNCING else ELSyncStatus.SYNCED
          syncController.updateElSyncStatus(status)
          Thread.sleep(5)
        }
      }

      // Thread 3: Chain head updates
      executor.submit {
        barrier.await()
        repeat(iterations) {
          val target = Random.nextInt(100)
          syncController.onBeaconChainSyncTargetUpdated(target.toULong())
          Thread.sleep(5)
        }
      }

      executor.shutdown()
      assertThat(executor.awaitTermination(30, TimeUnit.SECONDS)).isTrue()

      // Verify that ELSyncStatus is never SYNCED when CL status is SYNCING
      val finalClStatus = syncController.getCLSyncStatus()
      val finalElStatus = syncController.getElSyncStatus()
      val isFullInSync = syncController.isNodeFullInSync()

      assertThat(isFullInSync)
        .withFailMessage("finalElStatus = $finalElStatus, finalClStatus = $finalClStatus")
        .isEqualTo(finalClStatus == CLSyncStatus.SYNCED && finalElStatus == ELSyncStatus.SYNCED)
      assertThat(finalElStatus == ELSyncStatus.SYNCED && finalClStatus == CLSyncStatus.SYNCING)
        .withFailMessage("finalElStatus = $finalElStatus, finalClStatus = $finalClStatus")
        .isFalse

      // Verify we received status updates (exact count may vary due to concurrency)
      assertThat(clStatusUpdates).size().isGreaterThanOrEqualTo(iterations / 2)
      assertThat(elStatusUpdates).size().isGreaterThan(0)
      assertThat(beaconSyncCompletions.get()).isGreaterThanOrEqualTo(iterations / 2)
    } finally {
      if (!executor.isShutdown) {
        executor.shutdownNow()
      }
    }
  }

  @Test
  fun `should handle concurrent handler invocation`() {
    val executor = Executors.newFixedThreadPool(2)
    val iterations = 100
    val beaconSyncCompletions = AtomicInteger(0)
    val fullSyncCompletions = AtomicInteger(0)
    val latch = CountDownLatch(2)

    // Set handlers once during initialization
    syncController.onClSyncStatusUpdate { beaconSyncCompletions.incrementAndGet() }
    syncController.onElSyncStatusUpdate { fullSyncCompletions.incrementAndGet() }

    try {
      // Thread 1: Triggers CL status changes
      executor.submit {
        repeat(iterations) { i ->
          syncController.updateClSyncStatus(if (i % 2 == 0) CLSyncStatus.SYNCING else CLSyncStatus.SYNCED)
          Thread.sleep(15)
        }
        latch.countDown()
      }

      // Thread 2: Triggers EL status changes
      executor.submit {
        repeat(iterations) { i ->
          syncController.updateElSyncStatus(if (i % 2 == 0) ELSyncStatus.SYNCING else ELSyncStatus.SYNCED)
          Thread.sleep(15)
        }
        latch.countDown()
      }

      assertThat(latch.await(30, TimeUnit.SECONDS)).isTrue()

      // Should have received handler calls without crashes or deadlocks
      assertThat(beaconSyncCompletions.get()).isGreaterThanOrEqualTo(iterations - 1)
      assertThat(fullSyncCompletions.get()).isGreaterThan(0)
    } finally {
      executor.shutdownNow()
    }
  }

  @Test
  fun `should handle concurrent sync target updates atomically`() {
    val executor = Executors.newFixedThreadPool(4)
    val iterations = 1000
    val latch = CountDownLatch(4)
    val syncTargetCalls = mutableListOf<ULong>()
    val trackingService =
      object : CLSyncService {
        override fun setSyncTarget(syncTarget: ULong) {
          synchronized(syncTargetCalls) {
            syncTargetCalls.add(syncTarget)
          }
        }

        override fun onSyncComplete(handler: (ULong) -> Unit) {}

        override fun getSyncDistance(): ULong = 0UL

        override fun getSyncTarget(): ULong = 0UL
      }

    val controller = createController(50UL, trackingService)

    try {
      // Multiple threads calling onChainHeadUpdated with different targets
      repeat(4) { threadIndex ->
        executor.submit {
          repeat(iterations) { i ->
            val target = (threadIndex * 1000 + i).toULong()
            controller.onBeaconChainSyncTargetUpdated(target)
          }
          latch.countDown()
        }
      }

      assertThat(latch.await(30, TimeUnit.SECONDS)).isTrue()

      synchronized(syncTargetCalls) {
        // Should have received sync target calls without duplicates for same values
        assertThat(syncTargetCalls).isNotEmpty()

        // No two consecutive calls should have the same value
        assertThat(syncTargetCalls.toSet().size == syncTargetCalls.size).isTrue
      }
    } finally {
      executor.shutdownNow()
    }
  }
}
