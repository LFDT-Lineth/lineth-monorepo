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
import java.util.stream.IntStream;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.web3j.protocol.core.methods.response.TransactionReceipt;

@Disabled(
    "Temporarily disabled while investigating bunlde timeout logic and is disabled in production")
public class BundleSelectionTimeoutTest extends AbstractSendBundleTest {

  @Test
  public void singleBundleSelectionTimeout() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();

    final var calls =
        IntStream.rangeClosed(1, 10)
            .mapToObj(
                nonce ->
                    mulmodOperation(
                        mulmodExecutor,
                        accounts.getPrimaryBenefactor(),
                        nonce,
                        2_000,
                        BigInteger.valueOf(MAX_TX_GAS_LIMIT / 10)))
            .toArray(MulmodCall[]::new);

    final var rawTxs = Arrays.stream(calls).map(MulmodCall::rawTx).toArray(String[]::new);

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(rawTxs, Integer.toHexString(2)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    final var transferTxHash =
        accountTransactions
            .createTransfer(accounts.getSecondaryBenefactor(), accounts.getPrimaryBenefactor(), 1)
            .execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()));

    // none of the bundle txs must be included in a block
    Arrays.stream(calls)
        .map(MulmodCall::txHash)
        .forEach(
            txHash -> {
              minerNode.verify(eth.expectNoTransactionReceipt(txHash));
            });
  }

  @Test
  public void multipleBundleSelectionTimeout() throws Exception {
    final var mulmodExecutor = deployMulmodExecutor();

    final var calls =
        IntStream.rangeClosed(1, 30)
            .mapToObj(
                nonce ->
                    mulmodOperation(
                        mulmodExecutor,
                        accounts.getPrimaryBenefactor(),
                        nonce,
                        2_000,
                        BigInteger.valueOf(MAX_TX_GAS_LIMIT / 10)))
            .toArray(MulmodCall[]::new);

    final var rawTxs = Arrays.stream(calls).map(MulmodCall::rawTx).toArray(String[]::new);

    final var sendBundleRequestSmall =
        new SendBundleRequest(
            new BundleParams(Arrays.copyOfRange(rawTxs, 0, 1), Integer.toHexString(2)));

    // this bundle is meant to go in timeout during its selection
    final var sendBundleRequestBig1 =
        new SendBundleRequest(
            new BundleParams(Arrays.copyOfRange(rawTxs, 1, 30), Integer.toHexString(2)));

    // second bundle contains one tx only to be fast to execute,
    // and ensure timeout occurs on the 2nd bundle and following are not event considered.
    // We are sending a bunch of bundles instead of just one to reproduce what happened in
    // production, where each following bundle where not skipped and would take ~200ms
    // to the not selected, due to the fact the first tx in the bundle was executed.
    final int followingBundleCount = 20;
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
        .forEach(
            txHash -> {
              minerNode.verify(eth.expectNoTransactionReceipt(txHash));
            });
  }
}
