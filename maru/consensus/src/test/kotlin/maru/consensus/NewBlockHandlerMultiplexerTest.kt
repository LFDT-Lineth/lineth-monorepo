/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.core.ext.DataGenerators
import maru.extensions.encodeHex
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.doThrow
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.text.contains

class NewBlockHandlerMultiplexerTest {
  @Test
  fun `should invoke all handlers for BeaconBlock`() {
    val block = DataGenerators.randomBeaconBlock(1u)
    val handler1 =
      mock<NewBlockHandler<Unit>> {
        on { handleNewBlock(any()) } doReturn SafeFuture.completedFuture(Unit)
      }
    val handler2 =
      mock<NewBlockHandler<Unit>> {
        on { handleNewBlock(any()) } doReturn SafeFuture.completedFuture(Unit)
      }
    val multiplexer =
      NewBlockHandlerMultiplexer(
        mapOf("h1" to handler1, "h2" to handler2),
      )

    val future = multiplexer.handleNewBlock(block)
    future.join()

    verify(handler1).handleNewBlock(block)
    verify(handler2).handleNewBlock(block)
    assertThat(future.isDone).isTrue()
  }

  @Test
  fun `should log and throw error if handler throws`() {
    val block = DataGenerators.randomBeaconBlock(1u)
    val handler =
      mock<NewBlockHandler<Unit>> {
        on { handleNewBlock(any()) } doThrow RuntimeException("fail")
      }
    val logger: Logger = mock<Logger>()
    val multiplexer = NewBlockHandlerMultiplexer(handlersMap = mapOf(pair = "h" to handler), log = logger)

    assertThrows<Throwable> {
      multiplexer.handleNewBlock(block).get()
    }
    verify(logger).error(
      eq("new block handling failed: handler={} clBlockNumber={} elBlockNumber={} clBlockHash={} errorMessage={}"),
      eq("h"),
      eq(block.beaconBlockHeader.number),
      eq(block.beaconBlockBody.executionPayload.blockNumber),
      eq(block.beaconBlockHeader.hash.encodeHex()),
      eq("fail"),
      any<Throwable>(),
    )
  }
}
