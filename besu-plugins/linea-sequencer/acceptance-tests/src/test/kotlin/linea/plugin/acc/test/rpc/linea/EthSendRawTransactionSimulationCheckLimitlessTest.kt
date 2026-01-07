/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import linea.plugin.acc.test.LineaPluginPoSTestBase
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import linea.plugin.acc.test.tests.web3j.generated.ExcludedPrecompiles
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransactionSet
import org.junit.jupiter.api.Test
import org.web3j.abi.datatypes.generated.Bytes8
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.nio.charset.StandardCharsets

class EthSendRawTransactionSimulationCheckLimitlessTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // set the module limits file
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/moduleLimitsLimitless.toml"),
      )
      // enabled the ZkCounter
      .set("--plugin-linea-limitless-enabled=", "true")
      .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "true")
      .build()
  }

  @Test
  fun validTransactionsAreAccepted() {
    // these are under the line count limit and should be accepted and selected
    val recipient = accounts.createAccount("recipient")
    val expectedConfirmedTxs = mutableListOf<Hash>()

    val transfers =
      (0..3)
        .map { i ->
          accountTransactions.createTransfer(
            accounts.getSecondaryBenefactor(),
            recipient,
            i + 1,
            BigInteger.valueOf(i.toLong()),
          )
        }
        .reversed()
    // reversed, so we are sure no tx is selected before all are sent due to the nonce gap,
    // otherwise a block can be built with some txs before we can check the txpool content

    expectedConfirmedTxs.addAll(minerNode.execute(TransferTransactionSet(transfers)))

    val txPoolContentByHash = getTxPoolContent().map { it["hash"] }
    assertThat(txPoolContentByHash)
      .containsExactlyInAnyOrderElementsOf(
        expectedConfirmedTxs.map { it.toHexString() },
      )

    expectedConfirmedTxs
      .map { it.toHexString() }
      .forEach { hash -> minerNode.verify(eth.expectSuccessfulTransactionReceipt(hash)) }
  }

  @Test
  fun transactionsThatRevertAreAccepted() {
    val revertExample = deployRevertExample()
    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = revertExample.contractAddress
    val txData = revertExample.setValue(BigInteger.ZERO).encodeFunctionCall()

    // this tx reverts but nevertheless it is accepted in the pool
    val txThatReverts =
      RawTransaction.createTransaction(
        CHAIN_ID,
        BigInteger.ZERO,
        GAS_LIMIT.divide(BigInteger.TEN),
        contractAddress,
        VALUE,
        txData,
        GAS_PRICE,
        GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
      )
    val signedTxContractInteraction =
      TransactionEncoder.signMessage(
        txThatReverts,
        Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY),
      )

    val signedTxContractInteractionResp =
      web3j.ethSendRawTransaction(Numeric.toHexString(signedTxContractInteraction)).send()

    assertThat(signedTxContractInteractionResp.hasError()).isFalse()

    val expectedConfirmedTxHash = signedTxContractInteractionResp.transactionHash

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(expectedConfirmedTxHash))
  }

  @Test
  fun transactionsWithExcludedPrecompilesAreNotAccepted() {
    val excludedPrecompiles = deployExcludedPrecompiles()
    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = excludedPrecompiles.contractAddress

    data class InvalidCall(val encodedContractCall: String, val expectedErrorMessage: String)

    val invalidCalls = arrayOf(
      InvalidCall(
        excludedPrecompiles
          .callRIPEMD160("I am not allowed here".toByteArray(StandardCharsets.UTF_8))
          .encodeFunctionCall(),
        "Transaction 0x35451c83b480b45df19105a30f22704df8750b7e328e1ebc646e6442f2f426f9 " +
          "line count for module PRECOMPILE_RIPEMD_BLOCKS=21 is above the limit 0",
      ),
      InvalidCall(
        encodedCallBlake2F(excludedPrecompiles),
        "Transaction 0xfd447b2b688f7448c875f68d9c85ffcb976e1cc722b70dae53e4f2e30d871be8 " +
          "line count for module PRECOMPILE_BLAKE_EFFECTIVE_CALLS=1 is above the limit 0",
      ),
    )

    invalidCalls.forEach { invalidCall ->
      // this tx must not be accepted
      val txInvalid =
        RawTransaction.createTransaction(
          CHAIN_ID,
          BigInteger.ZERO,
          GAS_LIMIT.divide(BigInteger.TEN),
          contractAddress,
          VALUE,
          invalidCall.encodedContractCall,
          GAS_PRICE,
          GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
        )

      val signedTxInvalid =
        TransactionEncoder.signMessage(
          txInvalid,
          Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY),
        )

      val signedTxContractInteractionResp: EthSendTransaction =
        web3j.ethSendRawTransaction(Numeric.toHexString(signedTxInvalid)).send()

      assertThat(signedTxContractInteractionResp.hasError()).isTrue()
      assertThat(signedTxContractInteractionResp.error.message)
        .isEqualTo(invalidCall.expectedErrorMessage)
    }
    assertThat(getTxPoolContent()).isEmpty()
  }

  private fun encodedCallBlake2F(excludedPrecompiles: ExcludedPrecompiles): String {
    return excludedPrecompiles
      .callBlake2f(
        BigInteger.valueOf(12),
        listOf(
          Bytes32.fromHexString(
            "0x48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5",
          ).toArrayUnsafe(),
          Bytes32.fromHexString(
            "0xd182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b",
          ).toArrayUnsafe(),
        ),
        listOf(
          Bytes32.fromHexString(
            "0x6162630000000000000000000000000000000000000000000000000000000000",
          ).toArrayUnsafe(),
          Bytes32.ZERO.toArrayUnsafe(),
          Bytes32.ZERO.toArrayUnsafe(),
          Bytes32.ZERO.toArrayUnsafe(),
        ),
        listOf(Bytes8.DEFAULT.value, Bytes8.DEFAULT.value),
        true,
      )
      .encodeFunctionCall()
  }

  companion object {
    private val GAS_LIMIT: BigInteger = DefaultGasProvider.GAS_LIMIT
    private val VALUE: BigInteger = BigInteger.ZERO
    private val GAS_PRICE: BigInteger = BigInteger.TEN.pow(9)
  }
}
