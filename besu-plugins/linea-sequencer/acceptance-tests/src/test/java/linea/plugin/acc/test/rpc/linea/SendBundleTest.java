/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea;

import static org.assertj.core.api.Assertions.assertThat;
import static org.web3j.crypto.Hash.sha3;

import java.math.BigInteger;
import java.util.Arrays;

import linea.plugin.acc.test.tests.web3j.generated.AcceptanceTestToken;
import linea.plugin.acc.test.tests.web3j.generated.RevertExample;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.utils.Numeric;

public class SendBundleTest extends AbstractSendBundleTest {

  @Test
  public void singleTxBundleIsAcceptedAndMined() {
    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.getPrimaryBenefactor();

    final TransferTransaction tx = accountTransactions.createTransfer(sender, recipient, 1);

    final String bundleRawTx = tx.signedTransactionData();

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(new String[] {bundleRawTx}, Integer.toHexString(1)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx.transactionHash()));
  }

  @Test
  public void bundleIsAcceptedAndMined() {
    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.getPrimaryBenefactor();

    final TransferTransaction tx1 = accountTransactions.createTransfer(sender, recipient, 1);
    final TransferTransaction tx2 = accountTransactions.createTransfer(recipient, sender, 1);

    final String[] bundleRawTxs =
        new String[] {tx1.signedTransactionData(), tx2.signedTransactionData()};

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(1)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx1.transactionHash()));
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx2.transactionHash()));
  }

  @Test
  public void distributeTokensInBundle() throws Exception {
    final AcceptanceTestToken token = deployAcceptanceTestToken();

    final int numOfTransfers = 10;

    final TokenTransfer[] tokenTransfers = new TokenTransfer[numOfTransfers];
    for (int i = 0; i < numOfTransfers; i++) {
      tokenTransfers[i] =
          transferTokens(
              token,
              accounts.getPrimaryBenefactor(),
              i + 1,
              accounts.createAccount("recipient " + i),
              1);
    }

    final var bundleRawTxs =
        Arrays.stream(tokenTransfers).map(TokenTransfer::rawTx).toArray(String[]::new);

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(2)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    Arrays.stream(tokenTransfers)
        .forEach(
            tokenTransfer -> {
              minerNode.verify(eth.expectSuccessfulTransactionReceipt(tokenTransfer.txHash()));
              try {
                assertThat(token.balanceOf(tokenTransfer.recipient().getAddress()).send())
                    .isEqualTo(1);
              } catch (Exception e) {
                throw new RuntimeException(e);
              }
            });
  }

  @Test
  public void payGasWithTokensInBundle() throws Exception {
    final AcceptanceTestToken token = deployAcceptanceTestToken();

    final var recipient = accounts.createAccount("recipient");
    final var transferReceipt = token.transfer(recipient.getAddress(), BigInteger.TEN).send();
    assertThat(transferReceipt.isStatusOK()).isTrue();
    assertThat(token.balanceOf(recipient.getAddress()).send()).isEqualTo(10);

    final var transferGasTx =
        accountTransactions.createTransfer(accounts.getSecondaryBenefactor(), recipient, 1);
    final var payGasWithTokenRawTx =
        transferTokens(token, recipient, 0, accounts.getSecondaryBenefactor(), 1);

    final var bundleRawTxs =
        new String[] {transferGasTx.signedTransactionData(), payGasWithTokenRawTx.rawTx()};

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(3)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferGasTx.transactionHash()));
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(payGasWithTokenRawTx.txHash()));

    final var payGasWithTokenReceipt =
        ethTransactions
            .getTransactionReceipt(payGasWithTokenRawTx.txHash())
            .execute(minerNode.nodeRequests())
            .orElseThrow();
    final var gasPrice =
        Wei.fromHexString(payGasWithTokenReceipt.getEffectiveGasPrice()).toBigInteger();

    final var expectedBalance =
        Amount.ether(1)
            .subtract(Amount.wei(gasPrice.multiply(payGasWithTokenReceipt.getGasUsed())));

    minerNode.verify(recipient.balanceEquals(expectedBalance));

    assertThat(token.balanceOf(recipient.getAddress()).send()).isEqualTo(9);
    assertThat(token.balanceOf(accounts.getSecondaryBenefactor().getAddress()).send()).isEqualTo(1);
  }

  @Test
  public void singleNotSelectedTxBundleIsNotMined() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();

    final var mulmodOverflow =
        mulmodOperation(mulmodExecutor, accounts.getPrimaryBenefactor(), 1, 5_000);

    final var sendBundleRequest =
        new SendBundleRequest(
            new BundleParams(new String[] {mulmodOverflow.rawTx()}, Integer.toHexString(2)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    // transfer used as sentry to ensure a new block is mined without the bundles
    final var transferTxHash =
        accountTransactions
            .createTransfer(accounts.getSecondaryBenefactor(), accounts.getPrimaryBenefactor(), 1)
            .execute(minerNode.nodeRequests());

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()));
    minerNode.verify(eth.expectNoTransactionReceipt(mulmodOverflow.txHash()));
  }

  @Test
  public void bundleWithNotSelectedTxIsNotMined() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();
    final var recipient = accounts.createAccount("recipient");

    final var mulmodOverflow =
        mulmodOperation(mulmodExecutor, accounts.getPrimaryBenefactor(), 1, 5_000);
    final var inBundleTransferTx =
        accountTransactions.createTransfer(recipient, accounts.getPrimaryBenefactor(), 1);

    // first is not selected because exceeds line count limit
    final var bundleRawTxs =
        new String[] {mulmodOverflow.rawTx(), inBundleTransferTx.signedTransactionData()};

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(2)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    // transfer used as sentry to ensure a new block is mined without the bundles
    final var transferTxHash1 =
        accountTransactions
            .createTransfer(accounts.getSecondaryBenefactor(), recipient, 10)
            .execute(minerNode.nodeRequests());

    // first sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash1.toHexString()));
    minerNode.verify(eth.expectNoTransactionReceipt(mulmodOverflow.txHash()));
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()));

    // try with a bundle where first is selected but second no
    final var reverseBundleRawTxs =
        new String[] {inBundleTransferTx.signedTransactionData(), mulmodOverflow.rawTx()};
    final var sendReverseBundleRequest =
        new SendBundleRequest(new BundleParams(reverseBundleRawTxs, Integer.toHexString(3)));
    final var sendReverseBundleResponse =
        sendReverseBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendReverseBundleResponse.hasError()).isFalse();
    assertThat(sendReverseBundleResponse.getResult().bundleHash()).isNotBlank();

    // transfer used as sentry to ensure a new block is mined without the bundles
    final var transferTxHash2 =
        accountTransactions
            .createTransfer(
                accounts.getSecondaryBenefactor(),
                accounts.getPrimaryBenefactor(),
                1,
                BigInteger.valueOf(1))
            .execute(minerNode.nodeRequests());

    // second sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash2.toHexString()));
    minerNode.verify(eth.expectNoTransactionReceipt(mulmodOverflow.txHash()));
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()));
  }

  @Test
  public void mixOfSelectedNotSelectedBundles() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();

    final var mulmodOverflow =
        mulmodOperation(mulmodExecutor, accounts.getPrimaryBenefactor(), 1, 5_000);
    final var inBundleTransferTx1 =
        accountTransactions.createTransfer(
            accounts.getSecondaryBenefactor(), accounts.getPrimaryBenefactor(), 1, BigInteger.ZERO);

    // first is not selected because exceeds line count limit
    final var notSelectedBundleRawTxs =
        new String[] {mulmodOverflow.rawTx(), inBundleTransferTx1.signedTransactionData()};

    final var sendNotSelectedBundleRequest =
        new SendBundleRequest(new BundleParams(notSelectedBundleRawTxs, Integer.toHexString(2)));
    final var sendNotSelectedBundleResponse =
        sendNotSelectedBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendNotSelectedBundleResponse.hasError()).isFalse();
    assertThat(sendNotSelectedBundleResponse.getResult().bundleHash()).isNotBlank();

    final var mulmodOk = mulmodOperation(mulmodExecutor, accounts.getPrimaryBenefactor(), 1, 1_000);
    final var inBundleTransferTx2 =
        accountTransactions.createTransfer(
            accounts.getSecondaryBenefactor(), accounts.getPrimaryBenefactor(), 2, BigInteger.ZERO);

    // both txs are valid
    final var selectedBundleRawTxs =
        new String[] {mulmodOk.rawTx(), inBundleTransferTx2.signedTransactionData()};

    final var sendSelectedBundleRequest =
        new SendBundleRequest(new BundleParams(selectedBundleRawTxs, Integer.toHexString(2)));
    final var sendSelectedBundleResponse =
        sendSelectedBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendSelectedBundleResponse.hasError()).isFalse();
    assertThat(sendSelectedBundleResponse.getResult().bundleHash()).isNotBlank();

    // assert second bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(mulmodOk.txHash()));
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(inBundleTransferTx2.transactionHash()));

    // while first bundle is not selected
    minerNode.verify(eth.expectNoTransactionReceipt(mulmodOverflow.txHash()));
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx1.transactionHash()));
  }

  @Test
  public void bundleWithRevertedTxIsNotMined() throws Exception {
    final RevertExample revertExample = deployRevertExample();

    // fund a new account
    final var recipient = accounts.createAccount("recipient");
    final var txHashFundRecipient =
        accountTransactions
            .createTransfer(accounts.getPrimaryBenefactor(), recipient, 10, BigInteger.valueOf(1))
            .execute(minerNode.nodeRequests());
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHashFundRecipient.toHexString()));

    // create a tx that reverts
    final String contractAddress = revertExample.getContractAddress();
    final String txData = revertExample.setValue(BigInteger.ZERO).encodeFunctionCall();

    final RawTransaction txThatReverts =
        RawTransaction.createTransaction(
            CHAIN_ID,
            BigInteger.ZERO,
            TRANSFER_GAS_LIMIT,
            contractAddress,
            BigInteger.ZERO,
            txData,
            GAS_PRICE,
            GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE));
    final var signedTxThatReverts =
        Numeric.toHexString(
            TransactionEncoder.signMessage(
                txThatReverts, Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)));
    final var txThatRevertsHash = sha3(signedTxThatReverts);

    final var inBundleTransferTx =
        accountTransactions.createTransfer(recipient, accounts.getSecondaryBenefactor(), 1);

    // first tx reverts and bundle is not selected
    final var bundleRawTxs =
        new String[] {signedTxThatReverts, inBundleTransferTx.signedTransactionData()};

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(3)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    // transfer used as sentry to ensure a new block is mined without the bundles
    final var transferTxHash1 =
        accountTransactions
            .createTransfer(
                accounts.getPrimaryBenefactor(),
                accounts.getSecondaryBenefactor(),
                1,
                BigInteger.valueOf(2))
            .execute(minerNode.nodeRequests());

    // first sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash1.toHexString()));
    minerNode.verify(eth.expectNoTransactionReceipt(txThatRevertsHash));
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()));

    // try with a bundle where first is selected but second reverts
    final var reverseBundleRawTxs =
        new String[] {inBundleTransferTx.signedTransactionData(), signedTxThatReverts};
    final var sendReverseBundleRequest =
        new SendBundleRequest(new BundleParams(reverseBundleRawTxs, Integer.toHexString(4)));
    final var sendReverseBundleResponse =
        sendReverseBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendReverseBundleResponse.hasError()).isFalse();
    assertThat(sendReverseBundleResponse.getResult().bundleHash()).isNotBlank();

    // transfer used as sentry to ensure a new block is mined without the bundles
    final var transferTxHash2 =
        accountTransactions
            .createTransfer(accounts.getPrimaryBenefactor(), recipient, 1, BigInteger.valueOf(3))
            .execute(minerNode.nodeRequests());

    // second sentry is mined and no tx of the bundle is mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash2.toHexString()));
    minerNode.verify(eth.expectNoTransactionReceipt(txThatRevertsHash));
    minerNode.verify(eth.expectNoTransactionReceipt(inBundleTransferTx.transactionHash()));
  }
}
