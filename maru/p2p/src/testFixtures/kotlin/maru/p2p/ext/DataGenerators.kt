/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.ext

import maru.core.SealedBeaconBlock
import maru.p2p.GossipMessageType
import maru.p2p.Message
import maru.p2p.MessageData
import maru.p2p.Version
import maru.p2p.messages.Status
import kotlin.random.Random
import maru.core.ext.DataGenerators as CoreDataGenerators

object DataGenerators {
  fun randomStatus(latestBlockNumber: ULong): Status =
    Status(
      forkIdHash = Random.nextBytes(32),
      latestStateRoot = Random.nextBytes(32),
      latestBlockNumber = latestBlockNumber,
    )

  fun randomBlockMessage(blockNumber: ULong = 1uL): Message<SealedBeaconBlock, GossipMessageType> {
    val sealedBeaconBlock =
      CoreDataGenerators.randomSealedBeaconBlock(
        blockNumber,
      )
    return MessageData(GossipMessageType.BEACON_BLOCK, Version.V1, sealedBeaconBlock)
  }
}
