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
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.web3j.crypto.Credentials
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.tx.RawTransactionManager
import org.web3j.utils.Convert
import java.io.FileOutputStream
import java.io.IOException
import java.math.BigInteger
import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.exists
import kotlin.io.path.writeText

class TransactionPoolDenyListReloadTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    tempDenyList = tempDir.resolve("denyList.txt")
    if (!tempDenyList.exists()) {
      Files.createFile(tempDenyList)
    }
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-deny-list-path=", tempDenyList.toString())
      .build()
  }

  @Test
  fun testEmptyDenyList() {
    val allowedCredentials = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)
    val miner = minerNode.nodeRequests().eth()

    val transactionManager = RawTransactionManager(miner, allowedCredentials, CHAIN_ID)
    assertAddressAllowed(transactionManager, allowedCredentials.address)
  }

  @Test
  fun testEmptyDenyList_thenDenySender_cannotAddTxToPool() {
    val willBeDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)
    val miner = minerNode.nodeRequests().eth()
    val transactionManager = RawTransactionManager(miner, willBeDenied, CHAIN_ID)

    assertAddressAllowed(transactionManager, willBeDenied.address)

    addAddressToDenyList(willBeDenied.address)
    reloadPluginConfig()

    assertAddressNotAllowed(transactionManager, willBeDenied.address)
  }

  @Test
  fun testDenySender_cannotAddTxToPool_thenAllowSender() {
    val willBeDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)
    val miner = minerNode.nodeRequests().eth()
    val transactionManager = RawTransactionManager(miner, willBeDenied, CHAIN_ID)

    addAddressToDenyList(willBeDenied.address)
    reloadPluginConfig()
    assertAddressNotAllowed(transactionManager, willBeDenied.address)

    emptyDenyList()
    reloadPluginConfig()
    assertAddressAllowed(transactionManager, willBeDenied.address)
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

  private fun assertAddressAllowed(transactionManager: RawTransactionManager, address: String) {
    val transactionResponse: EthSendTransaction = transactionManager.sendTransaction(
      GAS_PRICE,
      GAS_LIMIT,
      address,
      "",
      VALUE,
    )
    assertThat(transactionResponse.transactionHash).isNotNull()
    assertThat(transactionResponse.error).isNull()
  }

  private fun assertAddressNotAllowed(transactionManager: RawTransactionManager, address: String) {
    val transactionResponse: EthSendTransaction = transactionManager.sendTransaction(
      GAS_PRICE,
      GAS_LIMIT,
      address,
      "",
      VALUE,
    )

    assertThat(transactionResponse.transactionHash).isNull()
    assertThat(transactionResponse.error.message)
      .isEqualTo(
        "sender $address is blocked as appearing on the SDN or other legally prohibited list",
      )
  }

  private fun reloadPluginConfig() {
    val reqLinea = ReloadPluginConfigRequest()
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea).isEqualTo("Success")
  }

  class ReloadPluginConfigRequest : Transaction<String> {
    override fun execute(nodeRequests: NodeRequests): String {
      return try {
        // plugin name is class name
        Request<Any, ReloadPluginConfigResponse>(
          "plugins_reloadPluginConfig",
          listOf("net.consensys.linea.sequencer.txpoolvalidation.LineaTransactionPoolValidatorPlugin"),
          nodeRequests.web3jService,
          ReloadPluginConfigResponse::class.java,
        )
          .send()
          .result
      } catch (e: IOException) {
        throw RuntimeException(e)
      }
    }
  }

  class ReloadPluginConfigResponse : org.web3j.protocol.core.Response<String>()

  companion object {
    private val GAS_PRICE = Convert.toWei("20", Convert.Unit.GWEI).toBigInteger()
    private val GAS_LIMIT = BigInteger.valueOf(210000)
    private val VALUE = BigInteger.ONE // 1 wei

    @JvmStatic
    @TempDir
    lateinit var tempDir: Path

    @JvmStatic
    lateinit var tempDenyList: Path
  }
}
