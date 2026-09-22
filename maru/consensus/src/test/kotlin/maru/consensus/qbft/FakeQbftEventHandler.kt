/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.hyperledger.besu.consensus.common.bft.events.BlockTimerExpiry
import org.hyperledger.besu.consensus.common.bft.events.RoundExpiry
import org.hyperledger.besu.consensus.qbft.core.types.QbftEventHandler
import org.hyperledger.besu.consensus.qbft.core.types.QbftNewChainHead
import org.hyperledger.besu.consensus.qbft.core.types.QbftReceivedMessageEvent

internal class FakeQbftEventHandler(
  private val onBlockTimerExpiry: (BlockTimerExpiry) -> Unit = {},
) : QbftEventHandler {
  var starts = 0
    private set
  var stops = 0
    private set

  override fun start() {
    starts++
  }

  override fun stop() {
    stops++
  }

  override fun handleMessageEvent(event: QbftReceivedMessageEvent) = Unit

  override fun handleNewBlockEvent(event: QbftNewChainHead) = Unit

  override fun handleRoundExpiry(event: RoundExpiry) = Unit

  override fun handleBlockTimerExpiry(event: BlockTimerExpiry) = onBlockTimerExpiry(event)
}
