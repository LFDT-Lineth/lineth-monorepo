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

class TransactionGasLimitTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-max-tx-gas-limit=", MAX_TX_GAS_LIMIT.toString())
      .build()
  }

  @Test
  fun transactionIsMinedWhenGasLimitIsNotExceeded() {
    val simpleStorage = deploySimpleStorage()

    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = simpleStorage.contractAddress
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager: TransactionManager = RawTransactionManager(web3j, credentials, CHAIN_ID)

    val txData = simpleStorage.set("hello").encodeFunctionCall()

    val hashGood = txManager
      .sendTransaction(
        GAS_PRICE,
        BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()),
        contractAddress,
        txData,
        VALUE,
      )
      .transactionHash

    // make sure that a transaction that is not too big was mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(hashGood))
  }

  @Test
  fun transactionIsNotMinedWhenGasLimitIsExceeded() {
    val simpleStorage = deploySimpleStorage()

    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = simpleStorage.contractAddress
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager: TransactionManager = RawTransactionManager(web3j, credentials, CHAIN_ID)

    val txData = simpleStorage.set("hello").encodeFunctionCall()

    val txTooBigResp = txManager.sendTransaction(
      GAS_PRICE,
      BigInteger.valueOf(MAX_TX_GAS_LIMIT + 1L),
      contractAddress,
      txData,
      VALUE,
    )

    assertThat(txTooBigResp.hasError()).isTrue()
    assertThat(txTooBigResp.error.message)
      .isEqualTo("Gas limit of transaction is greater than the allowed max of 9000000")
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

    val lowGasSender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")

    val expectedConfirmedTxs = ArrayList<Hash>(4)

    expectedConfirmedTxs.add(
      minerNode.execute(accountTransactions.createTransfer(lowGasSender, recipient, 1)),
    )

    val txData = simpleStorage.set("too BIG").encodeFunctionCall()

    val txTooBigResp = txManager.sendTransaction(
      GAS_PRICE,
      BigInteger.valueOf(MAX_TX_GAS_LIMIT + 1L),
      contractAddress,
      txData,
      VALUE,
    )

    expectedConfirmedTxs.addAll(
      minerNode.execute(
        accountTransactions.createIncrementalTransfers(lowGasSender, recipient, 3),
      ),
    )

    assertThat(txTooBigResp.hasError()).isTrue()
    assertThat(txTooBigResp.error.message)
      .isEqualTo("Gas limit of transaction is greater than the allowed max of 9000000")

    expectedConfirmedTxs
      .map { it.bytes.toHexString() }
      .forEach { hash -> minerNode.verify(eth.expectSuccessfulTransactionReceipt(hash)) }
  }

  companion object {
    val MAX_TX_GAS_LIMIT = DefaultGasProvider.GAS_LIMIT.toInt()
    private val GAS_PRICE = DefaultGasProvider.GAS_PRICE
    private val VALUE = BigInteger.ZERO
  }
}
