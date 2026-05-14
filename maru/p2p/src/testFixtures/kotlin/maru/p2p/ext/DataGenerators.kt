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
import maru.core.ext.DataGenerators
import maru.p2p.GossipMessageType
import maru.p2p.Message
import maru.p2p.MessageData
import maru.p2p.Version

object DataGenerators {
  fun randomBlockMessage(blockNumber: ULong = 1uL): Message<SealedBeaconBlock, GossipMessageType> {
    val sealedBeaconBlock = DataGenerators.randomSealedBeaconBlock(blockNumber)
    return MessageData(GossipMessageType.BEACON_BLOCK, Version.V1, sealedBeaconBlock)
  }
}
