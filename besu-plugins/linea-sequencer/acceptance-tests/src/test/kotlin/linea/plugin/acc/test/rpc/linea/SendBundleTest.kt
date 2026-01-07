/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import linea.plugin.acc.test.rpc.SendBundleRequest
import linea.plugin.acc.test.tests.web3j.generated.ExcludedPrecompiles
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.Hash.sha3
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.nio.charset.StandardCharsets

class SendBundleTest : AbstractSendBundleTest() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set(
        "--plugin-linea-bundle-overriding-deny-list-path=",
        getResourcePath("/bundleDenyList.txt"),
      )
      // set the module limits file
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/moduleLimitsLimitless.toml"),
      )
      // enabled the ZkCounter
      .set("--plugin-linea-limitless-enabled=", "true")
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
  fun singleTxBundleIsAcceptedAndMined() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    val tx = accountTransactions.createTransfer(sender, recipient, 1)

    val bundleRawTx = tx.signedTransactionData()

    val sendBundleRequest =
      SendBundleRequest(BundleParams(arrayOf(bundleRawTx), Integer.toHexString(1)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx.transactionHash()))
  }

  @Test
  fun bundleIsAcceptedAndMined() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    val tx1 = accountTransactions.createTransfer(sender, recipient, 1)
    val tx2 = accountTransactions.createTransfer(recipient, sender, 1)

    val bundleRawTxs = arrayOf(tx1.signedTransactionData(), tx2.signedTransactionData())

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(1)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx1.transactionHash()))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx2.transactionHash()))
  }

  @Test
  fun bundleWithInvalidTxIsNotAccepted() {
    val mulmodExecutor = deployMulmodExecutor()
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    val tx1 = accountTransactions.createTransfer(sender, recipient, 1)
    val mulmodOk = mulmodOperation(
      mulmodExecutor,
      accounts.primaryBenefactor,
      1,
      1_000,
      BigInteger.valueOf((MAX_TX_GAS_LIMIT + 1).toLong()),
    )

    val bundleRawTxs = arrayOf(tx1.signedTransactionData(), mulmodOk.rawTx)

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(2)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isTrue()
    assertThat(sendBundleResponse.error.message)
      .isEqualTo(
        "Invalid transaction in bundle: hash 0x3f6ff4384305623a7c5cbf05afd9b97c8409be23c4b39c7b16d60001aee4340b," +
          " reason: Gas limit of transaction is greater than the allowed max of 9000000",
      )
  }

  @Test
  fun bundleTxRecipientOnDenyListIsNotAccepted() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount(DENY_TO_ADDRESS)

    val tx1 = accountTransactions.createTransfer(sender, recipient, 1)

    val bundleRawTxs = arrayOf(tx1.signedTransactionData())

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(1)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isTrue()
    assertThat(sendBundleResponse.error.message)
      .isEqualTo(
        "Invalid transaction in bundle: hash " +
          "0xfb47ad29ecf898031bae210263198385f35818d4d154dc752d942a42acabc0cc, " +
          "reason: recipient 0xf17f52151ebef6c7334fad080c5704d77216b732 is blocked as " +
          "appearing on the SDN or other legally prohibited list",
      )
  }

  @Test
  fun bundleTxFromOnDenyListIsNotAccepted() {
    val sender = Account.fromPrivateKey(ethTransactions, "denied", DENY_FROM_PRIVATE_KEY)
    val recipient = accounts.primaryBenefactor

    val tx1 = accountTransactions.createTransfer(sender, recipient, 1)

    val bundleRawTxs = arrayOf(tx1.signedTransactionData())

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(1)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isTrue()
    assertThat(sendBundleResponse.error.message)
      .isEqualTo(
        "Invalid transaction in bundle: hash 0xd631d31a09e865fcd0d86a7f7763747ece057f9f3a63350bb56a206051020a71," +
          " reason: sender 0x44b30d738d2dec1952b92c091724e8aedd52b9b2 " +
          "is blocked as appearing on the SDN or other legally prohibited list",
      )
  }

  @Test
  fun distributeTokensInBundle() {
    val token = deployAcceptanceTestToken()

    val numOfTransfers = 10

    val tokenTransfers = Array(numOfTransfers) { i ->
      transferTokens(
        token,
        accounts.primaryBenefactor,
        i + 1,
        accounts.createAccount("recipient $i"),
        1,
      )
    }

    val bundleRawTxs = tokenTransfers.map { it.rawTx }.toTypedArray()

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(2)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    tokenTransfers.forEach { tokenTransfer ->
      minerNode.verify(eth.expectSuccessfulTransactionReceipt(tokenTransfer.txHash))
      assertThat(token.balanceOf(tokenTransfer.recipient.address).send()).isEqualTo(1)
    }
  }

  @Test
  fun payGasWithTokensInBundle() {
    val token = deployAcceptanceTestToken()

    val recipient = accounts.createAccount("recipient")
    val transferReceipt = token.transfer(recipient.address, BigInteger.TEN).send()
    assertThat(transferReceipt.isStatusOK).isTrue()
    assertThat(token.balanceOf(recipient.address).send()).isEqualTo(10)

    val transferGasTx =
      accountTransactions.createTransfer(accounts.secondaryBenefactor, recipient, 1)
    val payGasWithTokenRawTx =
      transferTokens(token, recipient, 0, accounts.secondaryBenefactor, 1)

    val bundleRawTxs =
      arrayOf(transferGasTx.signedTransactionData(), payGasWithTokenRawTx.rawTx)

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(3)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferGasTx.transactionHash()))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(payGasWithTokenRawTx.txHash))

    val payGasWithTokenReceipt = ethTransactions
      .getTransactionReceipt(payGasWithTokenRawTx.txHash)
      .execute(minerNode.nodeRequests())
      .orElseThrow()

    val gasPrice = Wei.fromHexString(payGasWithTokenReceipt.effectiveGasPrice).toBigInteger()

    val expectedBalance = Amount.ether(1)
      .subtract(Amount.wei(gasPrice.multiply(payGasWithTokenReceipt.gasUsed)))

    minerNode.verify(recipient.balanceEquals(expectedBalance))

    assertThat(token.balanceOf(recipient.address).send()).isEqualTo(9)
    assertThat(token.balanceOf(accounts.secondaryBenefactor.address).send()).isEqualTo(1)
  }

  @Test
  fun singleNotSelectedTxBundleIsNotMined() {
    val excludedPrecompiles = deployExcludedPrecompiles()

    val signedTxInvalid = createInvalidTransaction(excludedPrecompiles)
    val invalidTxHash = Numeric.toHexString(sha3(signedTxInvalid))

    val sendBundleRequest = SendBundleRequest(
      BundleParams(
        arrayOf(Numeric.toHexString(signedTxInvalid)),
        Integer.toHexString(2),
      ),
    )
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    // transfer used as sentry to ensure a new block is mined without the bundles
    val transferTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, accounts.primaryBenefactor, 1)
      .execute(minerNode.nodeRequests())

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))
    minerNode.verify(eth.expectNoTransactionReceipt(invalidTxHash))

    // make sure the bundle is not select for the right reason
    val log = getLog()
    assertThat(log)
      .contains(
        "Failed bundle ${sendBundleResponse.result.bundleHash}, reason TX_MODULE_LINE_COUNT_OVERFLOW",
      )
  }

  @Test
  fun bundleWithFirstNotSelectedTxIsNotMined() {
    val excludedPrecompiles = deployExcludedPrecompiles()

    val recipient = accounts.createAccount("recipient")

    val signedTxInvalid = createInvalidTransaction(excludedPrecompiles)
    val invalidTxHash = Numeric.toHexString(sha3(signedTxInvalid))

    val inBundleTransferTx =
      accountTransactions.createTransfer(recipient, accounts.primaryBenefactor, 1)

    // first is not selected because exceeds line count limit
    val bundleRawTxs = arrayOf(
      Numeric.toHexString(signedTxInvalid),
      inBundleTransferTx.signedTransactionData(),
    )

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(2)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    // transfer used as sentry to ensure a new block is mined without the bundles
    val transferTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, recipient, 10)
      .execute(minerNode.nodeRequests())

    // first sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))
    minerNode.verify(eth.expectNoTransactionReceipt(invalidTxHash))
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()))

    // make sure the bundle is not select for the right reason
    val log = getLog()
    assertThat(log)
      .contains(
        "Failed bundle ${sendBundleResponse.result.bundleHash}, reason TX_MODULE_LINE_COUNT_OVERFLOW",
      )
  }

  @Test
  fun bundleWithLastNotSelectedTxIsNotMined() {
    val excludedPrecompiles = deployExcludedPrecompiles()

    val recipient = accounts.createAccount("recipient")

    val signedTxInvalid = createInvalidTransaction(excludedPrecompiles)
    val invalidTxHash = Numeric.toHexString(sha3(signedTxInvalid))

    val inBundleTransferTx =
      accountTransactions.createTransfer(accounts.secondaryBenefactor, recipient, 10)

    // try with a bundle where first is selected but second no due to the nonce not increased
    val reverseBundleRawTxs = arrayOf(
      inBundleTransferTx.signedTransactionData(),
      Numeric.toHexString(signedTxInvalid),
    )
    val sendReverseBundleRequest =
      SendBundleRequest(BundleParams(reverseBundleRawTxs, Integer.toHexString(2)))
    val sendBundleResponse = sendReverseBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    // transfer used as sentry to ensure a new block is mined without the bundles
    val transferTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, recipient, 1, BigInteger.ZERO)
      .execute(minerNode.nodeRequests())

    // second sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))
    minerNode.verify(eth.expectNoTransactionReceipt(invalidTxHash))
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()))

    // make sure the bundle is not select for the right reason
    val log = getLog()
    assertThat(log)
      .contains(
        "Failed bundle ${sendBundleResponse.result.bundleHash}, reason INVALID(NONCE_TOO_LOW)",
      )
  }

  @Test
  fun mixOfSelectedNotSelectedBundles() {
    val excludedPrecompiles = deployExcludedPrecompiles()
    val mulmodExecutor = deployMulmodExecutor()

    val signedTxInvalid = createInvalidTransaction(excludedPrecompiles)
    val invalidTxHash = Numeric.toHexString(sha3(signedTxInvalid))

    val inBundleTransferTx1 = accountTransactions.createTransfer(
      accounts.secondaryBenefactor,
      accounts.primaryBenefactor,
      1,
      BigInteger.ONE,
    )

    // first is not selected because exceeds line count limit
    val notSelectedBundleRawTxs = arrayOf(
      Numeric.toHexString(signedTxInvalid),
      inBundleTransferTx1.signedTransactionData(),
    )

    val sendNotSelectedBundleRequest =
      SendBundleRequest(BundleParams(notSelectedBundleRawTxs, Integer.toHexString(3)))
    val sendNotSelectedBundleResponse =
      sendNotSelectedBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendNotSelectedBundleResponse.hasError()).isFalse()
    assertThat(sendNotSelectedBundleResponse.result.bundleHash).isNotBlank()

    val mulmodOk = mulmodOperation(mulmodExecutor, accounts.primaryBenefactor, 2, 1_000)
    val inBundleTransferTx2 = accountTransactions.createTransfer(
      accounts.secondaryBenefactor,
      accounts.primaryBenefactor,
      2,
      BigInteger.ZERO,
    )

    // both txs are valid
    val selectedBundleRawTxs =
      arrayOf(mulmodOk.rawTx, inBundleTransferTx2.signedTransactionData())

    val sendSelectedBundleRequest =
      SendBundleRequest(BundleParams(selectedBundleRawTxs, Integer.toHexString(3)))
    val sendSelectedBundleResponse =
      sendSelectedBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendSelectedBundleResponse.hasError()).isFalse()
    assertThat(sendSelectedBundleResponse.result.bundleHash).isNotBlank()

    // assert second bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(mulmodOk.txHash))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(inBundleTransferTx2.transactionHash()))

    // while first bundle is not selected
    minerNode.verify(eth.expectNoTransactionReceipt(invalidTxHash))
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx1.transactionHash()))
    // make sure the first bundle is not select for the right reason
    val log = getLog()
    assertThat(log)
      .contains(
        "Failed bundle ${sendNotSelectedBundleResponse.result.bundleHash}, reason TX_MODULE_LINE_COUNT_OVERFLOW",
      )
  }

  @Test
  fun bundleWithRevertedTxIsNotMined() {
    val revertExample = deployRevertExample()

    // fund a new account
    val recipient = accounts.createAccount("recipient")
    val txHashFundRecipient = accountTransactions
      .createTransfer(accounts.primaryBenefactor, recipient, 10, BigInteger.valueOf(1))
      .execute(minerNode.nodeRequests())
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHashFundRecipient.toHexString()))

    // create a tx that reverts
    val contractAddress = revertExample.contractAddress
    val txData = revertExample.setValue(BigInteger.ZERO).encodeFunctionCall()

    val txThatReverts = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.ZERO,
      TRANSFER_GAS_LIMIT,
      contractAddress,
      BigInteger.ZERO,
      txData,
      GAS_PRICE,
      GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )
    val signedTxThatReverts = Numeric.toHexString(
      TransactionEncoder.signMessage(
        txThatReverts,
        Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY),
      ),
    )
    val txThatRevertsHash = sha3(signedTxThatReverts)

    val inBundleTransferTx =
      accountTransactions.createTransfer(recipient, accounts.secondaryBenefactor, 1)

    // first tx reverts and bundle is not selected
    val bundleRawTxs = arrayOf(signedTxThatReverts, inBundleTransferTx.signedTransactionData())

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(3)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    // transfer used as sentry to ensure a new block is mined without the bundles
    val transferTxHash1 = accountTransactions
      .createTransfer(
        accounts.primaryBenefactor,
        accounts.secondaryBenefactor,
        1,
        BigInteger.valueOf(2),
      )
      .execute(minerNode.nodeRequests())

    // first sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash1.toHexString()))
    minerNode.verify(eth.expectNoTransactionReceipt(txThatRevertsHash))
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()))

    // try with a bundle where first is selected but second reverts
    val reverseBundleRawTxs =
      arrayOf(inBundleTransferTx.signedTransactionData(), signedTxThatReverts)
    val sendReverseBundleRequest =
      SendBundleRequest(BundleParams(reverseBundleRawTxs, Integer.toHexString(4)))
    val sendReverseBundleResponse =
      sendReverseBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendReverseBundleResponse.hasError()).isFalse()
    assertThat(sendReverseBundleResponse.result.bundleHash).isNotBlank()

    // transfer used as sentry to ensure a new block is mined without the bundles
    val transferTxHash2 = accountTransactions
      .createTransfer(accounts.primaryBenefactor, recipient, 1, BigInteger.valueOf(3))
      .execute(minerNode.nodeRequests())

    // second sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash2.toHexString()))
    minerNode.verify(eth.expectNoTransactionReceipt(txThatRevertsHash))
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()))
  }

  companion object {
    private val DENY_TO_ADDRESS =
      Address.fromHexString("0xf17f52151EbEF6C7334FAD080c5704D77216b732")

    // Address 0x44b30d738d2dec1952b92c091724e8aedd52b9b2
    private const val DENY_FROM_PRIVATE_KEY =
      "0xf326e86ba27e2286725a154922094f02573f4921a25a27046b74ec90e653438e"

    private fun createInvalidTransaction(excludedPrecompiles: ExcludedPrecompiles): ByteArray {
      // this tx must not be accepted
      val txInvalid = RawTransaction.createTransaction(
        CHAIN_ID,
        BigInteger.ZERO,
        MULMOD_GAS_LIMIT,
        excludedPrecompiles.contractAddress,
        BigInteger.ZERO,
        excludedPrecompiles
          .callRIPEMD160("I am not allowed here".toByteArray(StandardCharsets.UTF_8))
          .encodeFunctionCall(),
        GAS_PRICE,
        GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
      )

      return TransactionEncoder.signMessage(
        txInvalid,
        Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY),
      )
    }
  }
}
