/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.web3j.protocol.core.Request
import java.io.FileOutputStream
import java.io.IOException
import java.math.BigInteger
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.exists
import kotlin.io.path.writeText

class TransactionSelectorDenyListReloadTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    tempDenyList = tempDir.resolve("selectorDenyList.txt")
    if (!tempDenyList.exists()) {
      Files.createFile(tempDenyList)
    }
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-deny-list-path=", tempDenyList.toString())
      .build()
  }

  @Test
  fun denyListUpdatesTakeEffectOnTransactionsInThePool() {
    val sender = accounts.createAccount("willBeTemporarilyDenied")
    val recipient = accounts.primaryBenefactor

    // Fund the sender account
    val fundTxHash = accountTransactions
      .createTransfer(accounts.primaryBenefactor, sender, 100)
      .execute(minerNode.nodeRequests())
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(fundTxHash.toHexString()))

    // First verify sender is allowed
    val tx1Hash = accountTransactions
      .createTransfer(sender, recipient, 1)
      .execute(minerNode.nodeRequests())
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx1Hash.toHexString()))

    // to get more control over the test and avoid flacky failures
    buildBlocksInBackground = false

    // Submit tx2 while sender is allowed
    val tx2Hash = accountTransactions
      .createTransfer(sender, recipient, 1)
      .execute(minerNode.nodeRequests())

    addAddressToDenyList(sender.address)
    reloadSelectorPluginOnly()

    buildBlocksInBackground = true

    // tx2 won't be selected - verify with canary
    val canary1TxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, accounts.secondaryBenefactor, 1)
      .execute(minerNode.nodeRequests())
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(canary1TxHash.toHexString()))
    minerNode.verify(eth.expectNoTransactionReceipt(tx2Hash.toHexString()))

    emptyDenyList()
    reloadSelectorPluginOnly()

    // Resend transaction, because it was dropped from the pool. Txhash should be exactly the same as before
    accountTransactions
      .createTransfer(sender, recipient, 1, BigInteger.ONE)
      .execute(minerNode.nodeRequests())

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx2Hash.toHexString()))
  }

  private fun addAddressToDenyList(address: String) {
    tempDenyList.writeText(address)
  }

  private fun emptyDenyList() {
    FileOutputStream(tempDenyList.toFile(), false).use { fileOutputStream ->
      fileOutputStream.write(byteArrayOf())
      fileOutputStream.flush()
    }
  }

  /**
   * Reload ONLY the selector plugin to isolate selector behavior from pool validation.
   */
  private fun reloadSelectorPluginOnly() {
    reloadPlugin("net.consensys.linea.sequencer.txselection.LineaTransactionSelectorPlugin")
  }

  private fun reloadPlugin(pluginClassName: String) {
    val request = ReloadPluginConfigRequest(pluginClassName)
    val result = request.execute(minerNode.nodeRequests())
    assertThat(result).isEqualTo("Success")
  }

  class ReloadPluginConfigRequest(private val pluginClassName: String) : Transaction<String> {
    override fun execute(nodeRequests: NodeRequests): String {
      return try {
        Request<Any, ReloadPluginConfigResponse>(
          "plugins_reloadPluginConfig",
          listOf(pluginClassName),
          nodeRequests.web3jService,
          ReloadPluginConfigResponse::class.java,
        )
          .send()
          .result
      } catch (e: IOException) {
        throw RuntimeException("Failed to reload plugin configuration for $pluginClassName", e)
      }
    }
  }

  class ReloadPluginConfigResponse : org.web3j.protocol.core.Response<String>()

  companion object {
    @JvmStatic
    @TempDir
    lateinit var tempDir: Path

    @JvmStatic
    lateinit var tempDenyList: Path
  }
}
