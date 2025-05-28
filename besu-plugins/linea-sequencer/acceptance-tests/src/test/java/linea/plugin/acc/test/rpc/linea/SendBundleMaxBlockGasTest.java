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

import java.math.BigInteger;
import java.util.Arrays;
import java.util.List;

import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import linea.plugin.acc.test.tests.web3j.generated.AcceptanceTestToken;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.Test;

public class SendBundleMaxBlockGasTest extends AbstractSendBundleTest {
  private static final BigInteger BUNDLE_BLOCK_GAS_LIMIT = BigInteger.valueOf(100_000L);

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-max-bundle-block-gas=", BUNDLE_BLOCK_GAS_LIMIT.toString())
        .build();
  }

  @Test
  public void maxBlockGasForBundlesIsRespected() throws Exception {
    final AcceptanceTestToken token = deployAcceptanceTestToken();

    final int numOfTransfers = 2;

    // each token transfer has a gas limit of 100k so the bundle does not fit in the max block gas
    // reserved for bundles
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

    final var tokenTransferBundleRawTxs =
        Arrays.stream(tokenTransfers).map(TokenTransfer::rawTx).toArray(String[]::new);

    final var tokenTransferSendBundleRequest =
        new SendBundleRequest(new BundleParams(tokenTransferBundleRawTxs, Integer.toHexString(2)));
    final var tokenTransferSendBundleResponse =
        tokenTransferSendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(tokenTransferSendBundleResponse.hasError()).isFalse();
    assertThat(tokenTransferSendBundleResponse.getResult().bundleHash()).isNotBlank();

    // while 2 simple transfers each with a gas limit of 21k fit
    final TransferTransaction tx1 =
        accountTransactions.createTransfer(
            accounts.getSecondaryBenefactor(), accounts.getPrimaryBenefactor(), 1);
    final TransferTransaction tx2 =
        accountTransactions.createTransfer(
            accounts.getSecondaryBenefactor(), accounts.getPrimaryBenefactor(), 1);

    final String[] bundleRawTxs =
        new String[] {tx1.signedTransactionData(), tx2.signedTransactionData()};

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(2)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    // verify simple transfers are mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx1.transactionHash()));
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx2.transactionHash()));

    // but token transfers are not
    Arrays.stream(tokenTransfers)
        .forEach(
            tokenTransfer -> {
              minerNode.verify(eth.expectNoTransactionReceipt(tokenTransfer.txHash()));
            });
  }
}
