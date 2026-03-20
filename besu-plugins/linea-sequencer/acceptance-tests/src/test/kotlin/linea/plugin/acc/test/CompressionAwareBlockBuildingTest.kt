/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import linea.kotlin.encodeHex
import net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.junit.jupiter.api.Test
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.protocol.core.methods.response.TransactionReceipt
import org.web3j.tx.gas.DefaultGasProvider
import java.math.BigInteger
import java.util.concurrent.TimeUnit
import kotlin.jvm.optionals.getOrNull
import kotlin.random.Random

class CompressionAwareBlockBuildingTest : LineaPluginPoSTestBase() {
  companion object {
    // Set a blob size limit that allows testing compression-aware block building.
    // A transaction with random calldata compresses poorly (roughly 1:1 ratio).
    // Transaction overhead (signature, nonce, gas, etc.) adds ~100-200 bytes.
    //
    // We use smaller limits to ensure our test transactions actually exceed them.
    // The compressor is efficient, so we need tight limits.
    private const val BLOB_SIZE_LIMIT = 4096 // 4 KB
    private const val HEADER_OVERHEAD = 512 // 0.5 KB for block header

    // Effective limit for transactions = BLOB_SIZE_LIMIT - HEADER_OVERHEAD = 3584 bytes
    // A transaction with ~4000 bytes of random calldata should exceed this limit.
    // Two transactions with ~2000 bytes each should together exceed the limit.
    // A small transaction with ~500 bytes should fit easily.

    private val GAS_PRICE: BigInteger = DefaultGasProvider.GAS_PRICE
    private val GAS_LIMIT: BigInteger = DefaultGasProvider.GAS_LIMIT

    // Fixed seed for reproducible random data generation.
    // Random data compresses poorly regardless of seed, but using a fixed seed
    // ensures consistent test behavior across runs.
    private const val RANDOM_SEED = 42L

    // Shared Random instance to ensure different transactions get different calldata.
    // If we created a new Random(RANDOM_SEED) for each transaction, they would all
    // get identical calldata, which compresses very well together (defeating the test).
    private val random = Random(RANDOM_SEED)
  }

  override fun getRequestedPlugins(): List<String> =
    DEFAULT_REQUESTED_PLUGINS + "RecordingTransactionSelectorPlugin"

