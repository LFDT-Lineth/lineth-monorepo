/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import org.mockito.Mockito.mock
import org.mockito.Mockito.verifyNoInteractions
import kotlin.test.Test

class QbftGossiperTest {
  private val mockQbftMessage = mock<QbftMessage>()

  private val gossiper = QbftGossiper()

  @Test
  fun `should not send replayed messages because libp2p already propagated them`() {
    gossiper.send(mockQbftMessage, replayed = true)

    verifyNoInteractions(mockQbftMessage)
  }

  @Test
  fun `should not send non-replayed messages because Besu calls ValidatorMulticaster directly`() {
    gossiper.send(mockQbftMessage, replayed = false)

    verifyNoInteractions(mockQbftMessage)
  }
}
