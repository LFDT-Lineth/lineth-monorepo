/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.discovery

import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.timer.Timer
import linea.timer.TimerFactory
import linea.timer.TimerSchedule
import linea.timer.VertxTimerFactory
import maru.config.P2PConfig
import maru.database.P2PState
import maru.p2p.fork.ForkPeeringManager
import maru.services.LongRunningService
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.units.bigints.UInt64
import org.ethereum.beacon.discovery.DiscoverySystem
import org.ethereum.beacon.discovery.DiscoverySystemBuilder
import org.ethereum.beacon.discovery.crypto.DefaultSigner
import org.ethereum.beacon.discovery.schema.EnrField
import org.ethereum.beacon.discovery.schema.NodeRecord
import org.ethereum.beacon.discovery.schema.NodeRecordBuilder
import org.ethereum.beacon.discovery.schema.NodeRecordFactory
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.networking.p2p.discovery.DiscoveryPeer
import tech.pegasys.teku.networking.p2p.discovery.discv5.SecretKeyParser
import java.util.Optional
import java.util.concurrent.CompletableFuture
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

class MaruDiscoveryService(
  privateKeyBytes: ByteArray,
  private val p2pConfig: P2PConfig,
  private val forkIdHashManager: ForkPeeringManager,
  private val timerFactory: TimerFactory,
  private val p2PState: P2PState,
) : LongRunningService {
  init {
    require(p2pConfig.discovery != null) {
      "MaruDiscoveryService is being initialized without the discovery section in the P2P config!"
    }
    require(p2pConfig.discovery!!.port != 0u) {
      "MaruDiscoveryService requires discovery port to be set to a non zero value!"
    }
    require(p2pConfig.port != 0u) {
      "MaruDiscoveryService requires p2p port to be set to a non zero value!"
    }
  }

  companion object {
    private val log = LogManager.getLogger(MaruDiscoveryService::class.java)
    const val FORK_ID_HASH_FIELD_NAME = "mfidh"

    internal fun isValidNodeRecord(
      forkIdHashManager: ForkPeeringManager,
      node: NodeRecord,
    ): Boolean {
      if (node.get(FORK_ID_HASH_FIELD_NAME) == null) {
        log.trace("Node record is missing forkId field: {}", node)
        return false
      }
      val forkId =
        (node.get(FORK_ID_HASH_FIELD_NAME) as? Bytes) ?: run {
          log.trace("Failed to cast value for the forkId hash to Bytes")
          return false
        }
      if (!forkIdHashManager.isValidForPeering(forkId.toArray())) {
        log.trace(
          "peer={} is on a different forkId: localForkId={} peerForkId={}",
          node.nodeId,
          forkIdHashManager.currentForkHash().encodeHex(),
          forkId,
        )
        return false
      }
      if (node.get(EnrField.PKEY_SECP256K1) == null) {
        log.trace("Node record is missing public key field: {}", node)
        return false
      }
      (node.get(EnrField.PKEY_SECP256K1) as? Bytes) ?: run {
        log.trace("Failed to cast value for the public key to Bytes")
        return false
      }
      if (node.tcpAddress.isEmpty) {
        log.trace(
          "node record doesn't have a TCP address: {}",
          node,
        )
        return false
      }
      return true
    }

    internal fun convertSafeNodeRecordToDiscoveryPeer(node: NodeRecord): DiscoveryPeer {
      // node record has been checked in checkNodeRecord, so we can convert to MaruDiscoveryPeer safely
      return DiscoveryPeer(
        node.get(EnrField.PKEY_SECP256K1) as Bytes,
        node.nodeId,
        node.tcpAddress.get(),
        null,
        null,
        null,
        Optional.empty(),
        Optional.empty(),
      )
    }
  }

  private val log: Logger = LogManager.getLogger(this.javaClass)

  private val privateKey = SecretKeyParser.fromLibP2pPrivKey(Bytes.wrap(privateKeyBytes))

  private val signer = DefaultSigner(privateKey)

  private val bootnodes =
    p2pConfig.discovery!!
      .bootnodes
      .map { NodeRecordFactory.DEFAULT.fromEnr(it) }
      .toList()

  private val discoverySystem: DiscoverySystem =
    DiscoverySystemBuilder()
      .listen(p2pConfig.ipAddress, p2pConfig.discovery!!.port.toInt())
      .signer(signer)
      .localNodeRecord(createLocalNodeRecord())
      .localNodeRecordListener { _, newRecord -> localNodeRecordUpdated(newRecord) }
      .build()

  private var poller: Timer? = null

  private val isStarted = AtomicBoolean(false)

  fun getLocalNodeRecord(): NodeRecord = discoverySystem.localNodeRecord

  override fun start(): CompletableFuture<Unit> {
    if (!isStarted.compareAndSet(false, true)) {
      log.warn("MaruDiscoveryService has already been started!")
      return SafeFuture.completedFuture(Unit)
    }
    return discoverySystem
      .start()
      .thenRun {
        poller = timerFactory.createTimer(
          name = "boot-node-refresher",
          initialDelay = if (timerFactory is VertxTimerFactory) 1.milliseconds else Duration.ZERO,
          period = p2pConfig.discovery!!.refreshInterval,
          timerSchedule = TimerSchedule.FIXED_RATE,
          errorHandler = { e -> log.warn("Failed to ping bootnodes", e) },
          task = Runnable { pingBootnodes() },
        )
        poller!!.start()
      }.thenApply {}
  }

  override fun stop(): CompletableFuture<Unit> {
    if (!isStarted.compareAndSet(true, false)) {
      log.warn("Calling stop on MaruDiscoveryService that has not been started!")
      return SafeFuture.completedFuture(Unit)
    }
    poller?.stop()
    discoverySystem.stop()
    return SafeFuture.completedFuture(Unit)
  }

  fun updateForkIdHash(forkIdHash: ByteArray) {
    discoverySystem.updateCustomFieldValue(
      FORK_ID_HASH_FIELD_NAME,
      Bytes.wrap(forkIdHash),
    )
  }

  fun searchForPeers(): SafeFuture<Collection<DiscoveryPeer>> =
    discoverySystem
      .searchForNewPeers()
      // The current version of discovery doesn't return the found peers but next version will
      .toSafeFuture()
      .thenApply { getKnownPeers() }

  fun getKnownPeers(): Collection<DiscoveryPeer> =
    discoverySystem
      .streamLiveNodes()
      .filter { isValidNodeRecord(forkIdHashManager, it) }
      .map { node: NodeRecord ->
        convertSafeNodeRecordToDiscoveryPeer(node)
      }.toList()
      .toSet()

  private fun pingBootnodes() {
    log.trace("Pinging bootnodes")
    bootnodes.forEach {
      SafeFuture
        .of(discoverySystem.ping(it))
        .whenException { e ->
          log.warn("bootnode={} is unresponsive: errorMessage={}", it, e.message, e)
        }
    }
  }

  private fun createLocalNodeRecord(): NodeRecord {
    val sequenceNumber = p2PState.getLocalNodeRecordSequenceNumber() + 1uL
    p2PState
      .newP2PStateUpdater()
      .putDiscoverySequenceNumber(sequenceNumber)
      .commit()
    val nodeRecordBuilder: NodeRecordBuilder =
      NodeRecordBuilder()
        .signer(signer)
        .seq(UInt64.valueOf(sequenceNumber.toBigInteger()))
        .address(
          /* ipAddress = */
          p2pConfig.discovery!!.advertisedIp ?: p2pConfig.ipAddress,
          /* udpPort = */
          p2pConfig.discovery!!.port.toInt(),
          /* tcpPort = */
          p2pConfig.port.toInt(),
        ).customField(FORK_ID_HASH_FIELD_NAME, Bytes.wrap(forkIdHashManager.currentForkHash()))
    // TODO: do we want more custom fields to identify version/topics/role/something else?

    return nodeRecordBuilder.build()
  }

  private fun localNodeRecordUpdated(newRecord: NodeRecord) {
    log.info("Node record updated, enr={}", newRecord.asEnr())
    p2PState
      .newP2PStateUpdater()
      .putDiscoverySequenceNumber(newRecord.seq.toBigInteger().toULong())
      .commit()
  }
}
