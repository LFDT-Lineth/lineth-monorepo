/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import linea.plugin.acc.test.tests.web3j.generated.ExcludedPrecompiles
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Test
import org.web3j.abi.datatypes.generated.Bytes8
import org.web3j.crypto.Credentials
import org.web3j.crypto.Hash
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.nio.charset.StandardCharsets

class ExcludedPrecompilesTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // disable line count validation to accept excluded precompile txs in the txpool
      .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "false")
      .set("--plugin-linea-limitless-enabled=", "false")
      .set("--plugin-linea-module-limit-file-path=", getResourcePath("/noModuleLimits.toml"))
      .build()
  }

  @Test
  fun transactionsWithExcludedPrecompilesAreNotAccepted() {
    val excludedPrecompiles = deployExcludedPrecompiles()
    val web3j = minerNode.nodeRequests().eth()
    val contractAddress = excludedPrecompiles.contractAddress

    // fund a new account
    val recipient = accounts.createAccount("recipient")
    val txHashFundRecipient = accountTransactions
      .createTransfer(accounts.primaryBenefactor, recipient, 10, BigInteger.valueOf(1))
      .execute(minerNode.nodeRequests())
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHashFundRecipient.toHexString()))

    data class InvalidCall(
      val senderPrivateKey: String,
      val nonce: Int,
      val encodedContractCall: String,
      val expectedTraceLog: String,
    )

    val invalidCalls = arrayOf(
      InvalidCall(
        Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY,
        2,
        excludedPrecompiles
          .callRIPEMD160("I am not allowed here".toByteArray(StandardCharsets.UTF_8))
          .encodeFunctionCall(),
        "Tx 0xe4648fd59d4289e59b112bf60931336440d306c85c2aac5a8b0c64ab35bc55b7 " +
          "line count for module PRECOMPILE_RIPEMD_BLOCKS=1 is above the limit 0",
      ),
      InvalidCall(
        Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY,
        0,
        encodedCallBlake2F(excludedPrecompiles),
        "Tx 0x9f457b1b5244b03c54234f7f9e8225d4253135dd3c99a46dc527d115e7ea5dac " +
          "line count for module PRECOMPILE_BLAKE_ROUNDS=12 is above the limit 0",
      ),
    )

    val invalidTxHashes = invalidCalls.map { invalidCall ->
      // this tx must not be accepted but not mined
      val txInvalid = RawTransaction.createTransaction(
        CHAIN_ID,
        BigInteger.valueOf(invalidCall.nonce.toLong()),
        GAS_LIMIT.divide(BigInteger.TEN),
        contractAddress,
        BigInteger.ZERO,
        invalidCall.encodedContractCall,
        GAS_PRICE,
        GAS_PRICE,
      )

      val signedTxInvalid = TransactionEncoder.signMessage(
        txInvalid,
        Credentials.create(invalidCall.senderPrivateKey),
      )

      val signedTxInvalidResp = web3j.ethSendRawTransaction(Numeric.toHexString(signedTxInvalid)).send()

      assertThat(signedTxInvalidResp.hasError()).isFalse()
      signedTxInvalidResp.transactionHash
    }

    assertThat(getTxPoolContent()).hasSize(invalidTxHashes.size)

    // transfer used as sentry to ensure a new block is mined without the invalid txs
    val transferTxHash1 = accountTransactions
      .createTransfer(recipient, accounts.secondaryBenefactor, 1)
      .execute(minerNode.nodeRequests())

    // first sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash1.toHexString()))
    invalidCalls.forEach { invalidCall ->
      minerNode.verify(
        eth.expectNoTransactionReceipt(Hash.sha3(invalidCall.encodedContractCall)),
      )
    }

    val log = getLog()
    // verify trace log contains the exclusion cause
    invalidCalls.forEach { invalidCall ->
      assertThat(log).contains(invalidCall.expectedTraceLog)
    }
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
    private val GAS_PRICE: BigInteger = DefaultGasProvider.GAS_PRICE
  }
}
