/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

import linea.kotlin.minusCoercingUnderflow
import linea.timer.TimerFactory
import maru.consensus.ValidatorProvider
import maru.database.BeaconChain
import maru.p2p.PeerLookup
import maru.p2p.PeersHeadBlockProvider
import maru.services.LongRunningService
import maru.subscription.InOrderFanoutSubscriptionManager
import maru.syncing.beaconchain.CLSyncServiceImpl
import maru.syncing.beaconchain.pipeline.BeaconChainDownloadPipelineFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.plugin.services.MetricsSystem
import java.util.concurrent.CompletableFuture
import java.util.concurrent.Executors
import java.util.concurrent.locks.ReentrantReadWriteLock
import kotlin.concurrent.read
import kotlin.concurrent.write

internal data class SyncState(
  val clStatus: CLSyncStatus,
  val elStatus: ELSyncStatus,
)

class BeaconSyncControllerImpl(
  private val beaconChain: BeaconChain,
  private val clSyncService: CLSyncService,
  private val desyncTolerance: ULong,
  clState: CLSyncStatus = CLSyncStatus.SYNCING,
  elState: ELSyncStatus = ELSyncStatus.SYNCING,
  private val elSyncEnabled: Boolean = true,
) : SyncStatusProvider,
  BeaconSyncTargetUpdateHandler {
  private val log = LogManager.getLogger(this.javaClass)
  private val lock = ReentrantReadWriteLock()
  private var currentState = SyncState(clState, if (elSyncEnabled) elState else ELSyncStatus.SYNCED)

  private val clSyncHandlers = InOrderFanoutSubscriptionManager<CLSyncStatus>()
  private val elSyncHandlers = InOrderFanoutSubscriptionManager<ELSyncStatus>()
  private val beaconSyncCompleteHandlers = InOrderFanoutSubscriptionManager<Unit>()
  private val fullSyncCompleteHandlers = InOrderFanoutSubscriptionManager<Unit>()

  init {
    clSyncService.onSyncComplete { syncTarget ->
      updateClSyncStatus(CLSyncStatus.SYNCED)
    }
    onFullSyncComplete { log.info("Maru is fully synced now") }
  }

  override fun getCLSyncStatus(): CLSyncStatus = lock.read { currentState.clStatus }

  override fun getElSyncStatus(): ELSyncStatus = lock.read { currentState.elStatus }

  override fun isNodeFullInSync(): Boolean = lock.read { isELSynced() && isBeaconChainSynced() }

  override fun isBeaconChainSynced(): Boolean = lock.read { currentState.clStatus == CLSyncStatus.SYNCED }

  override fun isELSynced(): Boolean = lock.read { currentState.elStatus == ELSyncStatus.SYNCED }

  override fun onClSyncStatusUpdate(handler: (CLSyncStatus) -> Unit) {
    clSyncHandlers.addSyncSubscriber(handler)
  }

  override fun onElSyncStatusUpdate(handler: (ELSyncStatus) -> Unit) {
    elSyncHandlers.addSyncSubscriber(handler)
  }

  override fun onBeaconSyncComplete(handler: () -> Unit) {
    beaconSyncCompleteHandlers.addSyncSubscriber(handler.toString()) { handler() }
  }

  override fun onFullSyncComplete(handler: () -> Unit) {
    fullSyncCompleteHandlers.addSyncSubscriber(handler.toString()) { handler() }
  }

  override fun getBeaconSyncDistance(): ULong = clSyncService.getSyncDistance()

  fun updateClSyncStatus(newStatus: CLSyncStatus) {
    val callbacks: List<() -> Unit> =
      lock.write {
        log.debug("Updating CL sync status to {}", newStatus)

        val previousState = currentState

        if (previousState.clStatus == newStatus) return@write emptyList()

        currentState = if (newStatus == CLSyncStatus.SYNCING && elSyncEnabled) {
          previousState.copy(clStatus = newStatus, elStatus = ELSyncStatus.SYNCING)
        } else {
          previousState.copy(clStatus = newStatus)
        }

        buildList {
          if (previousState.elStatus == ELSyncStatus.SYNCED && currentState.elStatus == ELSyncStatus.SYNCING) {
            add { elSyncHandlers.notifySubscribers(currentState.elStatus) }
          }
          add { clSyncHandlers.notifySubscribers(newStatus) }

          // If CL is SYNCED, beacon sync is complete
          if (isBeaconChainSynced()) {
            log.debug("Beacon chain is synced now")
            add { beaconSyncCompleteHandlers.notifySubscribers(Unit) }
          }

          // Check if this makes the node fully synced
          addNodeFullInSyncNotification(this)
        }
      }
    log.trace("firing CL sync status update callbacks: {}", callbacks.size)

    // Fire callbacks outside of lock
    callbacks.forEach { it() }
  }

  fun updateElSyncStatus(newStatus: ELSyncStatus) {
    if (!elSyncEnabled) return
    val callbacks: List<() -> Unit> =
      lock.write {
        log.debug("Updating EL sync status to {}", newStatus)
        val previousState = currentState

        if (previousState.elStatus == newStatus) {
          return@write emptyList()
        }

        // Enforce invariant: EL cannot be SYNCED when CL is SYNCING
        currentState = if (previousState.clStatus == CLSyncStatus.SYNCING) {
          previousState
        } else {
          previousState.copy(elStatus = newStatus)
        }

        buildList {
          add { elSyncHandlers.notifySubscribers(currentState.elStatus) }

          // Check if this makes the node fully synced
          addNodeFullInSyncNotification(this)
        }
      }

    log.trace("firing EL sync status update callbacks: {}", callbacks.size)

    // Fire callbacks outside of lock
    callbacks.forEach { it() }
  }

  private fun addNodeFullInSyncNotification(list: MutableList<() -> Unit>) {
    if (isNodeFullInSync()) {
      log.debug("Node is fully synced now")
      list.add { fullSyncCompleteHandlers.notifySubscribers(Unit) }
    }
  }

  override fun onBeaconChainSyncTargetUpdated(syncTargetBlockNumber: ULong) {
    val currentHead = beaconChain.getLatestBeaconState().beaconBlockHeader.number
    val blockDifference = syncTargetBlockNumber.minusCoercingUnderflow(currentHead)

    val currentClStatus = lock.read { currentState.clStatus }

    if (currentClStatus == CLSyncStatus.SYNCING) {
      // If already syncing, always update the sync target regardless of tolerance
      log.debug(
        "Updating target while node is syncing syncTarget={} currentHead={}",
        syncTargetBlockNumber,
        currentHead,
      )
      clSyncService.setSyncTarget(syncTargetBlockNumber)
    } else if (blockDifference > desyncTolerance) {
      // Only start new sync if difference exceeds tolerance
      log.debug(
        "Node got desynced updating sync target syncTarget={} blockDifference={} currentHead={} desyncTolerance={}",
        syncTargetBlockNumber,
        blockDifference,
        currentHead,
        desyncTolerance,
      )
      updateClSyncStatus(CLSyncStatus.SYNCING)
      clSyncService.setSyncTarget(syncTargetBlockNumber)
    }
    // If not syncing and within tolerance, do nothing
  }

  override fun getCLSyncTarget(): ULong = clSyncService.getSyncTarget()

  companion object {
    fun create(
      beaconChain: BeaconChain,
      peersHeadsProvider: PeersHeadBlockProvider,
      targetChainHeadCalculator: SyncTargetSelector,
      validatorProvider: ValidatorProvider,
      peerLookup: PeerLookup,
      besuMetrics: MetricsSystem,
      metricsFacade: MetricsFacade,
      peerChainTrackerConfig: PeerChainTracker.Config,
      elSyncServiceFactory: ((ELSyncStatus) -> Unit) -> LongRunningService,
      elSyncEnabled: Boolean = true,
      desyncTolerance: ULong,
      pipelineConfig: BeaconChainDownloadPipelineFactory.Config,
      allowEmptyBlocks: Boolean = true,
      timerFactory: TimerFactory,
    ): SyncController {
      val clSyncService =
        CLSyncServiceImpl(
          beaconChain = beaconChain,
          validatorProvider = validatorProvider,
          allowEmptyBlocks = allowEmptyBlocks,
          executorService = Executors.newCachedThreadPool(Thread.ofPlatform().daemon().factory()),
          pipelineConfig = pipelineConfig,
          peerLookup = peerLookup,
          besuMetrics = besuMetrics,
          metricsFacade = metricsFacade,
        )
      val controller =
        BeaconSyncControllerImpl(
          beaconChain = beaconChain,
          clSyncService = clSyncService,
          desyncTolerance = desyncTolerance,
          elSyncEnabled = elSyncEnabled,
        )

      val elSyncService = elSyncServiceFactory(controller::updateElSyncStatus)

      val peerChainTracker =
        PeerChainTracker(
          peersHeadsProvider = peersHeadsProvider,
          beaconSyncTargetUpdateHandler = controller,
          targetChainHeadCalculator = targetChainHeadCalculator,
          config = peerChainTrackerConfig,
          beaconChain = beaconChain,
          timerFactory = timerFactory,
        )

      return SyncControllerManager(
        syncStatusController = controller,
        elSyncService = elSyncService,
        clSyncService = clSyncService,
        peerChainTracker = peerChainTracker,
      )
    }
  }
}

