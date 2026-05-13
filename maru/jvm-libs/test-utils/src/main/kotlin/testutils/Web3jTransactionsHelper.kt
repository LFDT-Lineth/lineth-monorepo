/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.tx.RawTransactionManager
import java.math.BigInteger
import java.util.concurrent.TimeUnit

class Web3jTransactionsHelper(
  private val web3j: Web3j,
) {
  private val transactionManager = createTransactionManager(web3j)

  companion object {
    /**
     * The private key used for creating credentials for test transactions.
     */
    const val TEST_PRIVATE_KEY = "0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae"

    fun createTestCredentials(): Credentials = Credentials.create(TEST_PRIVATE_KEY)

    fun createTransactionManager(web3j: Web3j): RawTransactionManager =
      RawTransactionManager(web3j, createTestCredentials())
  }

  /**
   * Extension function that waits for a transaction to be included in a block.
   */
  fun EthSendTransaction.waitForInclusion() {
    await
      .timeout(30, TimeUnit.SECONDS)
      .untilAsserted {
        val lastTransaction =
          web3j
            .ethGetTransactionByHash(transactionHash)
            .send()
            .transaction
            .get()
        assertThat(lastTransaction.blockNumberRaw)
          .withFailMessage("Transaction $transactionHash wasn't included!")
          .isNotNull()
      }
  }

  /**
   * Sends a single arbitrary transaction.
   * @param web3j The Web3j instance to use for sending the transaction
   * @param transactionManager The transaction manager to use for sending the transaction
   * @return The transaction response
   */
  fun sendArbitraryTransaction(): EthSendTransaction {
    // gas price must be greater than the cost so that block value is greater than that of an empty block
    // during transaction selection otherwise Besu will choose the initial empty block when rebuilding it
    val gasPrice = web3j.ethGasPrice().send().gasPrice + BigInteger.ONE
    val gasLimit = BigInteger.valueOf(21000)
    val to = transactionManager.fromAddress
    return transactionManager.sendTransaction(gasPrice, gasLimit, to, "", BigInteger.ZERO)
  }
}
