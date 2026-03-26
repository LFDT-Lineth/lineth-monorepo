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
    // Set BEFORE super.setup() so the background scheduler starts in paused state
    // (delay=0 means it fires immediately; if it ran with buildBlocksInBackground=true
    // before the Engine API is ready it would throw, killing all future executions).
    buildBlocksInBackground = false
    super.setup()
  }

  /**
   * if we have a list of transactions [t_0.34, t_0.34, t_0.60, t_0.40], just two blocks are
   * created, where t_x fills X% of the gas limit.
   *
   * Gas estimates come from TransactionProcessingResult.getEstimateGasUsedByTransaction(), which
   * Besu computes as max(actualGasUsed, transactionFloorCost) per EIP-7778 (Block Gas Accounting
   * without Refunds). The floor cost uses the EIP-7623 formula:
   *   floor = 21000 + nonzero_bytes * 4 tokens * 10 gas/token  (= nonzero_bytes * 40 + 21000)
   * Since 40 > 16 (standard calldata cost), the floor always wins for non-zero calldata bytes.
   * Calldata strings are hex-encoded by web3j, so "a".repeat(N) produces N/2 calldata bytes.
   */
  @Test
  fun multipleBlocksFilledRespectingUserBlockGasLimit() {
    val web3j = minerNode.nodeRequests().eth()
    val credentials1 = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager1 = RawTransactionManager(web3j, credentials1, CHAIN_ID)
    val credentials2 = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)
    val txManager2 = RawTransactionManager(web3j, credentials2, CHAIN_ID)

    // 2000 bytes of 0xAA → estimate = 2000 * 40 + 21000 = 101000 ≈ 34% of 300k block gas
    val tx100kGas1 = txManager1.sendTransaction(
      GAS_PRICE,
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.secondaryBenefactor.address,
      "a".repeat(4000),
      VALUE,
    )

    // 2000 bytes of 0xBB → estimate = 2000 * 40 + 21000 = 101000 ≈ 34% of 300k block gas
    val tx100kGas2 = txManager1.sendTransaction(
      GAS_PRICE.multiply(BigInteger.TWO),
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.secondaryBenefactor.address,
      "b".repeat(4000),
      VALUE,
    )

    // 4000 bytes of 0xCC → estimate = 4000 * 40 + 21000 = 181000 ≈ 60% of 300k block gas
    val tx200kGas = txManager2.sendTransaction(
      GAS_PRICE.multiply(BigInteger.TEN),
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.primaryBenefactor.address,
      "c".repeat(8000),
      VALUE,
    )

    // 2500 bytes of 0xDD → estimate = 2500 * 40 + 21000 = 121000 ≈ 40% of 300k block gas
    val tx125kGas = txManager1.sendTransaction(
      GAS_PRICE.multiply(BigInteger.TWO),
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()).divide(BigInteger.TEN),
      accounts.secondaryBenefactor.address,
      "d".repeat(5000),
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
    buildBlocksInBackground = true
  }

  companion object {
    private val GAS_PRICE: BigInteger = BigInteger.TEN.pow(9)
    private val VALUE: BigInteger = BigInteger.TWO
  }
}
