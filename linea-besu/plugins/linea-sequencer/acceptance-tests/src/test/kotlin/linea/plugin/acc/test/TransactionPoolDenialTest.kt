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
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.tx.RawTransactionManager
import org.web3j.utils.Convert
import java.math.BigInteger

class TransactionPoolDenialTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-deny-list-path=", getResourcePath("/denyList.txt"))
      .build()
  }

  @Test
  fun senderOnDenyListCannotAddTransactionToPool() {
    val notDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val denied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)
    val miner = minerNode.nodeRequests().eth()

    val transactionManager = RawTransactionManager(miner, denied, CHAIN_ID)
    val transactionResponse: EthSendTransaction = transactionManager.sendTransaction(
      GAS_PRICE,
      GAS_LIMIT,
      notDenied.address,
      "",
      VALUE,
    )

    assertThat(transactionResponse.transactionHash).isNull()
    assertThat(transactionResponse.error.message)
      .isEqualTo(
        "sender 0x627306090abab3a6e1400e9345bc60c78a8bef57 is blocked as appearing on " +
          "the SDN or other legally prohibited list",
      )
  }

  @Test
  fun transactionWithRecipientOnDenyListCannotBeAddedToPool() {
    val notDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val denied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)
    val miner = minerNode.nodeRequests().eth()

    val transactionManager = RawTransactionManager(miner, notDenied, CHAIN_ID)
    val transactionResponse: EthSendTransaction = transactionManager.sendTransaction(
      GAS_PRICE,
      GAS_LIMIT,
      denied.address,
      "",
      VALUE,
    )

    assertThat(transactionResponse.transactionHash).isNull()
    assertThat(transactionResponse.error.message)
      .isEqualTo(
        "recipient 0x627306090abab3a6e1400e9345bc60c78a8bef57 " +
          "is blocked as appearing on the SDN or other legally prohibited list",
      )
  }

  @Test
  fun transactionThatTargetPrecompileIsNotAccepted() {
    (1..9)
      .map { index ->
        accountTransactions.createTransfer(
          accounts.primaryBenefactor,
          accounts.createAccount(Address.precompiled(index)),
          1,
          BigInteger.ONE,
        )
      }
      .forEach { txWithPrecompileRecipient ->
        assertThatThrownBy { txWithPrecompileRecipient.execute(minerNode.nodeRequests()) }
          .hasMessage(
            "Error sending transaction: destination address is a precompile address and cannot receive transactions",
          )
      }
  }

  companion object {
    private val GAS_PRICE = Convert.toWei("20", Convert.Unit.GWEI).toBigInteger()
    private val GAS_LIMIT = BigInteger.valueOf(210000)
    private val VALUE = BigInteger.ONE // 1 wei
  }
}
