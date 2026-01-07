/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigInteger

class TransactionTraceLimitLimitlessTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // set the module limits file
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/strictModuleLimitsLimitless.toml"),
      )
      // enabled the ZkCounter
      .set("--plugin-linea-limitless-enabled=", "true")
      .build()
  }

  @Test
  fun transactionsMinedInSeparateBlocksTest() {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)

    val txData = Bytes.repeat(3.toByte(), 1000).toUnprefixedHexString()

    // send txs that when encoded to RLP are bigger than 1000 byte, so only one should fit in a
    // block, since the
    // block size limit is 2000 byte
    val hashes = ArrayList<String>(5)
    for (i in 0 until 5) {
      val transaction = RawTransaction.createTransaction(
        CHAIN_ID,
        BigInteger.valueOf(i.toLong()),
        GAS_LIMIT,
        accounts.secondaryBenefactor.address,
        VALUE,
        txData,
        GAS_PRICE,
        GAS_PRICE.multiply(BigInteger.TEN),
      )
      val signedTransaction = TransactionEncoder.signMessage(transaction, credentials)
      val response = web3j.ethSendRawTransaction(Numeric.toHexString(signedTransaction)).send()
      hashes.add(response.transactionHash)
    }

    // make sure that there are no more than one transaction per block, because the BLOCK_L1_SIZE
    // limit only allows for one of these transactions.
    assertTransactionsMinedInSeparateBlocks(web3j, hashes)
  }

  companion object {
    private val GAS_LIMIT = DefaultGasProvider.GAS_LIMIT
    private val VALUE = BigInteger.ZERO
    private val GAS_PRICE = BigInteger.TEN.pow(9)
  }
}
