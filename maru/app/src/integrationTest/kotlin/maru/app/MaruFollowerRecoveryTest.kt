/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.startWithRetry
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
    network.startBesuNodes()
  }

  @AfterEach
  fun tearDown() {
    network.close()
  }

  @Test
  fun `Maru follower is able to import blocks after going down`() {
    network.startMaruNodes()

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduce)

    network.stopFollowerMaru()
    network.startFollowerMaru()

    network.followerStack.maruApp.awaitTillMaruHasPeers(1u)
    network.validatorStack.maruApp.awaitTillMaruHasPeers(1u)

    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after Validator stack goes down`() {
    // Use a port below the OS ephemeral range (32768+ Linux, 49152+ macOS) so the kernel
    // never auto-assigns it to another fork's port-0 bind. This makes the stop→restart
    // gap safe
    val validatorP2pPort = findStablePort()
    network.startMaruNodes(validatorP2pPort = validatorP2pPort)

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduce)

    val validatorStack = network.validatorStack
    validatorStack.maruApp.stop().get()
    validatorStack.maruApp.close()

    validatorStack.setMaruApp(
      network.maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        p2pPort = validatorP2pPort,
      ),
    )
    validatorStack.maruApp.start().get()

    network.followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)

    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after its validator el node goes down`() {
    network.startMaruNodes()

    val blocksToProduce = 5
    network.produceBlocks(blocksToProduce)

    // This is here mainly to wait until block propagation is complete
    network.checkValidatorAndFollowerBlocks(blocksToProduce)

    network.cluster.stop()
    Thread.sleep(3000)
    network.cluster.startWithRetry(network.followerStack.besuNode, network.validatorStack.besuNode)

    network.produceBlocks(blocksToProduce)

    network.checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }
}
