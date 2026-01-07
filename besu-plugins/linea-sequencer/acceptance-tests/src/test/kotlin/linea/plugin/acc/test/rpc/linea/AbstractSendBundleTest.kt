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
import linea.plugin.acc.test.tests.web3j.generated.AcceptanceTestToken
import linea.plugin.acc.test.tests.web3j.generated.MulmodExecutor
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.web3j.crypto.Hash.sha3
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.utils.Numeric
import java.math.BigInteger

abstract class AbstractSendBundleTest : LineaPluginPoSTestBase() {
  protected fun transferTokens(
    token: AcceptanceTestToken,
    sender: Account,
    nonce: Int,
    recipient: Account,
    amount: Int,
  ): TokenTransfer {
    val transferCalldata =
      token.transfer(recipient.address, BigInteger.valueOf(amount.toLong())).encodeFunctionCall()

    val transferTx = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      TRANSFER_GAS_LIMIT,
      token.contractAddress,
      BigInteger.ZERO,
      transferCalldata,
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    val signedTransferTx = Numeric.toHexString(
      TransactionEncoder.signMessage(transferTx, sender.web3jCredentialsOrThrow()),
    )

    val hashTx = sha3(signedTransferTx)

    return TokenTransfer(recipient, hashTx, signedTransferTx)
  }

  protected fun mulmodOperation(
    executor: MulmodExecutor,
    sender: Account,
    nonce: Int,
    iterations: Int,
  ): MulmodCall = mulmodOperation(executor, sender, nonce, iterations, MULMOD_GAS_LIMIT)

  protected fun mulmodOperation(
    executor: MulmodExecutor,
    sender: Account,
    nonce: Int,
    iterations: Int,
    gasLimit: BigInteger,
  ): MulmodCall {
    val operationCalldata =
      executor.executeMulmod(BigInteger.valueOf(iterations.toLong())).encodeFunctionCall()

    val operationTx = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      gasLimit,
      executor.contractAddress,
      BigInteger.ZERO,
      operationCalldata,
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    val signedTransferTx = Numeric.toHexString(
      TransactionEncoder.signMessage(operationTx, sender.web3jCredentialsOrThrow()),
    )

    val hashTx = sha3(signedTransferTx)

    return MulmodCall(hashTx, signedTransferTx)
  }

  data class BundleParams(val txs: Array<String>, val blockNumber: String) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as BundleParams

      if (!txs.contentEquals(other.txs)) return false
      if (blockNumber != other.blockNumber) return false

      return true
    }

    override fun hashCode(): Int {
      var result = txs.contentHashCode()
      result = 31 * result + blockNumber.hashCode()
      return result
    }
  }

  data class TokenTransfer(val recipient: Account, val txHash: String, val rawTx: String)

  data class MulmodCall(val txHash: String, val rawTx: String)

  companion object {
    @JvmStatic
    protected val TRANSFER_GAS_LIMIT: BigInteger = BigInteger.valueOf(100_000L)

    @JvmStatic
    protected val MULMOD_GAS_LIMIT: BigInteger = BigInteger.valueOf(9_000_000L)

    @JvmStatic
    protected val GAS_PRICE: BigInteger = BigInteger.TEN.pow(9)
  }
}
