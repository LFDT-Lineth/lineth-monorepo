/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import linea.plugin.acc.test.PrecompileCallEncoder
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import linea.plugin.acc.test.rpc.ForcedTransactionParam
import linea.plugin.acc.test.rpc.GetForcedTransactionInclusionStatusRequest
import linea.plugin.acc.test.rpc.SendForcedRawTransactionRequest
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.Hash.sha3
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicLong

class ForcedTransactionTest : AbstractSendBundleTest() {
  private val forcedTxNumberGenerator = AtomicLong(1)

  private fun nextForcedTxNumber(): Long = forcedTxNumberGenerator.getAndIncrement()

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/moduleLimitsLimitless.toml"),
      )
      .set("--plugin-linea-limitless-enabled=", "true")
      .set("--plugin-linea-deny-list-path=", getResourcePath("/defaultDenyList.txt"))
      .build()
  }

  override fun getCliqueOptions(): GenesisConfigurationFactory.CliqueOptions {
    return GenesisConfigurationFactory.CliqueOptions(
      BLOCK_PERIOD_SECONDS,
      GenesisConfigurationFactory.CliqueOptions.DEFAULT.epochLength(),
      false,
    )
  }

  @Test
  fun singleForcedTransactionIsAcceptedAndMined() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    val rawTx = createSignedTransfer(sender, recipient, 0)
    val txHash = sha3(rawTx)
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)
    assertThat(sendResponse.result[0].forcedTransactionNumber).isEqualTo(forcedTxNumber)
    assertThat(sendResponse.result[0].hash).isEqualTo(txHash)
    assertThat(sendResponse.result[0].error).isNull()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash))

    val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
      .execute(minerNode.nodeRequests())

    assertThat(statusResponse.hasError()).isFalse()
    assertThat(statusResponse.result).isNotNull
    assertThat(statusResponse.result!!.forcedTransactionNumber).isEqualTo(forcedTxNumber)
    assertThat(statusResponse.result!!.inclusionResult).isEqualTo("Included")
    assertThat(statusResponse.result!!.transactionHash).isEqualTo(txHash.lowercase())
  }

  @Test
  fun multipleForcedTransactionsAreAcceptedAndMined() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    val rawTx1 = createSignedTransfer(sender, recipient, 0)
    val rawTx2 = createSignedTransfer(sender, recipient, 1)
    val rawTx3 = createSignedTransfer(sender, recipient, 2)
    val txHash1 = sha3(rawTx1)
    val txHash2 = sha3(rawTx2)
    val txHash3 = sha3(rawTx3)
    val forcedTxNumber1 = nextForcedTxNumber()
    val forcedTxNumber2 = nextForcedTxNumber()
    val forcedTxNumber3 = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(
        ForcedTransactionParam(forcedTxNumber1, rawTx1, DEFAULT_DEADLINE),
        ForcedTransactionParam(forcedTxNumber2, rawTx2, DEFAULT_DEADLINE),
        ForcedTransactionParam(forcedTxNumber3, rawTx3, DEFAULT_DEADLINE),
      ),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(3)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash1))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash2))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash3))

    for (forcedTxNumber in listOf(forcedTxNumber1, forcedTxNumber2, forcedTxNumber3)) {
      val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
        .execute(minerNode.nodeRequests())

      assertThat(statusResponse.result?.inclusionResult).isEqualTo("Included")
    }
  }

  @Test
  fun forcedTransactionWithBadNonceIsRejected() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    val rawTx = createSignedTransfer(sender, recipient, 999)
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)

    await()
      .atMost(30, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("BadNonce")
      }
  }

  @Test
  fun forcedTransactionWithInsufficientBalanceIsRejected() {
    val poorSender = accounts.createAccount("poor")
    val recipient = accounts.primaryBenefactor

    val rawTx = createSignedTransferWithValue(
      poorSender,
      recipient,
      0,
      BigInteger.valueOf(1_000_000_000_000_000_000L), // 1 ETH
    )
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()

    await()
      .atMost(30, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("BadBalance")
      }
  }

  @Test
  fun unknownTransactionReturnsNullStatus() {
    val unknownForcedTxNumber = 999999L

    val statusResponse = GetForcedTransactionInclusionStatusRequest(unknownForcedTxNumber)
      .execute(minerNode.nodeRequests())

    assertThat(statusResponse.hasError()).isFalse()
    assertThat(statusResponse.result).isNull()
  }

  @Test
  fun forcedTransactionsHavePriorityOverRegularTransactions() {
    val forcedSender = accounts.secondaryBenefactor
    val regularSender = accounts.primaryBenefactor
    val recipient = accounts.createAccount("recipient")

    // Send regular transaction first (to tx pool)
    val regularTx = accountTransactions.createTransfer(regularSender, recipient, 1)
    val regularTxHash = regularTx.execute(minerNode.nodeRequests())

    // Send forced transaction after
    val forcedRawTx = createSignedTransfer(forcedSender, recipient, 0)
    val forcedTxHash = sha3(forcedRawTx)
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, forcedRawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(forcedTxHash))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(regularTxHash.toHexString()))

    // Check the block numbers - forced tx should be in an earlier or same block
    val forcedReceipt = ethTransactions.getTransactionReceipt(forcedTxHash)
      .execute(minerNode.nodeRequests())
      .orElseThrow()
    val regularReceipt = ethTransactions.getTransactionReceipt(regularTxHash.toHexString())
      .execute(minerNode.nodeRequests())
      .orElseThrow()

    // If in same block, forced should have lower transaction index
    if (forcedReceipt.blockNumber == regularReceipt.blockNumber) {
      assertThat(forcedReceipt.transactionIndex)
        .isLessThan(regularReceipt.transactionIndex)
    } else {
      assertThat(forcedReceipt.blockNumber)
        .isLessThanOrEqualTo(regularReceipt.blockNumber)
    }
  }

  /**
   * Verifies that when a valid FTX is followed by an invalid one, the invalid one
   * gets its rejection status recorded in a later block (not the same block).
   *
   * This ensures the invariant: only one invalidity proof can be generated per block.
   */
  @Test
  fun failingForcedTransactionNotInSameBlockAsSuccessfulOne() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    // First tx is valid (nonce 0), second tx has bad nonce (nonce 999)
    val validRawTx = createSignedTransfer(sender, recipient, 0)
    val invalidRawTx = createSignedTransfer(sender, recipient, 999) // Bad nonce
    val validTxHash = sha3(validRawTx)
    val validForcedTxNumber = nextForcedTxNumber()
    val invalidForcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(
        ForcedTransactionParam(validForcedTxNumber, validRawTx, DEFAULT_DEADLINE),
        ForcedTransactionParam(invalidForcedTxNumber, invalidRawTx, DEFAULT_DEADLINE),
      ),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(2)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(validTxHash))

    await()
      .atMost(60, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(invalidForcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("BadNonce")
      }

    val validStatus = GetForcedTransactionInclusionStatusRequest(validForcedTxNumber)
      .execute(minerNode.nodeRequests())
    val invalidStatus = GetForcedTransactionInclusionStatusRequest(invalidForcedTxNumber)
      .execute(minerNode.nodeRequests())

    assertThat(validStatus.result).isNotNull
    assertThat(invalidStatus.result).isNotNull
    assertThat(validStatus.result!!.inclusionResult).isEqualTo("Included")

    // The failing tx must be in a different (later) block than the successful one
    val validBlockNumber = java.lang.Long.decode(validStatus.result!!.blockNumber)
    val invalidBlockNumber = java.lang.Long.decode(invalidStatus.result!!.blockNumber)

    assertThat(invalidBlockNumber)
      .withFailMessage(
        "Failing FTX (block %d) should not be in same block as successful FTX (block %d)",
        invalidBlockNumber,
        validBlockNumber,
      )
      .isGreaterThan(validBlockNumber)
  }

  /**
   * Verifies that multiple failing FTXs each get their rejection status in different blocks.
   *
   * This ensures the invariant: only one invalidity proof can be generated per block.
   */
  @Test
  fun multipleFailingForcedTransactionsAreInDifferentBlocks() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    // Create 3 transactions with bad nonces (all will fail because expected nonce is 0)
    val rawTx1 = createSignedTransfer(sender, recipient, 100)
    val rawTx2 = createSignedTransfer(sender, recipient, 200)
    val rawTx3 = createSignedTransfer(sender, recipient, 300)
    val forcedTxNumber1 = nextForcedTxNumber()
    val forcedTxNumber2 = nextForcedTxNumber()
    val forcedTxNumber3 = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(
        ForcedTransactionParam(forcedTxNumber1, rawTx1, DEFAULT_DEADLINE),
        ForcedTransactionParam(forcedTxNumber2, rawTx2, DEFAULT_DEADLINE),
        ForcedTransactionParam(forcedTxNumber3, rawTx3, DEFAULT_DEADLINE),
      ),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(3)

    val forcedTxNumbers = listOf(forcedTxNumber1, forcedTxNumber2, forcedTxNumber3)
    for (forcedTxNumber in forcedTxNumbers) {
      await()
        .atMost(90, TimeUnit.SECONDS)
        .pollInterval(1, TimeUnit.SECONDS)
        .untilAsserted {
          val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
            .execute(minerNode.nodeRequests())

          assertThat(statusResponse.result).isNotNull
          assertThat(statusResponse.result!!.inclusionResult).isEqualTo("BadNonce")
        }
    }

    val blockNumbers = forcedTxNumbers.map { forcedTxNumber ->
      val status = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
        .execute(minerNode.nodeRequests())
      java.lang.Long.decode(status.result!!.blockNumber)
    }

    // Each failing FTX must be in a different block (single invalidity proof per block constraint)
    assertThat(blockNumbers.toSet())
      .withFailMessage(
        "All failing FTXs should be in different blocks, but got blocks: %s",
        blockNumbers,
      )
      .hasSize(3)

    // They should also be in sequential order (first submitted = first processed)
    assertThat(blockNumbers[0])
      .withFailMessage("First tx should be in earliest block")
      .isLessThan(blockNumbers[1])
    assertThat(blockNumbers[1])
      .withFailMessage("Second tx should be in middle block")
      .isLessThan(blockNumbers[2])
  }

  @Test
  fun forcedTransactionCallingBlake2fIsRejected() {
    val excludedPrecompiles = deployExcludedPrecompiles()
    val sender = accounts.secondaryBenefactor

    val callData = PrecompileCallEncoder.encodeBlake2fCall(excludedPrecompiles)
    val rawTx = createSignedContractCall(sender, excludedPrecompiles.contractAddress, callData, 0)
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)

    await()
      .atMost(30, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("BadPrecompile")
      }
  }

  @Test
  fun forcedTransactionToDeniedAddressIsRejected() {
    val sender = accounts.secondaryBenefactor

    val rawTx = createSignedTransferToAddress(sender, DENIED_RECIPIENT_ADDRESS, 0)
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)

    await()
      .atMost(30, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("FilteredAddressTo")
      }
  }

  @Test
  fun forcedTransactionFromDeniedAddressIsRejected() {
    val recipient = accounts.primaryBenefactor

    val rawTx = createSignedTransferFromPrivateKey(
      DENIED_SENDER_PRIVATE_KEY,
      recipient.address,
      0,
    )
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)

    await()
      .atMost(30, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("FilteredAddressFrom")
      }
  }

  @Test
  fun underpricedTransactionIsIncludedWhenForcedButRejectedWhenRegular() {
    val minGasPrice: Wei = Wei.of(1_000_000_000)
    minerNode.miningParameters.minTransactionGasPrice = minGasPrice
    val web3j = minerNode.nodeRequests().eth()
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    // Create an underpriced transaction (gas price 100x lower than normal)
    val underpricedGasPrice = minGasPrice.toBigInteger().divide(BigInteger.valueOf(100))
    val rawTx = createSignedTransferWithCustomGasPrice(sender, recipient, 0, underpricedGasPrice)
    val txHash = sha3(rawTx)

    // First, verify this transaction would be rejected as a regular transaction
    val regularTxResponse = web3j.ethSendRawTransaction(rawTx).send()
    Thread.sleep(blockTimeSeconds!! * 2 * 1000)
    minerNode.verify(eth.expectNoTransactionReceipt(regularTxResponse.transactionHash))

    // Now submit it as a forced transaction - it should be accepted and included
    val forcedTxNumber = nextForcedTxNumber()
    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)
    assertThat(sendResponse.result[0].hash).isEqualTo(txHash)
    assertThat(sendResponse.result[0].error).isNull()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash))

    val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
      .execute(minerNode.nodeRequests())

    assertThat(statusResponse.result).isNotNull
    assertThat(statusResponse.result!!.inclusionResult).isEqualTo("Included")
    assertThat(statusResponse.result!!.transactionHash).isEqualTo(txHash.lowercase())
  }

  @Test
  fun forcedTransactionCallingRipemd160IsRejected() {
    val excludedPrecompiles = deployExcludedPrecompiles()
    val sender = accounts.secondaryBenefactor

    val callData = PrecompileCallEncoder.encodeRipemd160Call(excludedPrecompiles)
    val rawTx = createSignedContractCall(sender, excludedPrecompiles.contractAddress, callData, 0)
    val forcedTxNumber = nextForcedTxNumber()

    val sendResponse = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(forcedTxNumber, rawTx, DEFAULT_DEADLINE)),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(1)

    await()
      .atMost(30, TimeUnit.SECONDS)
      .pollInterval(1, TimeUnit.SECONDS)
      .untilAsserted {
        val statusResponse = GetForcedTransactionInclusionStatusRequest(forcedTxNumber)
          .execute(minerNode.nodeRequests())

        assertThat(statusResponse.result).isNotNull
        assertThat(statusResponse.result!!.inclusionResult).isEqualTo("BadPrecompile")
      }
  }

  private fun createSignedTransfer(
    sender: Account,
    recipient: Account,
    nonce: Int,
  ): String {
    return createSignedTransferWithValue(sender, recipient, nonce, BigInteger.valueOf(1000))
  }

  private fun createSignedTransferWithValue(
    sender: Account,
    recipient: Account,
    nonce: Int,
    value: BigInteger,
  ): String {
    return createSignedTransferToAddressWithValue(sender, recipient.address, nonce, value)
  }

  private fun createSignedTransferToAddress(
    sender: Account,
    recipientAddress: String,
    nonce: Int,
  ): String {
    return createSignedTransferToAddressWithValue(sender, recipientAddress, nonce, BigInteger.valueOf(1000))
  }

  private fun createSignedTransferToAddressWithValue(
    sender: Account,
    recipientAddress: String,
    nonce: Int,
    value: BigInteger,
  ): String {
    val tx = RawTransaction.createTransaction(
      CHAIN_ID,
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

  private fun createSignedContractCall(
    sender: Account,
    contractAddress: String,
    callData: String,
    nonce: Int,
  ): String {
    val tx = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      CONTRACT_CALL_GAS_LIMIT,
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

  private fun createSignedTransferFromPrivateKey(
    senderPrivateKey: String,
    recipientAddress: String,
    nonce: Int,
  ): String {
    val tx = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      TRANSFER_GAS_LIMIT,
      recipientAddress,
      BigInteger.valueOf(1000),
      "",
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return Numeric.toHexString(
      TransactionEncoder.signMessage(tx, Credentials.create(senderPrivateKey)),
    )
  }

  private fun createSignedTransferWithCustomGasPrice(
    sender: Account,
    recipient: Account,
    nonce: Int,
    gasPrice: BigInteger,
  ): String {
    val tx = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      TRANSFER_GAS_LIMIT,
      recipient.address,
      BigInteger.valueOf(1000),
      "",
      gasPrice,
      gasPrice.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return Numeric.toHexString(
      TransactionEncoder.signMessage(tx, sender.web3jCredentialsOrThrow()),
    )
  }

  companion object {
    private const val DEFAULT_DEADLINE = "0xF4240" // 1000000
    private const val DENIED_RECIPIENT_ADDRESS = "0xf17f52151EbEF6C7334FAD080c5704D77216b732"

    // Address 0x44b30d738d2dec1952b92c091724e8aedd52b9b2 - on the deny list
    private const val DENIED_SENDER_PRIVATE_KEY =
      "0xf326e86ba27e2286725a154922094f02573f4921a25a27046b74ec90e653438e"
    private val CONTRACT_CALL_GAS_LIMIT: BigInteger = DefaultGasProvider.GAS_LIMIT.divide(BigInteger.TEN)
  }
}
