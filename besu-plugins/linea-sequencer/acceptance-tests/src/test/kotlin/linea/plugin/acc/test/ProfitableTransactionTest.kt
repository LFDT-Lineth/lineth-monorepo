/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.plugin.acc.test

import org.bouncycastle.crypto.digests.KeccakDigest
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.tx.RawTransactionManager
import java.math.BigInteger

class ProfitableTransactionTest : LineaPluginPoSTestBase() {

  companion object {
    private const val MIN_MARGIN = 1.5
    private val MIN_GAS_PRICE: Wei = Wei.of(1_000_000_000)
  }

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-min-margin=", MIN_MARGIN.toString())
      .build()
  }

  @BeforeEach
  fun setMinGasPrice() {
    minerNode.miningParameters.minTransactionGasPrice = MIN_GAS_PRICE
  }

  @Test
  fun transactionIsNotMinedWhenUnprofitable() {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID)

    val keccakDigest = KeccakDigest(256)
    val txData = StringBuilder()
    txData.append("0x")
    for (i in 0 until 10) {
      keccakDigest.update(byteArrayOf(i.toByte()), 0, 1)
      val out = ByteArray(32)
      keccakDigest.doFinal(out, 0)
      txData.append(BigInteger(out))
    }

    val txUnprofitable = txManager.sendTransaction(
      MIN_GAS_PRICE.asBigInteger.divide(BigInteger.valueOf(100)),
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong() / 2),
      credentials.address,
      txData.toString(),
      BigInteger.ZERO,
    )

    val sender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")
    val transferTx = accountTransactions.createTransfer(sender, recipient, 1)
    val txHash = minerNode.execute(transferTx)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.bytes.toHexString()))

    // assert that tx below margin is not confirmed
    minerNode.verify(eth.expectNoTransactionReceipt(txUnprofitable.transactionHash))
  }

  /**
   * if we have a list of transactions [t_small, t_tooBig, t_small, ..., t_small] where t_tooBig is
   * too big to fit in a block, we have blocks created that contain all t_small transactions.
   */
  @Test
  fun transactionIsMinedWhenProfitable() {
    minerNode.miningParameters.minTransactionGasPrice = MIN_GAS_PRICE
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")

    val transferTx = accountTransactions.createTransfer(sender, recipient, 1)
    val txHash = minerNode.execute(transferTx)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.bytes.toHexString()))
  }
}
