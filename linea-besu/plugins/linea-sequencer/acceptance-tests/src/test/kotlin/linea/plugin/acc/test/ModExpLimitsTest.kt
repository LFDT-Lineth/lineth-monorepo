/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.utils.Numeric
import java.math.BigInteger

class ModExpLimitsTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // disable line count validation to accept excluded precompile txs in the txpool
      .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "false")
      // set the module limits file
      .set("--plugin-linea-module-limit-file-path=", getResourcePath("/moduleLimits.toml"))
      .build()
  }

  /**
   * Tests the ModExp PRECOMPILE_MODEXP_EFFECTIVE_CALLS limit, that is the number of times the
   * corresponding circuit may be invoked in a single block.
   */
  @Test
  fun modExpLimitTest() {
    val moduleLimits = ModuleLineCountValidator.createLimitModules(getResourcePath("/moduleLimits.toml"))
    val PRECOMPILE_MODEXP_EFFECTIVE_CALLS = moduleLimits["PRECOMPILE_MODEXP_EFFECTIVE_CALLS"]!!

        /*
         * nTransactions: the number of transactions to try to include in the same block. The last
         *     one is not supposed to fit as it exceeds the limit, thus it is included in the next block
         * input: input data for each transaction
         * target: the expected string to be found in the blocks log
         */
    val nTransactions = PRECOMPILE_MODEXP_EFFECTIVE_CALLS + 1
    val input =
      "0000000000000000000000000000000000000000000000000000000000000001" +
        "0000000000000000000000000000000000000000000000000000000000000001" +
        "0000000000000000000000000000000000000000000000000000000000000001" +
        "aabbcc"
    val target =
      "Cumulated line count for module PRECOMPILE_MODEXP_EFFECTIVE_CALLS=" +
        (PRECOMPILE_MODEXP_EFFECTIVE_CALLS + 1) +
        " is above the limit " +
        PRECOMPILE_MODEXP_EFFECTIVE_CALLS +
        ", stopping selection"

    // Deploy the ModExp contract
    val modExp = deployModExp()

    // Create an account to send the transactions
    val modExpSender = accounts.createAccount("modExpSender")

    // Fund the account using secondary benefactor
    val fundTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, modExpSender, 1, BigInteger.ZERO)
      .execute(minerNode.nodeRequests())
      .bytes.toHexString()
    // Verify that the transaction for transferring funds was successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(fundTxHash))

    val txHashes = Array<String?>(nTransactions) { null }
    for (i in 0 until nTransactions) {
      // With decreasing nonce we force the transactions to be included in the same block
      // i     = 0                , 1                , ..., nTransactions - 1
      // nonce = nTransactions - 1, nTransactions - 2, ..., 0
      val nonce = nTransactions - 1 - i

      // Craft the transaction data
      val encodedCallEcRecover = encodedCallModExp(modExp, modExpSender, nonce, Bytes.fromHexString(input))

      // Send the transaction
      val web3j = minerNode.nodeRequests().eth()
      val resp = web3j.ethSendRawTransaction(Numeric.toHexString(encodedCallEcRecover)).send()

      // Store the transaction hash
      txHashes[nonce] = resp.transactionHash
    }

    // Transfer used as sentry to ensure a new block is mined
    val transferTxHash = accountTransactions
      .createTransfer(
        accounts.primaryBenefactor,
        accounts.secondaryBenefactor,
        1,
        BigInteger.ONE, // nonce is 1 as primary benefactor also deploys the contract
      )
      .execute(minerNode.nodeRequests())
    // Wait for the sentry to be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.bytes.toHexString()))

    // Assert that all the transactions involving the EcPairing precompile, but the last one, were
    // included in the same block
    assertTransactionsMinedInSameBlock(
      minerNode.nodeRequests().eth(),
      txHashes.toList().subList(0, nTransactions - 1).filterNotNull(),
    )

    // Assert that the last transaction was included in another block
    assertTransactionsMinedInSeparateBlocks(
      minerNode.nodeRequests().eth(),
      listOf(txHashes[0]!!, txHashes[nTransactions - 1]!!),
    )

    // Assert that the target string is contained in the blocks log
    val blockLog = getAndResetLog()
    assertThat(blockLog).contains(target)
  }
}
