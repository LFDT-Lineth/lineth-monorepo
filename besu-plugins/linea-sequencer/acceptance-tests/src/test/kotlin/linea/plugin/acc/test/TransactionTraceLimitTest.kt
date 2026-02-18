/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigInteger

class TransactionTraceLimitTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-limitless-enabled=", "false")
      .set("--plugin-linea-module-limit-file-path=", getResourcePath("/strictModuleLimits.toml"))
      .build()
  }

  @Test
  fun transactionsMinedInSeparateBlocksTest() {
    val dummyAdder = deployDummyAdder()
    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = dummyAdder.contractAddress
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txData = dummyAdder.add(BigInteger.valueOf(100)).encodeFunctionCall()

    val hashes = ArrayList<String>(5)
    for (i in 0 until 5) {
      val transaction = RawTransaction.createTransaction(
        CHAIN_ID,
        BigInteger.valueOf((i + 1).toLong()),
        GAS_LIMIT,
        contractAddress,
        VALUE,
        txData,
        GAS_PRICE,
        GAS_PRICE.multiply(BigInteger.TEN),
      )
      val signedTransaction = TransactionEncoder.signMessage(transaction, credentials)
      val response = web3j.ethSendRawTransaction(Numeric.toHexString(signedTransaction)).send()
      hashes.add(response.transactionHash)
    }

    // make sure that there are no more than one transaction per block, because the limit for the
    // add module only allows for one of these transactions.
    assertTransactionsMinedInSeparateBlocks(web3j, hashes)
  }

  companion object {
    private val GAS_LIMIT = DefaultGasProvider.GAS_LIMIT
    private val VALUE = BigInteger.ZERO
    private val GAS_PRICE = BigInteger.TEN.pow(9)
  }
}
