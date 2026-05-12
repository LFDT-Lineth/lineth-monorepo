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
import linea.plugin.acc.test.RawTransactionHelper
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicLong

/**
 * Abstract base class for forced transaction tests.
 * Provides FTX-specific helpers while delegating generic transaction signing to RawTransactionHelper.
 */
abstract class AbstractForcedTransactionTest : LineaPluginPoSTestBase() {

  private val forcedTxNumberGenerator = AtomicLong(1)

  /**
   * Generates a unique forced transaction number for each test.
   */
  protected fun nextForcedTxNumber(): Long = forcedTxNumberGenerator.getAndIncrement()

  // Convenience wrappers that pass CHAIN_ID to RawTransactionHelper

  protected fun createSignedTransfer(
    sender: Account,
    recipient: Account,
    nonce: Int,
  ): String = RawTransactionHelper.createSignedTransfer(CHAIN_ID, sender, recipient, nonce)

  protected fun createSignedTransferWithValue(
    sender: Account,
    recipient: Account,
    nonce: Int,
    value: BigInteger,
  ): String = RawTransactionHelper.createSignedTransfer(CHAIN_ID, sender, recipient, nonce, value)

  protected fun createSignedTransferToAddress(
    sender: Account,
    recipientAddress: String,
    nonce: Int,
  ): String = RawTransactionHelper.createSignedTransferToAddress(CHAIN_ID, sender, recipientAddress, nonce)

  protected fun createSignedContractCall(
    sender: Account,
    contractAddress: String,
    callData: String,
    nonce: Int,
  ): String = RawTransactionHelper.createSignedContractCall(CHAIN_ID, sender, contractAddress, callData, nonce)

  protected fun createSignedTransferFromPrivateKey(
    senderPrivateKey: String,
    recipientAddress: String,
    nonce: Int,
  ): String = RawTransactionHelper.createSignedTransferFromPrivateKey(
    CHAIN_ID,
    senderPrivateKey,
    recipientAddress,
    nonce,
  )

  protected fun createSignedTransferWithCustomGasPrice(
    sender: Account,
    recipient: Account,
    nonce: Int,
    gasPrice: BigInteger,
  ): String = RawTransactionHelper.createSignedTransferWithCustomGasPrice(CHAIN_ID, sender, recipient, nonce, gasPrice)

  companion object {
    const val DEFAULT_DEADLINE = "0xF4240" // 1000000
  }
}
