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
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.tx.RawTransactionManager
import org.web3j.tx.TransactionManager
import org.web3j.tx.gas.DefaultGasProvider
import java.math.BigInteger

class TransactionCallDataSizeLimitTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-max-tx-calldata-size=", MAX_CALLDATA_SIZE.toString())
      .set("--plugin-linea-max-block-calldata-size=", MAX_CALLDATA_SIZE.toString())
      .build()
  }

  @Test
  fun shouldMineTransactions() {
    val simpleStorage = deploySimpleStorage()

    val accounts = listOf(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY, Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)

    val web3j = minerNode.nodeRequests().eth()
    val numCharactersInStringList = listOf(150, 200, 400)

    numCharactersInStringList.forEach { num ->
      sendTransactionsWithGivenLengthPayload(simpleStorage, accounts, web3j, num)
    }
  }

  @Test
  fun transactionIsMinedWhenNotTooBig() {
    val simpleStorage = deploySimpleStorage()
    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = simpleStorage.contractAddress
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager: TransactionManager = RawTransactionManager(web3j, credentials, CHAIN_ID)

    val txDataGood = simpleStorage.set("a".repeat(1200 - 80)).encodeFunctionCall()
    val hashGood = txManager
      .sendTransaction(GAS_PRICE, GAS_LIMIT, contractAddress, txDataGood, VALUE)
      .transactionHash

    // make sure that a transaction that is not too big was mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(hashGood))
  }

  @Test
  fun transactionIsNotMinedWhenTooBig() {
    val simpleStorage = deploySimpleStorage()
    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = simpleStorage.contractAddress
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager: TransactionManager = RawTransactionManager(web3j, credentials, CHAIN_ID)

    val txDataTooBig = simpleStorage.set("a".repeat(1200 - 79)).encodeFunctionCall()
    val txTooBigResp = txManager.sendTransaction(GAS_PRICE, GAS_LIMIT, contractAddress, txDataTooBig, VALUE)

    assertThat(txTooBigResp.hasError()).isTrue()
    assertThat(txTooBigResp.error.message)
      .isEqualTo("Calldata of transaction is greater than the allowed max of 1188")
  }

  /**
   * if we have a list of transactions [t_small, t_tooBig, t_small, ..., t_small] where t_tooBig is
   * too big to fit in a block, we have blocks created that contain all t_small transactions.
   *
   * @throws Exception if send transaction fails
   */
  @Test
  fun multipleSmallTxsMinedWhileTxTooBigNot() {
    val simpleStorage = deploySimpleStorage()

    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = simpleStorage.contractAddress
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager: TransactionManager = RawTransactionManager(web3j, credentials, CHAIN_ID)

    val smallCalldataSender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")

    val expectedConfirmedTxs = ArrayList<Hash>(4)

    expectedConfirmedTxs.add(
      minerNode.execute(accountTransactions.createTransfer(smallCalldataSender, recipient, 1)),
    )

    val txDataTooBig = simpleStorage.set("a".repeat(1200 - 79)).encodeFunctionCall()

    val txTooBigResp = txManager.sendTransaction(
      GAS_PRICE,
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()),
      contractAddress,
      txDataTooBig,
      VALUE,
    )

    expectedConfirmedTxs.addAll(
      minerNode.execute(
        accountTransactions.createIncrementalTransfers(smallCalldataSender, recipient, 3),
      ),
    )

    assertThat(txTooBigResp.hasError()).isTrue()
    assertThat(txTooBigResp.error.message)
      .isEqualTo("Calldata of transaction is greater than the allowed max of 1188")

    expectedConfirmedTxs
      .map { it.bytes.toHexString() }
      .forEach { hash -> minerNode.verify(eth.expectSuccessfulTransactionReceipt(hash)) }
  }

  companion object {
    const val MAX_CALLDATA_SIZE = 1188 // contract has a call data size of 1160
    private val GAS_PRICE = DefaultGasProvider.GAS_PRICE
    private val GAS_LIMIT = DefaultGasProvider.GAS_LIMIT
    private val VALUE = BigInteger.ZERO
  }
}
