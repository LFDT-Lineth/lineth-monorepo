/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain.pipeline

import maru.consensus.blockimport.SealedBeaconBlockImporter
import maru.extensions.encodeHex
import maru.p2p.ValidationResult
import maru.p2p.ValidationResultCode
import maru.p2p.ValidationResultCode.ACCEPT
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.util.log.LogUtil
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.reputation.ReputationAdjustment
import java.util.concurrent.atomic.AtomicBoolean
import java.util.function.Consumer

class ImportBlocksStep(
  private val blockImporter: SealedBeaconBlockImporter<ValidationResult>,
) : Consumer<List<SealedBlockWithPeer>> {
  private val log: Logger = LogManager.getLogger(this.javaClass)
  private val shouldLog = AtomicBoolean(true)

  override fun accept(blocksWithPeers: List<SealedBlockWithPeer>) {
    // Process blocks sequentially
    blocksWithPeers.forEach { blockAndPeer ->
      val beaconBlockHeader = blockAndPeer.sealedBeaconBlock.beaconBlock.beaconBlockHeader
      try {
        val result = blockImporter.importBlock(blockAndPeer.sealedBeaconBlock).join()
        when (result.code) {
          ACCEPT -> {
            LogUtil.throttledLog(
              log::info,
              "Imported block: " +
                "clBlockNumber=${beaconBlockHeader.number} " +
                "clBlockHash=${beaconBlockHeader.hash.encodeHex()}",
              shouldLog,
              30,
            )
          }
          ValidationResultCode.REJECT -> {
            blockAndPeer.peer.disconnectCleanly(DisconnectReason.REMOTE_FAULT)
            log.error(
              "Block validation failed for block: clBlockNumber:{} clBlockHash={}",
              beaconBlockHeader.number,
              beaconBlockHeader.hash.encodeHex(),
            )
            return
          }
          ValidationResultCode.IGNORE -> {
            log.warn(
              "Block validation ignored for block: clBlockNumber:{}, clBlockHash={}",
              beaconBlockHeader.number,
              beaconBlockHeader.hash.encodeHex(),
            )
            return
          }
        }
      } catch (e: Exception) {
        log.error(
          "Exception importing block: clBlockNumber:{}, clBlockHash={}",
          beaconBlockHeader.number,
          beaconBlockHeader.hash
            .encodeHex(),
          e,
        )
        throw e
      }
    }
    if (blocksWithPeers.isNotEmpty()) {
      // get a list of peers that have provided at least one block and reward them
      blocksWithPeers.stream().map({ it.peer }).distinct().forEach(
        Consumer { peer ->
          peer.adjustReputation(ReputationAdjustment.SMALL_REWARD)
        },
      )
    }
  }
}
