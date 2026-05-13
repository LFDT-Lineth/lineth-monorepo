/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class HighestHeadTargetSelectorTest {
  private val selector = HighestHeadTargetSelector()

  @Test
  fun `should throw exception when peer heads list is empty`() {
    val exception =
      assertThrows<IllegalArgumentException> {
        selector.selectBestSyncTarget(emptyList())
      }
    assertEquals("Peer heads list cannot be empty", exception.message)
  }

  @Test
  fun `should return the only element when list has single item`() {
    val result = selector.selectBestSyncTarget(listOf(100UL))
    assertEquals(100UL, result)
  }

  @Test
  fun `should return highest value from multiple elements`() {
    val peerHeads = listOf(100UL, 300UL, 200UL, 50UL)
    val result = selector.selectBestSyncTarget(peerHeads)
    assertEquals(300UL, result)
  }

  @Test
  fun `should return highest value when all elements are the same`() {
    val peerHeads = listOf(100UL, 100UL, 100UL, 100UL)
    val result = selector.selectBestSyncTarget(peerHeads)
    assertEquals(100UL, result)
  }

  @Test
  fun `should handle very large ULong values`() {
    val peerHeads = listOf(ULong.MAX_VALUE, 100UL, 1000UL)
    val result = selector.selectBestSyncTarget(peerHeads)
    assertEquals(ULong.MAX_VALUE, result)
  }
}