  override fun getAdditionalRpcApis(): Set<String> = setOf("TEST")

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-blob-size-limit=", BLOB_SIZE_LIMIT.toString())
      .set("--plugin-linea-compressed-block-header-overhead=", HEADER_OVERHEAD.toString())
      .set("--plugin-linea-module-limit-file-path=", getResourcePath("/noModuleLimits.toml"))
      .build()
  }

  /**
   * Test that a transaction with calldata that compresses to more than the blob size limit
   * is not included in any block, while a slightly smaller transaction from a different sender
   * is accepted in a subsequent block.
   *
   * Strategy:
   * 1. Disable background block building
   * 2. Submit a large transaction that exceeds the compressed size limit (~4000 bytes calldata)
   * 3. Submit a small transaction that fits easily (~500 bytes calldata)
   * 4. Build a block — verify the small transaction is included, the large one is not
   * 5. Submit a medium transaction slightly below the threshold (~3400 bytes calldata)
   * 6. Build another block — verify the medium transaction is now included
   */
  @Test
  fun largeTransactionExceedingCompressedLimitIsNotIncluded() {
    val newAccounts = createAccounts(numAccounts = 3, initialBalanceEther = 10)

    // Now disable background block building for the actual test
    buildBlocksInBackground = false
    val largeTxSender = newAccounts[0]
    val smallTxSender = newAccounts[1]

    val largeTxRaw = createRawTransactionWithRandomCalldata(largeTxSender, 0, 4000)
    val smallTxRaw = createRawTransactionWithRandomCalldata(smallTxSender, 0, 500)
    val web3j = minerNode.nodeRequests().eth()
    val largeTxResponse = web3j.ethSendRawTransaction(largeTxRaw).send()
    val smallTxResponse = web3j.ethSendRawTransaction(smallTxRaw).send()

    // Both should be accepted into the pool (no error at submission time)
    assertThat(largeTxResponse.hasError())
      .withFailMessage { "Large tx submission failed: ${largeTxResponse.error?.message}" }
      .isFalse()
    assertThat(smallTxResponse.hasError())
      .withFailMessage { "Small tx submission failed: ${smallTxResponse.error?.message}" }
      .isFalse()

    val largeTxHash = largeTxResponse.transactionHash
    val smallTxHash = smallTxResponse.transactionHash

    buildNewBlockAndWait()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(smallTxHash))
    minerNode.verify(eth.expectNoTransactionReceipt(largeTxHash))
    await()
      .atMost(4, TimeUnit.SECONDS)
      .pollInterval(50, TimeUnit.MILLISECONDS)
      .untilAsserted {
        assertThat(getRejectionReason(largeTxHash))
          .withFailMessage { "Expected large tx to be rejected with BLOCK_COMPRESSED_SIZE_OVERFLOW" }
          .isEqualTo(LineaTransactionSelectionResult.BLOCK_COMPRESSED_SIZE_OVERFLOW.toString())
      }
    assertThat(getRejectionReason(smallTxHash))
      .withFailMessage { "Expected small tx to not have a rejection reason (it was selected)" }
      .isNull()
  }

  /**
   * Test that two transactions that individually fit but together exceed the compressed size limit
   * are spaced out in separate blocks.
   *
   * Strategy:
   * 1. Disable background block building
   * 2. Submit two medium-sized transactions that each fit individually but together exceed the limit
   * 3. Build a block - only one should be included
   * 4. Build another block - the second should be included
   */
  @Test
  fun twoTransactionsExceedingLimitAreSpacedInSeparateBlocks() {
    val newAccounts = createAccounts(numAccounts = 2, initialBalanceEther = 10)

    buildBlocksInBackground = false
    val sender1 = newAccounts[0]
    val sender2 = newAccounts[1]

    val tx1Raw = createRawTransactionWithRandomCalldata(sender1, 0, 2000)
    val tx2Raw = createRawTransactionWithRandomCalldata(sender2, 0, 2000)

    val web3j = minerNode.nodeRequests().eth()
    val tx1Response = web3j.ethSendRawTransaction(tx1Raw).send()
    val tx2Response = web3j.ethSendRawTransaction(tx2Raw).send()

    assertThat(tx1Response.hasError())
      .withFailMessage { "Tx1 submission failed: ${tx1Response.error?.message}" }
      .isFalse()
    assertThat(tx2Response.hasError())
      .withFailMessage { "Tx2 submission failed: ${tx2Response.error?.message}" }
      .isFalse()

    val tx1Hash = tx1Response.transactionHash
    val tx2Hash = tx2Response.transactionHash

    buildNewBlockAndWait()

    val tx1ReceiptAfterBlock1 = getTransactionReceiptIfExists(tx1Hash)
    val tx2ReceiptAfterBlock1 = getTransactionReceiptIfExists(tx2Hash)

    val tx1InBlock1 = tx1ReceiptAfterBlock1 != null
    val tx2InBlock1 = tx2ReceiptAfterBlock1 != null

    assertThat(tx1InBlock1 xor tx2InBlock1)
      .withFailMessage {
        "Expected exactly one transaction in first block, but tx1InBlock1=$tx1InBlock1, tx2InBlock1=$tx2InBlock1"
      }
      .isTrue()

    buildNewBlockAndWait()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx1Hash))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx2Hash))

    val tx1Receipt = ethTransactions.getTransactionReceipt(tx1Hash).execute(minerNode.nodeRequests())
    val tx2Receipt = ethTransactions.getTransactionReceipt(tx2Hash).execute(minerNode.nodeRequests())

    assertThat(tx1Receipt).isPresent
    assertThat(tx2Receipt).isPresent

    val tx1BlockNumber = tx1Receipt.get().blockNumber
    val tx2BlockNumber = tx2Receipt.get().blockNumber

    assertThat(tx1BlockNumber)
      .withFailMessage {
        "Expected transactions in different blocks, but both in block $tx1BlockNumber"
      }
      .isNotEqualTo(tx2BlockNumber)
  }

  /**
   * Creates a signed raw transaction with random calldata of the specified size.
   * Random data compresses poorly, making it ideal for testing compression limits.
   */
  private fun createRawTransactionWithRandomCalldata(
    sender: Account,
    nonce: Int,
    calldataSize: Int,
  ): String {
    val randomCalldata = ByteArray(calldataSize)
    random.nextBytes(randomCalldata)

    val rawTx = RawTransaction.createTransaction(
      CHAIN_ID,
      nonce.toBigInteger(),
      GAS_LIMIT,
      "0x" + "00".repeat(20),
      BigInteger.ZERO,
      randomCalldata.encodeHex(),
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return TransactionEncoder.signMessage(rawTx, sender.web3jCredentialsOrThrow()).encodeHex()
  }

  private fun getRejectionReason(txHash: String): String? {
    val response = org.web3j.protocol.core.Request(
      "test_getRejectionReason",
      listOf(txHash),
      minerNode.nodeRequests().web3jService,
      TestRejectionResponse::class.java,
    ).send()
    return response.result
  }

  class TestRejectionResponse : org.web3j.protocol.core.Response<String>()

  private fun getTransactionReceiptIfExists(txHash: String): TransactionReceipt? {
    return ethTransactions.getTransactionReceipt(txHash).execute(minerNode.nodeRequests()).getOrNull()
  }
}
