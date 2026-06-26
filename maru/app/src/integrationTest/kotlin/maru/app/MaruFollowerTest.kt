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
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.junit.jupiter.api.Test
import testutils.maru.awaitTillMaruHasPeers

/**
 * Follower "import blocks" scenarios. Shared fixture and the longer syncing scenarios live in
 * [MaruFollowerTestBase] and [MaruFollowerSyncingTest] respectively.
 */
class MaruFollowerTest : MaruFollowerTestBase() {
  @Test
  fun `Maru follower is able to import blocks`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce)
  }

  @Test
  fun `Maru follower is able to import blocks with payload validation disabled`() {
    setupMaruHelper(payloadValidationEnabled = false)

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce)
  }

  @Test
  fun `Maru follower is able to import blocks after going down`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    followerStack.maruApp.stop().get()
    followerStack.maruApp.close()
    followerStack.setMaruApp(
      maruFactory.buildTestMaruFollowerWithP2pPeering(
        ethereumJsonRpcUrl = followerStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = followerStack.besuNode.engineRpcUrl().get(),
        dataDir = followerStack.tmpDir,
        validatorPortForStaticPeering = validatorStack.p2pPort,
      ),
    )
    followerStack.maruApp.start().get()

    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after Validator stack goes down`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    val validatorP2pPort = validatorStack.p2pPort
    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    validatorStack.maruApp.stop().get()
    validatorStack.maruApp.close()
    validatorStack.setMaruApp(
      maruFactory.buildTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = validatorStack.besuNode.jsonRpcBaseUrl().get(),
        engineApiRpc = validatorStack.besuNode.engineRpcUrl().get(),
        dataDir = validatorStack.tmpDir,
        p2pPort = validatorP2pPort,
      ),
    )
    validatorStack.maruApp.start().get()

    followerStack.maruApp.awaitTillMaruHasPeers(1u)
    validatorStack.maruApp.awaitTillMaruHasPeers(1u)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }

  @Test
  fun `Maru follower is able to import blocks after its validator el node goes down`() {
    setupMaruHelper()

    val blocksToProduce = 5
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    // This is here mainly to wait until block propagation is complete
    checkValidatorAndFollowerBlocks(blocksToProduce)

    cluster.stop()
    Thread.sleep(3000)
    cluster.startWithRetry(followerStack.besuNode, validatorStack.besuNode)

    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    checkValidatorAndFollowerBlocks(blocksToProduce * 2)
  }
}
