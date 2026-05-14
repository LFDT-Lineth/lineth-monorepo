/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import io.libp2p.pubsub.MessageAlreadySeenException
import io.libp2p.pubsub.NoPeersForOutboundMessageException
import maru.p2p.GossipMessageType
import maru.p2p.Message
import maru.p2p.NoOpP2PNetwork
import maru.p2p.P2PNetwork
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatNoException
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.ethereum.p2p.rlpx.wire.MessageData
import org.mockito.Mockito.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import kotlin.test.Test

class P2PValidatorMulticasterTest {
  private val message = mock<MessageData>()

  @Test
  fun `should wait for broadcast completion`() {
    val broadcastFuture = SafeFuture<Any?>()
    val multicaster = P2PValidatorMulticaster(networkReturning(broadcastFuture))
    val executor = Executors.newSingleThreadExecutor()

    try {
      val sendFuture = executor.submit { multicaster.send(message) }

      Thread.sleep(100)
      assertThat(sendFuture.isDone).isFalse()

      broadcastFuture.complete(Unit)

      assertThatNoException().isThrownBy {
        sendFuture.get(1, TimeUnit.SECONDS)
      }
    } finally {
      executor.shutdownNow()
    }
  }

  @Test
  fun `should ignore already seen messages`() {
    assertBroadcastFailureIsIgnored(MessageAlreadySeenException("already seen"))
  }

  @Test
  fun `should ignore missing gossip peers`() {
    assertBroadcastFailureIsIgnored(NoPeersForOutboundMessageException("no peers"))
  }

  @Test
  fun `should propagate unexpected broadcast failures`() {
    val multicaster = P2PValidatorMulticaster(
      networkReturning(SafeFuture.failedFuture<Any?>(IllegalStateException("network failure"))),
    )

    assertThatThrownBy {
      multicaster.send(message)
    }.hasRootCauseInstanceOf(IllegalStateException::class.java)
  }

  private fun assertBroadcastFailureIsIgnored(failure: Throwable) {
    val multicaster = P2PValidatorMulticaster(networkReturning(SafeFuture.failedFuture<Any?>(failure)))

    assertThatNoException().isThrownBy {
      multicaster.send(message)
    }
  }

  private fun networkReturning(future: SafeFuture<*>): P2PNetwork =
    object : P2PNetwork by NoOpP2PNetwork {
      override fun broadcastMessage(message: Message<*, GossipMessageType>): SafeFuture<*> = future
    }
}
