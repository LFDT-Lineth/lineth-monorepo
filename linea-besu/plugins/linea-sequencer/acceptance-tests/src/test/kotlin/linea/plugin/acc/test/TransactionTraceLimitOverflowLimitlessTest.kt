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
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigInteger

class TransactionTraceLimitOverflowLimitlessTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // set the module limits file
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/strictModuleLimitsLimitless.toml"),
      )
      // enabled the ZkCounter
      .set("--plugin-linea-limitless-enabled=", "true")
      .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "false")
      .build()
  }

  @Test
  fun transactionOverModuleLineCountRemoved() {
    val web3j = minerNode.nodeRequests().eth()
    val txData = Bytes.repeat(3.toByte(), 2000).toUnprefixedHexString()

    // this tx will not be selected since it goes above the max block size,
    // but selection should go on and select the next one
    val txModuleLineCountTooBig = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.ZERO,
      GAS_LIMIT.divide(BigInteger.TEN),
      accounts.primaryBenefactor.address,
      VALUE,
      txData,
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )
    val signedTxContractInteraction = TransactionEncoder.signMessage(
      txModuleLineCountTooBig,
      Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY),
    )
    val signedTxContractInteractionResp = web3j.ethSendRawTransaction(
      Numeric.toHexString(signedTxContractInteraction),
    ).send()

    // these are under the block size limit and should be selected
    val fewLinesSender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")
    val expectedConfirmedTxs = ArrayList<Hash>(4)

    expectedConfirmedTxs.addAll(
      minerNode.execute(
        accountTransactions.createIncrementalTransfers(fewLinesSender, recipient, 4),
      ),
    )

    expectedConfirmedTxs
      .map { it.bytes.toHexString() }
      .forEach { hash -> minerNode.verify(eth.expectSuccessfulTransactionReceipt(hash)) }

    // assert that tx over line count limit is not confirmed and is removed from the pool
    minerNode.verify(
      eth.expectNoTransactionReceipt(signedTxContractInteractionResp.transactionHash),
    )
    assertTransactionNotInThePool(signedTxContractInteractionResp.transactionHash)
  }

  companion object {
    private val GAS_LIMIT = DefaultGasProvider.GAS_LIMIT
    private val VALUE = BigInteger.ZERO
    private val GAS_PRICE = BigInteger.TEN.pow(11)
  }
}
