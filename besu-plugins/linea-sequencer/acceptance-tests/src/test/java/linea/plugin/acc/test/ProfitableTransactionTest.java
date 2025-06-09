/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test;

import java.math.BigInteger;
import java.util.List;
import org.bouncycastle.crypto.digests.KeccakDigest;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.tx.RawTransactionManager;
import org.web3j.tx.TransactionManager;

public class ProfitableTransactionTest extends LineaPluginTestBase {
  private static final double MIN_MARGIN = 1.5;
  private static final Wei MIN_GAS_PRICE = Wei.of(1_000_000_000);

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-min-margin=", String.valueOf(MIN_MARGIN))
        .build();
  }

  @BeforeEach
  public void setMinGasPrice() {
    minerNode.getMiningParameters().setMinTransactionGasPrice(MIN_GAS_PRICE);
  }

  @Test
  public void transactionIsNotMinedWhenUnprofitable() throws Exception {

    final Web3j web3j = minerNode.nodeRequests().eth();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    final TransactionManager txManager = new RawTransactionManager(web3j, credentials, CHAIN_ID);

    final KeccakDigest keccakDigest = new KeccakDigest(256);
    final StringBuilder txData = new StringBuilder();
    txData.append("0x");
    for (int i = 0; i < 10; i++) {
      keccakDigest.update(new byte[] {(byte) i}, 0, 1);
      final byte[] out = new byte[32];
      keccakDigest.doFinal(out, 0);
      txData.append(new BigInteger(out));
    }

    final var txUnprofitable =
        txManager.sendTransaction(
            MIN_GAS_PRICE.getAsBigInteger().divide(BigInteger.valueOf(100)),
            BigInteger.valueOf(MAX_TX_GAS_LIMIT / 2),
            credentials.getAddress(),
            txData.toString(),
            BigInteger.ZERO);

    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount("recipient");
    final TransferTransaction transferTx = accountTransactions.createTransfer(sender, recipient, 1);
    final var txHash = minerNode.execute(transferTx);

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toHexString()));

    // assert that tx below margin is not confirmed
    minerNode.verify(eth.expectNoTransactionReceipt(txUnprofitable.getTransactionHash()));
  }

  /**
   * if we have a list of transactions [t_small, t_tooBig, t_small, ..., t_small] where t_tooBig is
   * too big to fit in a block, we have blocks created that contain all t_small transactions.
   *
   * @throws Exception if send transaction fails
   */
  @Test
  public void transactionIsMinedWhenProfitable() {
    minerNode.getMiningParameters().setMinTransactionGasPrice(MIN_GAS_PRICE);
    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount("recipient");

    final TransferTransaction transferTx = accountTransactions.createTransfer(sender, recipient, 1);
    final var txHash = minerNode.execute(transferTx);

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toHexString()));
  }
}
