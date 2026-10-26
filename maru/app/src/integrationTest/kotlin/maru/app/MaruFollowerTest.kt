/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.ValidatorFollowerNetwork

class MaruFollowerTest {
  private lateinit var network: ValidatorFollowerNetwork

  @BeforeEach
  fun setUp() {
    network = ValidatorFollowerNetwork()
  }

  @AfterEach
  fun tearDown() {
    network.close()
  }

  @Test
  fun `Maru follower is able to import blocks`() {
    network.start()

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce)
  }

  @Test
  fun `Maru follower is able to import blocks with payload validation disabled`() {
    network.start(payloadValidationEnabled = false)

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce)
  }
}
