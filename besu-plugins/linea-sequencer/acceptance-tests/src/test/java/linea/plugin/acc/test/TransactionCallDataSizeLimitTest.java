/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test;

import static org.assertj.core.api.Assertions.assertThat;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import linea.plugin.acc.test.tests.web3j.generated.SimpleStorage;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.tx.RawTransactionManager;
import org.web3j.tx.TransactionManager;
import org.web3j.tx.gas.DefaultGasProvider;

public class TransactionCallDataSizeLimitTest extends LineaPluginTestBase {

  public static final int MAX_CALLDATA_SIZE = 1188; // contract has a call data size of 1160
  private static final BigInteger GAS_PRICE = DefaultGasProvider.GAS_PRICE;
  private static final BigInteger GAS_LIMIT = DefaultGasProvider.GAS_LIMIT;
  private static final BigInteger VALUE = BigInteger.ZERO;

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-max-tx-calldata-size=", String.valueOf(MAX_CALLDATA_SIZE))
        .set("--plugin-linea-max-block-calldata-size=", String.valueOf(MAX_CALLDATA_SIZE))
        .build();
  }

  @Test
  public void shouldMineTransactions() throws Exception {
    final SimpleStorage simpleStorage = deploySimpleStorage();

    List<String> accounts =
        List.of(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY, Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY);

    final Web3j web3j = minerNode.nodeRequests().eth();
    final List<Integer> numCharactersInStringList = List.of(150, 200, 400);

    numCharactersInStringList.forEach(
        num -> sendTransactionsWithGivenLengthPayload(simpleStorage, accounts, web3j, num));
  }

  @Test
  public void transactionIsMinedWhenNotTooBig() throws Exception {
    final SimpleStorage simpleStorage = deploySimpleStorage();
    final Web3j web3j = minerNode.nodeRequests().eth();
    final String contractAddress = simpleStorage.getContractAddress();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager = new RawTransactionManager(web3j, credentials, CHAIN_ID);

    final String txDataGood = simpleStorage.set("a".repeat(1200 - 80)).encodeFunctionCall();
    final String hashGood =
        txManager
            .sendTransaction(GAS_PRICE, GAS_LIMIT, contractAddress, txDataGood, VALUE)
            .getTransactionHash();

    // make sure that a transaction that is not too big was mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(hashGood));
  }

  @Test
  public void transactionIsNotMinedWhenTooBig() throws Exception {
    final SimpleStorage simpleStorage = deploySimpleStorage();
    final Web3j web3j = minerNode.nodeRequests().eth();
    final String contractAddress = simpleStorage.getContractAddress();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager = new RawTransactionManager(web3j, credentials, CHAIN_ID);

    final String txDataTooBig = simpleStorage.set("a".repeat(1200 - 79)).encodeFunctionCall();
    final var txTooBigResp =
        txManager.sendTransaction(GAS_PRICE, GAS_LIMIT, contractAddress, txDataTooBig, VALUE);

    assertThat(txTooBigResp.hasError()).isTrue();
    assertThat(txTooBigResp.getError().getMessage())
        .isEqualTo("Calldata of transaction is greater than the allowed max of 1188");
  }

  /**
   * if we have a list of transactions [t_small, t_tooBig, t_small, ..., t_small] where t_tooBig is
   * too big to fit in a block, we have blocks created that contain all t_small transactions.
   *
   * @throws Exception if send transaction fails
   */
  @Test
  public void multipleSmallTxsMinedWhileTxTooBigNot() throws Exception {
    final SimpleStorage simpleStorage = deploySimpleStorage();

    final Web3j web3j = minerNode.nodeRequests().eth();
    final String contractAddress = simpleStorage.getContractAddress();
    final Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    TransactionManager txManager = new RawTransactionManager(web3j, credentials, CHAIN_ID);

    final Account smallCalldataSender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount("recipient");

    final List<Hash> expectedConfirmedTxs = new ArrayList<>(4);

    expectedConfirmedTxs.add(
        minerNode.execute(accountTransactions.createTransfer(smallCalldataSender, recipient, 1)));

    final String txDataTooBig = simpleStorage.set("a".repeat(1200 - 79)).encodeFunctionCall();

    final var txTooBigResp =
        txManager.sendTransaction(
            GAS_PRICE, BigInteger.valueOf(MAX_TX_GAS_LIMIT), contractAddress, txDataTooBig, VALUE);

    expectedConfirmedTxs.addAll(
        minerNode.execute(
            accountTransactions.createIncrementalTransfers(smallCalldataSender, recipient, 3)));

    assertThat(txTooBigResp.hasError()).isTrue();
    assertThat(txTooBigResp.getError().getMessage())
        .isEqualTo("Calldata of transaction is greater than the allowed max of 1188");

    expectedConfirmedTxs.stream()
        .map(Hash::toHexString)
        .forEach(hash -> minerNode.verify(eth.expectSuccessfulTransactionReceipt(hash)));
  }
}
