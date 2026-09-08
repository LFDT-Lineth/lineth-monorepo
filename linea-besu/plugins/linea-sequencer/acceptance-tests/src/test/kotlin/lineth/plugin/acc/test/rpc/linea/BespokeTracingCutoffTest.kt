/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.plugin.acc.test.rpc.linea

import lineth.plugin.acc.test.LineaPluginPoSTestBase
import lineth.plugin.acc.test.TestCommandLineOptionsBuilder
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.nio.charset.StandardCharsets

class BespokeTracingCutoffTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/moduleLimitsLimitless.toml"),
      )
      .set("--plugin-linea-limitless-enabled=", "true")
      .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "true")
      .set("--plugin-linea-tracing-end-timestamp=", "0")
      .build()
  }

  @Test
  fun transactionOverModuleLineCountIsAdmittedAndSelectedAfterCutoff() {
    val excludedPrecompiles = deployExcludedPrecompiles()
    val overLimitTransaction = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.ZERO,
      DefaultGasProvider.GAS_LIMIT.divide(BigInteger.TEN),
      excludedPrecompiles.contractAddress,
      BigInteger.ZERO,
      excludedPrecompiles
        .callRIPEMD160("cutoff control".toByteArray(StandardCharsets.UTF_8))
        .encodeFunctionCall(),
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )
    val signedOverLimitTransaction = TransactionEncoder.signMessage(
      overLimitTransaction,
      Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY),
    )

    val sendTransactionResponse = minerNode.nodeRequests().eth()
      .ethSendRawTransaction(Numeric.toHexString(signedOverLimitTransaction))
      .send()

    assertThat(sendTransactionResponse.hasError()).isFalse()
    minerNode.verify(
      eth.expectSuccessfulTransactionReceipt(sendTransactionResponse.transactionHash),
    )
  }

  @Test
  fun estimateGasOverModuleLineCountSucceedsAfterCutoff() {
    val bls12MapFpToG1 = deployBLS12_MAP_FP_TO_G1()
    val overflowCallParameters = EstimateGasTest.CallParams(
      chainId = null,
      from = accounts.secondaryBenefactor.address,
      nonce = null,
      to = bls12MapFpToG1.contractAddress,
      value = null,
      data = BlsMapFpToG1OverflowSetup.encodeOverflowCall(bls12MapFpToG1),
      gas = "0",
      gasPrice = DefaultGasProvider.GAS_PRICE.toString(),
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val estimateGasResponse = EstimateGasTest.LineaEstimateGasRequest(overflowCallParameters)
      .execute(minerNode.nodeRequests())

    assertThat(estimateGasResponse.hasError()).isFalse()
    assertThat(estimateGasResponse.result.gasLimit).isNotBlank()
  }

  companion object {
    private val GAS_PRICE = BigInteger.TEN.pow(11)
  }
}
