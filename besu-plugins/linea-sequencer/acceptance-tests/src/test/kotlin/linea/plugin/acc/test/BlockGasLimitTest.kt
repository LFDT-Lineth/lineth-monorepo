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
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.tx.RawTransactionManager
import java.math.BigInteger

/** This class tests the block gas limit functionality of the plugin. */
class BlockGasLimitTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-max-block-gas=", "300000")
      .build()
  }

  @BeforeEach
  override fun setup() {
    super.setup()
    minerNode.execute(minerTransactions.minerStop())
  }

  /**
   * if we have a list of transactions [t_0.3, t_0.3, t_0.66, t_0.4], just two blocks are created,
   * where t_x fills X% of a limit.
   */
  @Test
  fun multipleBlocksFilledRespectingUserBlockGasLimit() {
    val web3j = minerNode.nodeRequests().eth()
    val credentials1 = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager1 = RawTransactionManager(web3j, credentials1, CHAIN_ID)
    val credentials2 = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)
    val txManager2 = RawTransactionManager(web3j, credentials2, CHAIN_ID)

    val tx100kGas1 = txManager1.sendTransaction(
      GAS_PRICE,
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.secondaryBenefactor.address,
      "a".repeat(10000),
      VALUE,
    )

    val tx100kGas2 = txManager1.sendTransaction(
      GAS_PRICE.multiply(BigInteger.TWO),
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.secondaryBenefactor.address,
      "b".repeat(10000),
      VALUE,
    )

    val tx200kGas = txManager2.sendTransaction(
      GAS_PRICE.multiply(BigInteger.TEN),
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.primaryBenefactor.address,
      "c".repeat(20000),
      VALUE,
    )

    val tx125kGas = txManager1.sendTransaction(
      GAS_PRICE.multiply(BigInteger.TWO),
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.secondaryBenefactor.address,
      "d".repeat(12500),
      VALUE,
    )

    startMining()

    assertTransactionsMinedInSameBlock(
      web3j,
      listOf(tx100kGas1.transactionHash, tx200kGas.transactionHash),
    )
    assertTransactionsMinedInSameBlock(
      web3j,
      listOf(tx100kGas2.transactionHash, tx125kGas.transactionHash),
    )
  }

  private fun startMining() {
    minerNode.execute(minerTransactions.minerStart())
  }

  companion object {
    private val GAS_PRICE: BigInteger = BigInteger.TEN.pow(9)
    private val VALUE: BigInteger = BigInteger.TWO
  }
}
