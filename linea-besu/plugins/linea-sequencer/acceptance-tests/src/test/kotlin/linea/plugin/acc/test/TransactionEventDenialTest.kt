/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.Request
import org.web3j.tx.RawTransactionManager
import org.web3j.tx.TransactionManager
import java.io.IOException
import java.math.BigInteger
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardOpenOption
import kotlin.io.path.exists

class TransactionEventDenialTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    denyEventListPath = tempDir.resolve("denyEventList.txt")
    if (!denyEventListPath.exists()) {
      Files.createFile(denyEventListPath)
    }

    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-events-deny-list-path=", denyEventListPath.toString())
      .set("--plugin-linea-events-bundle-deny-list-path=", denyEventListPath.toString())
      .build()
  }

  /** Test that a transaction emitting a denied topic is rejected. */
  @Test
  fun transactionWithDeniedTopicIsRejected() {
    val logEmitter = deployLogEmitter()
    val web3j = minerNode.nodeRequests().eth()
    val txManager = createTransactionManager(web3j)

    val blockedTopic = "0xaa"
    val payload = "data".toByteArray(StandardCharsets.UTF_8)

    addDenyListFilterAndReload(logEmitter.contractAddress, blockedTopic)

    val txData = logEmitter
      .log1(Bytes32.fromHexString(blockedTopic).toArray(), payload)
      .encodeFunctionCall()
    val transaction = txManager.sendTransaction(
      GAS_PRICE,
      GAS_LIMIT,
      logEmitter.contractAddress,
      txData,
      VALUE,
    )

    // transfer used as canary to ensure a new block is mined without the invalid txs
    val transferTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, accounts.secondaryBenefactor, 1)
      .execute(minerNode.nodeRequests())

    // Canary should be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.bytes.toHexString()))

    // Denied transaction should not be mined
    minerNode.verify(eth.expectNoTransactionReceipt(transaction.transactionHash))

    // Because the transaction is denied based on its emitted event,
    // it should not even be in the pool
    assertTransactionNotInThePool(transaction.transactionHash)

    // Assert that the target string is contained in the block creation log
    val blockLog = getAndResetLog()
    assertThat(blockLog)
      .contains(
        "Transaction ${transaction.transactionHash} is blocked due to contract address " +
          "and event logs appearing on SDN or other legally prohibited list",
      )
  }

  /** Test that a transaction emitting denied topics, with a wildcard, is rejected. */
  @Test
  fun transactionWithDeniedTopicsWithWildcardIsRejected() {
    val logEmitter = deployLogEmitter()
    val web3j = minerNode.nodeRequests().eth()
    val txManager = createTransactionManager(web3j)

    val blockedTopic1 = "0xaa"
    val blockedByWildcardTopic = "0xbb"
    val blockedTopic3 = "0xcc"
    val blockedTopic4 = "0xdd"
    val payload = "data".toByteArray(StandardCharsets.UTF_8)

    addDenyListFilterAndReload(
      logEmitter.contractAddress,
      blockedTopic1,
      null,
      blockedTopic3,
      blockedTopic4,
    )

    val txData = logEmitter
      .log4(
        Bytes32.fromHexString(blockedTopic1).toArray(),
        Bytes32.fromHexString(blockedByWildcardTopic).toArray(),
        Bytes32.fromHexString(blockedTopic3).toArray(),
        Bytes32.fromHexString(blockedTopic4).toArray(),
        payload,
      )
      .encodeFunctionCall()
    val transaction = txManager.sendTransaction(
      GAS_PRICE,
      GAS_LIMIT,
      logEmitter.contractAddress,
      txData,
      VALUE,
    )

    // transfer used as canary to ensure a new block is mined without the invalid txs
    val transferTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, accounts.secondaryBenefactor, 1)
      .execute(minerNode.nodeRequests())

    // Canary should be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.bytes.toHexString()))

    // Denied transaction should not be mined
    minerNode.verify(eth.expectNoTransactionReceipt(transaction.transactionHash))

    // Because the transaction is denied based on its emitted event,
    // it should not even be in the pool
    assertTransactionNotInThePool(transaction.transactionHash)

    // Assert that the target string is contained in the block creation log
    val blockLog = getAndResetLog()
    assertThat(blockLog)
      .contains(
        "Transaction ${transaction.transactionHash} is blocked due to contract address " +
          "and event logs appearing on SDN or other legally prohibited list",
      )
  }

  /** Test that a transaction emitting denied topics, with a wildcard, is rejected. */
  @Test
  fun transactionWithoutDeniedTopicsWithWildcardIsAllowed() {
    val logEmitter = deployLogEmitter()
    val web3j = minerNode.nodeRequests().eth()
    val txManager = createTransactionManager(web3j)

    val blockedTopic1 = "0xaa"
    val blockedTopic2: String? = null
    val blockedTopic3 = "0xcc"
    val blockedTopic4 = "0xdd"
    val allowedTopic1 = "0x11"
    val allowedTopic2 = "0x22"
    val allowedTopic3 = "0x33"
    val allowedTopic4 = "0x44"
    val payload = "data".toByteArray(StandardCharsets.UTF_8)

    addDenyListFilterAndReload(
      logEmitter.contractAddress,
      blockedTopic1,
      blockedTopic2,
      blockedTopic3,
      blockedTopic4,
    )

    val txData = logEmitter
      .log4(
        Bytes32.fromHexString(allowedTopic1).toArray(),
        Bytes32.fromHexString(allowedTopic2).toArray(),
        Bytes32.fromHexString(allowedTopic3).toArray(),
        Bytes32.fromHexString(allowedTopic4).toArray(),
        payload,
      )
      .encodeFunctionCall()
    val transaction = txManager.sendTransaction(
      GAS_PRICE,
      GAS_LIMIT,
      logEmitter.contractAddress,
      txData,
      VALUE,
    )

    // Transaction should be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transaction.transactionHash))
  }

  private fun createTransactionManager(web3j: Web3j): TransactionManager {
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    return RawTransactionManager(web3j, credentials, CHAIN_ID)
  }

  // Helper method to add a filter to the deny list and reload configuration
  private fun addDenyListFilterAndReload(address: String, vararg blockedTopics: String?) {
    // Add filter to deny list
    addFilterToDenyList(address, *blockedTopics)
    reloadPluginConfiguration()
  }

  private fun addFilterToDenyList(address: String, vararg blockedTopics: String?) {
    val entry = String.format(
      "%s,%s,%s,%s,%s",
      address,
      if (blockedTopics.isNotEmpty() && blockedTopics[0] != null) blockedTopics[0] else "",
      if (blockedTopics.size > 1 && blockedTopics[1] != null) blockedTopics[1] else "",
      if (blockedTopics.size > 2 && blockedTopics[2] != null) blockedTopics[2] else "",
      if (blockedTopics.size > 3 && blockedTopics[3] != null) blockedTopics[3] else "",
    )
    Files.writeString(denyEventListPath, "$entry\n", StandardOpenOption.APPEND)
  }

  private fun reloadPluginConfiguration() {
    val request = ReloadPluginConfigRequest()
    val result = request.execute(minerNode.nodeRequests())
    assertThat(result).isEqualTo("Success")
  }

  class ReloadPluginConfigRequest : Transaction<String> {
    override fun execute(nodeRequests: NodeRequests): String {
      return try {
        Request<Any, ReloadPluginConfigResponse>(
          "plugins_reloadPluginConfig",
          listOf("net.consensys.linea.sequencer.txselection.LineaTransactionSelectorPlugin"),
          nodeRequests.web3jService,
          ReloadPluginConfigResponse::class.java,
        )
          .send()
          .result
      } catch (e: IOException) {
        throw RuntimeException("Failed to reload plugin configuration", e)
      }
    }
  }

  class ReloadPluginConfigResponse : org.web3j.protocol.core.Response<String>()

  companion object {
    private val GAS_PRICE = BigInteger.TEN.pow(9)
    private val GAS_LIMIT = BigInteger.valueOf(210_000)
    private val VALUE = BigInteger.ZERO

    @JvmStatic
    @TempDir
    lateinit var tempDir: Path

    @JvmStatic
    lateinit var denyEventListPath: Path
  }
}
