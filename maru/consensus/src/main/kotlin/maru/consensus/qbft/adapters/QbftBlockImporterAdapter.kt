/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.blockimport.SealedBeaconBlockImporter
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockImporter
import org.hyperledger.besu.ethereum.mainnet.block.access.list.BlockAccessList
import java.util.Optional

/**
 * Responsible for: transactional  and El node
 * 1. state transition of node's BeaconChain
 * 2. new block import into an EL node
 * The import is transactional, I.e. all or nothing approach
 */
class QbftBlockImporterAdapter(
  private val sealedBeaconBlockImporter: SealedBeaconBlockImporter<*>,
) : QbftBlockImporter {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  override fun importBlock(
    qbftBlock: QbftBlock,
    blockAccessList: Optional<BlockAccessList>,
  ): Boolean {
    val sealedBeaconBlock = qbftBlock.toSealedBeaconBlock()
    try {
      sealedBeaconBlockImporter.importBlock(sealedBeaconBlock).get()
    } catch (e: Exception) {
      log.error("Block import failed: ${e.message}", e)
      return false
    }
    return true
  }
}
