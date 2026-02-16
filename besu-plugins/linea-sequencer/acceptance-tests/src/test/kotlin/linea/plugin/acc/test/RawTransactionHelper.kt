/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.utils.Numeric
import java.math.BigInteger

/**
 * Utility object for creating raw signed EIP-1559 transactions.
 * These methods are generic and can be used by any test that needs raw signed transactions.
 */
object RawTransactionHelper {
  val TRANSFER_GAS_LIMIT: BigInteger = BigInteger.valueOf(100_000L)
  val CONTRACT_CALL_GAS_LIMIT: BigInteger = BigInteger.valueOf(300_000) // FTX limit enforced on the contract
  val GAS_PRICE: BigInteger = BigInteger.TEN.pow(9)

  /**
   * Creates a signed transfer transaction between two accounts.
   */
  fun createSignedTransfer(
    chainId: Long,
    sender: Account,
    recipient: Account,
    nonce: Int,
    value: BigInteger = BigInteger.valueOf(1000),
  ): String {
    return createSignedTransferToAddress(chainId, sender, recipient.address, nonce, value)
  }

  /**
   * Creates a signed transfer transaction to a specific address.
   */
  fun createSignedTransferToAddress(
    chainId: Long,
    sender: Account,
    recipientAddress: String,
    nonce: Int,
    value: BigInteger = BigInteger.valueOf(1000),
  ): String {
    val tx = RawTransaction.createTransaction(
      chainId,
      BigInteger.valueOf(nonce.toLong()),
      TRANSFER_GAS_LIMIT,
      recipientAddress,
      value,
      "",
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return Numeric.toHexString(
      TransactionEncoder.signMessage(tx, sender.web3jCredentialsOrThrow()),
    )
  }

  /**
   * Creates a signed contract call transaction.
   */
  fun createSignedContractCall(
    chainId: Long,
    sender: Account,
    contractAddress: String,
    callData: String,
    nonce: Int,
    gasLimit: BigInteger = CONTRACT_CALL_GAS_LIMIT,
  ): String {
    val tx = RawTransaction.createTransaction(
      chainId,
      BigInteger.valueOf(nonce.toLong()),
      gasLimit,
      contractAddress,
      BigInteger.ZERO,
      callData,
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return Numeric.toHexString(
      TransactionEncoder.signMessage(tx, sender.web3jCredentialsOrThrow()),
    )
  }

  /**
   * Creates a signed transfer transaction using a raw private key.
   */
  fun createSignedTransferFromPrivateKey(
    chainId: Long,
    senderPrivateKey: String,
    recipientAddress: String,
    nonce: Int,
    value: BigInteger = BigInteger.valueOf(1000),
  ): String {
    val tx = RawTransaction.createTransaction(
      chainId,
      BigInteger.valueOf(nonce.toLong()),
      TRANSFER_GAS_LIMIT,
      recipientAddress,
      value,
      "",
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return Numeric.toHexString(
      TransactionEncoder.signMessage(tx, Credentials.create(senderPrivateKey)),
    )
  }

  /**
   * Creates a signed transfer transaction with a custom gas price.
   */
  fun createSignedTransferWithCustomGasPrice(
    chainId: Long,
    sender: Account,
    recipient: Account,
    nonce: Int,
    gasPrice: BigInteger,
    value: BigInteger = BigInteger.valueOf(1000),
  ): String {
    val tx = RawTransaction.createTransaction(
      chainId,
      BigInteger.valueOf(nonce.toLong()),
      TRANSFER_GAS_LIMIT,
      recipient.address,
      value,
      "",
      gasPrice,
      gasPrice.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return Numeric.toHexString(
      TransactionEncoder.signMessage(tx, sender.web3jCredentialsOrThrow()),
    )
  }
}
