/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.p2p.testutils.NetworkUtil.findStablePort
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.ValidatorFollowerNetwork
import testutils.maru.awaitTillMaruHasPeers

class MaruFollowerRecoveryTest {
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
  fun `Maru follower is able to import blocks after going down`() {
    network.start()

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduce)

    network.stopFollowerMaru()
    network.startFollowerMaru()

    network.followerMaru.awaitTillMaruHasPeers(1u)
    network.validatorMaru.awaitTillMaruHasPeers(1u)

    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after Validator stack goes down`() {
    // Use a port below the OS ephemeral range (32768+ Linux, 49152+ macOS) so the kernel
    // never auto-assigns it to another fork's port-0 bind. This makes the stop→restart
    // gap safe
    val validatorP2pPort = findStablePort()
    network.start(validatorP2pPort = validatorP2pPort)

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduce)

    network.restartValidatorMaru()

    network.followerMaru.awaitTillMaruHasPeers(1u)
    network.validatorMaru.awaitTillMaruHasPeers(1u)

    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after its validator el node goes down`() {
    network.start()

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduce)

    network.stopBesuNodes()
    Thread.sleep(3000)
    network.startBesuNodes()

    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }
}