interface SyncController :
  SyncStatusProvider,
  LongRunningService

class SyncControllerManager(
  val syncStatusController: BeaconSyncControllerImpl,
  val elSyncService: LongRunningService,
  val clSyncService: LongRunningService,
  val peerChainTracker: PeerChainTracker,
) : SyncController,
  SyncStatusProvider by syncStatusController {
  private val log = LogManager.getLogger(this.javaClass)

  override fun start(): CompletableFuture<Unit> {
    log.debug("Starting {}", this::class.simpleName)
    return clSyncService
      .start()
      .thenCompose {
        elSyncService.start()
      }.thenCompose {
        peerChainTracker.start()
      }
  }

  override fun stop(): CompletableFuture<Unit> =
    clSyncService
      .stop()
      .thenCompose {
        elSyncService.stop()
      }.thenCompose {
        peerChainTracker.stop()
      }.thenApply {
        // Setting to default status so that SYNCING -> SYNCED will actually trigger the callbacks
        syncStatusController.updateClSyncStatus(CLSyncStatus.SYNCING)
        syncStatusController.updateElSyncStatus(ELSyncStatus.SYNCING)
      }
}

class AlwaysSyncedController(
  val beaconChain: BeaconChain,
) : SyncController {
  private val clSyncHandlers = InOrderFanoutSubscriptionManager<CLSyncStatus>()
  private val elSyncHandlers = InOrderFanoutSubscriptionManager<ELSyncStatus>()
  private val beaconSyncCompleteHandlers = InOrderFanoutSubscriptionManager<Unit>()
  private val fullSyncCompleteHandlers = InOrderFanoutSubscriptionManager<Unit>()

  override fun getCLSyncStatus(): CLSyncStatus = CLSyncStatus.SYNCED

  override fun getElSyncStatus(): ELSyncStatus = ELSyncStatus.SYNCED

  override fun onClSyncStatusUpdate(handler: (CLSyncStatus) -> Unit) {
    clSyncHandlers.addSyncSubscriber(handler)
  }

  override fun onElSyncStatusUpdate(handler: (ELSyncStatus) -> Unit) {
    elSyncHandlers.addSyncSubscriber(handler)
  }

  override fun onBeaconSyncComplete(handler: () -> Unit) {
    beaconSyncCompleteHandlers.addSyncSubscriber { handler() }
  }

  override fun onFullSyncComplete(handler: () -> Unit) {
    fullSyncCompleteHandlers.addSyncSubscriber { handler() }
  }

  override fun isBeaconChainSynced(): Boolean = true

  override fun isELSynced(): Boolean = true

  override fun getBeaconSyncDistance(): ULong = 0UL

  override fun start(): CompletableFuture<Unit> {
    elSyncHandlers.notifySubscribers(getElSyncStatus())
    clSyncHandlers.notifySubscribers(getCLSyncStatus())
    fullSyncCompleteHandlers.notifySubscribers(Unit)
    beaconSyncCompleteHandlers.notifySubscribers(Unit)
    return CompletableFuture.completedFuture(Unit)
  }

  override fun getCLSyncTarget(): ULong = beaconChain.getLatestBeaconState().beaconBlockHeader.number

  override fun stop(): CompletableFuture<Unit> = CompletableFuture.completedFuture(Unit)
}
