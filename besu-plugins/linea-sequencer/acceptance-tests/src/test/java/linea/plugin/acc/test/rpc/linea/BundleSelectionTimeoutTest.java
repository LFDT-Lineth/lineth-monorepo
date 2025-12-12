/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea;

import static org.assertj.core.api.Assertions.assertThat;

import java.math.BigInteger;
import java.util.Arrays;
import java.util.List;
import java.util.stream.IntStream;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import linea.plugin.acc.test.tests.web3j.generated.MulmodExecutor;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.junit.jupiter.api.Test;
import org.web3j.protocol.core.methods.response.TransactionReceipt;

public class BundleSelectionTimeoutTest extends AbstractSendBundleTest {

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        // set the module limits file
        .set(
            "--plugin-linea-module-limit-file-path=",
            getResourcePath("/moduleLimitsLimitless.toml"))
        // enabled the ZkCounter
        .set("--plugin-linea-limitless-enabled=", "true")
        .build();
  }

  private MulmodCall[] generateMulmodCalls(
      Account account,
      MulmodExecutor mulmodExecutor,
      int startNonce,
      int endNonce,
      int numberOfMulmodIterations,
      int gasLimit)
      throws Exception {
    return IntStream.rangeClosed(startNonce, endNonce)
        .mapToObj(
            nonce ->
                mulmodOperation(
                    mulmodExecutor,
                    account,
                    nonce,
                    numberOfMulmodIterations,
                    BigInteger.valueOf(gasLimit)))
        .toArray(MulmodCall[]::new);
  }

  private SendBundleRequest createSendBundleRequest(MulmodCall[] calls, long blockNumber) {
    final var rawTxs = Arrays.stream(calls).map(MulmodCall::rawTx).toArray(String[]::new);
    return new SendBundleRequest(
        new BundleParams(rawTxs, /*blockNumber*/ Long.toHexString(blockNumber)));
  }

  @Test
  public void singleBundleSelectionTimeout() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();
    final var newAccounts = createAccounts(4, 5);
    // // stop automatic block production to
    // // ensure bundle and transfers are evaluated in the same block
    super.buildBlocksInBackground = false;

    final var callsBigBundle =
        generateMulmodCalls(
            newAccounts.get(0), mulmodExecutor, 0, 30, 2_000, MAX_TX_GAS_LIMIT / 10);
    final var callsSmallBundle =
        generateMulmodCalls(newAccounts.get(1), mulmodExecutor, 0, 3, 2, MAX_TX_GAS_LIMIT / 10);

    final var previousHeadBlockNumber = getLatestBlockNumber();
    final var sendBundleRequest =
        createSendBundleRequest(callsBigBundle, previousHeadBlockNumber + 1L);
    final var sendSmallBundleRequest =
        createSendBundleRequest(callsSmallBundle, previousHeadBlockNumber + 2L);

    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());
    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();
    final var sendSmallBundleResponse = sendSmallBundleRequest.execute(minerNode.nodeRequests());
    assertThat(sendSmallBundleResponse.hasError()).isFalse();
    assertThat(sendSmallBundleResponse.getResult().bundleHash()).isNotBlank();

    final var transferTxHash =
        accountTransactions
            .createTransfer(newAccounts.get(2), accounts.getPrimaryBenefactor(), 1)
            .execute(minerNode.nodeRequests());

    super.buildNewBlockAndWait();
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()));

    // none of the big bundle txs must be included in a block
    Arrays.stream(callsBigBundle)
        .map(MulmodCall::txHash)
        .forEach(
            txHash -> {
              minerNode.verify(eth.expectNoTransactionReceipt(txHash));
            });

    final var transfer2TxHash =
        accountTransactions
            .createTransfer(newAccounts.get(3), accounts.getPrimaryBenefactor(), 1)
            .execute(minerNode.nodeRequests());

    super.buildNewBlockAndWait();
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transfer2TxHash.toHexString()));
    // all tx in small bundle where included in a block
    Arrays.stream(callsSmallBundle)
        .map(MulmodCall::txHash)
        .forEach(
            txHash -> {
              minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash));
            });
  }

  @Test
  public void multipleBundleSelectionTimeout() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();

    final var calls =
        generateMulmodCalls(
            accounts.getPrimaryBenefactor(), mulmodExecutor, 1, 10, 2_000, MAX_TX_GAS_LIMIT / 10);

    final var rawTxs = Arrays.stream(calls).map(MulmodCall::rawTx).toArray(String[]::new);

    final var sendBundleRequestSmall =
        new SendBundleRequest(
            new BundleParams(Arrays.copyOfRange(rawTxs, 0, 1), Integer.toHexString(2)));

    // this bundle is meant to go in timeout during its selection
    final var sendBundleRequestBig1 =
        new SendBundleRequest(
            new BundleParams(Arrays.copyOfRange(rawTxs, 1, 10), Integer.toHexString(2)));

    // second bundle contains one tx only to be fast to execute,
    // and ensure timeout occurs on the 2nd bundle and following are not event considered.
    // We are sending a bunch of bundles instead of just one to reproduce what happened in
    // production, where each following bundle where not skipped and would take ~200ms
    // to be not selected, due to the fact the first tx in the bundle was executed.
    final int followingBundleCount = 5;
    final var followingSendBundleRequests = new SendBundleRequest[followingBundleCount];
    for (int i = 0; i < followingSendBundleRequests.length; i++) {
      followingSendBundleRequests[i] =
          new SendBundleRequest(
              new BundleParams(Arrays.copyOfRange(rawTxs, 1, 2 + i), Integer.toHexString(2)));
    }

    final var sendBundleResponseSmall = sendBundleRequestSmall.execute(minerNode.nodeRequests());
    final var sendBundleResponseBig1 = sendBundleRequestBig1.execute(minerNode.nodeRequests());
    final var followingBundleResponses =
        Arrays.stream(followingSendBundleRequests)
            .map(req -> req.execute(minerNode.nodeRequests()))
            .toList();

    final var transferTxHash =
        accountTransactions
            .createTransfer(accounts.getSecondaryBenefactor(), accounts.getPrimaryBenefactor(), 1)
            .execute(minerNode.nodeRequests());

    assertThat(sendBundleResponseSmall.hasError()).isFalse();
    assertThat(sendBundleResponseSmall.getResult().bundleHash()).isNotBlank();

    assertThat(sendBundleResponseBig1.hasError()).isFalse();
    assertThat(sendBundleResponseBig1.getResult().bundleHash()).isNotBlank();

    followingBundleResponses.forEach(
        resp -> {
          assertThat(resp.hasError()).isFalse();
          assertThat(resp.getResult().bundleHash()).isNotBlank();
        });

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()));
    final var transferReceipt = ethTransactions.getTransactionReceipt(transferTxHash.toHexString());
    assertThat(transferReceipt.execute(minerNode.nodeRequests()))
        .isPresent()
        .map(TransactionReceipt::getBlockNumber)
        .contains(BigInteger.TWO);

    // first bundle is successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(calls[0].txHash()));

    // following bundles are not selected
    Arrays.stream(calls)
        .skip(1)
        .map(MulmodCall::txHash)
        .forEach(txHash -> minerNode.verify(eth.expectNoTransactionReceipt(txHash)));
    final var log = getLog();
    assertThat(log).contains("PLUGIN_SELECTION_TIMEOUT");
    assertThat(log)
        .contains(
            "Bundle selection interrupted while processing bundle %s"
                .formatted(sendBundleResponseBig1.getResult().bundleHash()));
  }
}
