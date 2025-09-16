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

import java.util.List;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.Test;

public class SendBundleTest2 extends AbstractSendBundleTest {
  private static final Address DENY_TO_ADDRESS =
      Address.fromHexString("0xf17f52151EbEF6C7334FAD080c5704D77216b732");
  // Address 0x44b30d738d2dec1952b92c091724e8aedd52b9b2
  private static final String DENY_FROM_PRIVATE_KEY =
      "0xf326e86ba27e2286725a154922094f02573f4921a25a27046b74ec90e653438e";

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        // intentionally we do not set the deny list for bundles, but only the general one,
        // since we want to test the fallback for bundle to the default
        .set("--plugin-linea-deny-list-path=", getResourcePath("/defaultDenyList.txt"))
        .build();
  }

  @Test
  public void bundleTxRecipientOnDenyListIsNotAccepted() {
    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount(DENY_TO_ADDRESS);

    final TransferTransaction tx1 = accountTransactions.createTransfer(sender, recipient, 1);

    final String[] bundleRawTxs = new String[] {tx1.signedTransactionData()};

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(1)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isTrue();
    assertThat(sendBundleResponse.getError().getMessage())
        .isEqualTo(
            "Invalid transaction in bundle: hash 0xfb47ad29ecf898031bae210263198385f35818d4d154dc752d942a42acabc0cc, "
                + "reason: recipient 0xf17f52151ebef6c7334fad080c5704d77216b732 is blocked as appearing on the SDN or other legally prohibited list");
  }

  @Test
  public void bundleTxFromOnDenyListIsNotAccepted() {
    final Account sender = Account.fromPrivateKey(ethTransactions, "denied", DENY_FROM_PRIVATE_KEY);
    final Account recipient = accounts.getPrimaryBenefactor();

    final TransferTransaction tx1 = accountTransactions.createTransfer(sender, recipient, 1);

    final String[] bundleRawTxs = new String[] {tx1.signedTransactionData()};

    final var sendBundleRequest =
        new SendBundleRequest(new BundleParams(bundleRawTxs, Integer.toHexString(1)));
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isTrue();
    assertThat(sendBundleResponse.getError().getMessage())
        .isEqualTo(
            "Invalid transaction in bundle: hash 0xd631d31a09e865fcd0d86a7f7763747ece057f9f3a63350bb56a206051020a71,"
                + " reason: sender 0x44b30d738d2dec1952b92c091724e8aedd52b9b2 is blocked as appearing on the SDN or other legally prohibited list");
  }
}
