/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import maru.core.Seal
import maru.p2p.NoOpP2PNetwork
import maru.p2p.ValidationResult
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.DefaultBlockParameter
import testutils.Checks.getMinedBlocks
import testutils.InjectableSealedBlocksFakeNetwork
import testutils.SingleNodeNetworkStack
import testutils.SpyingP2PNetwork
import testutils.maru.MaruFactory
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class MaruFollowerNegativeTest {
  private val cluster: Cluster =
    Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )

  private val transactionsHelper: BesuTransactionsHelper = BesuTransactionsHelper()
  private val log = LogManager.getLogger(this.javaClass)
  private val maruFactory = MaruFactory()

  @Test
  fun `Maru follower doesn't import blocks without proper signature`() {
    val spyingP2PNetwork = SpyingP2PNetwork(NoOpP2PNetwork)
    val validatorStack =
      SingleNodeNetworkStack(
        cluster = cluster,
      ) { ethereumJsonRpcBaseUrl, engineRpcUrl, tmpDir ->
        maruFactory.buildTestMaruValidatorWithoutP2pPeering(
          ethereumJsonRpcUrl = ethereumJsonRpcBaseUrl,
          engineApiRpc = engineRpcUrl,
          dataDir = tmpDir,
          overridingP2PNetwork = spyingP2PNetwork,
        )
      }
    validatorStack.maruApp.start().get()

    val validatorGenesis =
      validatorStack.besuNode
        .nodeRequests()
        .eth()
        .ethGetBlockByNumber(
          DefaultBlockParameter.valueOf("earliest"),
          false,
        ).send()
        .block

    val followerP2PNetwork = InjectableSealedBlocksFakeNetwork()
    val followerStack =
      SingleNodeNetworkStack(
        cluster = cluster,
        besuBuilder = { BesuFactory.buildTestBesu(validator = false) },
      ) { ethereumJsonRpcBaseUrl, engineRpcUrl, tmpDir ->
        maruFactory.buildTestMaruFollowerWithoutP2pPeering(
          ethereumJsonRpcUrl = ethereumJsonRpcBaseUrl,
          engineApiRpc = engineRpcUrl,
          dataDir = tmpDir,
          p2pNetwork = followerP2PNetwork,
        )
      }
    followerStack.maruApp.start().get()

    val followerGenesis =
      followerStack.besuNode
        .nodeRequests()
        .eth()
        .ethGetBlockByNumber(
          DefaultBlockParameter.valueOf("earliest"),
          false,
        ).send()
        .block
    assertThat(validatorGenesis).isEqualTo(followerGenesis)

    val blocksToProduce = 1
    repeat(blocksToProduce) {
      transactionsHelper.run {
        validatorStack.besuNode.sendTransactionAndAssertExecution(
          logger = log,
          recipient = createAccount("another account"),
          amount = Amount.ether(100),
        )
      }
    }

    await
      .pollDelay(100.milliseconds.toJavaDuration())
      .timeout(5.seconds.toJavaDuration())
      .untilAsserted {
        val blocksProducedByQbftValidator = validatorStack.besuNode.getMinedBlocks(blocksToProduce)
        assertThat(blocksProducedByQbftValidator).hasSize(1)
      }

    val originalSealedBlock = spyingP2PNetwork.emittedBlockMessages.first()
    val originalSeal = originalSealedBlock.commitSeals.toList().first()
    val alternatedSignature = originalSeal.signature.reversed().toByteArray()
    val blockWithWrongSeal = originalSealedBlock.copy(commitSeals = setOf(Seal(alternatedSignature)))
    val result = followerP2PNetwork.injectSealedBlock(blockWithWrongSeal).get()

    assertThat(result).isInstanceOf(ValidationResult.Companion.Invalid::class.java)
  }
}
