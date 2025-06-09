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
import java.util.ArrayList;
import java.util.List;
import linea.plugin.acc.test.tests.web3j.generated.DummyAdder;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.tx.gas.DefaultGasProvider;
import org.web3j.utils.Numeric;

public class TransactionTraceLimitTest extends LineaPluginTestBase {

  private static final BigInteger GAS_LIMIT = DefaultGasProvider.GAS_LIMIT;
  private static final BigInteger VALUE = BigInteger.ZERO;
  private static final BigInteger GAS_PRICE = BigInteger.TEN.pow(9);

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-module-limit-file-path=", getResourcePath("/strictModuleLimits.toml"))
        .build();
  }

  @Test
  public void transactionsMinedInSeparateBlocksTest() throws Exception {
    final DummyAdder dummyAdder = deployDummyAdder();
    final Web3j web3j = minerNode.nodeRequests().eth();
    final String contractAddress = dummyAdder.getContractAddress();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    final String txData = dummyAdder.add(BigInteger.valueOf(100)).encodeFunctionCall();

    final ArrayList<String> hashes = new ArrayList<>(5);
    for (int i = 0; i < 5; i++) {
      final RawTransaction transaction =
          RawTransaction.createTransaction(
              CHAIN_ID,
              BigInteger.valueOf(i + 1),
              GAS_LIMIT,
              contractAddress,
              VALUE,
              txData,
              GAS_PRICE,
              GAS_PRICE.multiply(BigInteger.TEN));
      final byte[] signedTransaction = TransactionEncoder.signMessage(transaction, credentials);
      final EthSendTransaction response =
          web3j.ethSendRawTransaction(Numeric.toHexString(signedTransaction)).send();
      hashes.add(response.getTransactionHash());
    }

    // make sure that there are no more than one transaction per block, because the limit for the
    // add module only allows for one of these transactions.
    assertTransactionsMinedInSeparateBlocks(web3j, hashes);
  }
}
