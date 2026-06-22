/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.blockimport.SealedBeaconBlockImporter
import maru.core.Protocol
import maru.p2p.P2PNetwork
import maru.p2p.ValidationResult
import org.apache.logging.log4j.LogManager

class QbftConsensusFollower(
  private val p2pNetwork: P2PNetwork,
  private val blockImporter: SealedBeaconBlockImporter<ValidationResult>,
) : Protocol {
  private val log = LogManager.getLogger(this.javaClass)
  val subscriptionId = p2pNetwork.subscribeToBlocks(blockImporter::importBlock)

  override fun start() {}

  override fun pause() {}

  override fun close() {
    log.info("Stopping the QbftConsensusFollower block import")
    p2pNetwork.unsubscribeFromBlocks(subscriptionId)
  }
}
