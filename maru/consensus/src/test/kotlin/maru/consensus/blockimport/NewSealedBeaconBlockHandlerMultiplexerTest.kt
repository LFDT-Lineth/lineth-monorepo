/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.blockimport

import maru.core.ext.DataGenerators
import maru.p2p.SealedBeaconBlockHandler
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.kotlin.any
import org.mockito.kotlin.argThat
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.doThrow
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.text.contains

class NewSealedBeaconBlockHandlerMultiplexerTest {
  @Test
  fun `should invoke all handlers for SealedBeaconBlock`() {
    val sealedBlock = DataGenerators.randomSealedBeaconBlock(1u)
    val handler1 =
      mock<SealedBeaconBlockHandler<Unit>> {
        on { handleSealedBlock(any()) } doReturn SafeFuture.completedFuture(Unit)
      }
    val handler2 =
      mock<SealedBeaconBlockHandler<Unit>> {
        on { handleSealedBlock(any()) } doReturn SafeFuture.completedFuture(Unit)
      }
    val multiplexer =
      NewSealedBeaconBlockHandlerMultiplexer<Unit>(
        handlersMap = mapOf("h1" to handler1, "h2" to handler2),
      )

    val future = multiplexer.handleSealedBlock(sealedBlock)
    future.join()

    verify(handler1).handleSealedBlock(sealedBlock)
    verify(handler2).handleSealedBlock(sealedBlock)
    assertThat(future.isDone).isTrue()
  }

  @Test
  fun `should log and throw error if sealed handler throws`() {
    val sealedBlock = DataGenerators.randomSealedBeaconBlock(1u)
    val handler =
      mock<SealedBeaconBlockHandler<Unit>> {
        on { handleSealedBlock(any()) } doThrow RuntimeException("fail")
      }
    val logger: Logger = mock()
    val multiplexer =
      NewSealedBeaconBlockHandlerMultiplexer<Unit>(
        handlersMap = mapOf(pair = "h" to handler),
        log = logger,
      )

    assertThrows<Throwable> {
      multiplexer.handleSealedBlock(sealedBlock).get()
    }
    verify(logger).error(
      argThat<String> {
        contains("New sealed block handler h failed processing") &&
          contains("number=${sealedBlock.beaconBlock.beaconBlockHeader.number}")
      },
      any<Throwable>(),
    )
  }
}
