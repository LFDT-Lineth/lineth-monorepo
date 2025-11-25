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

import java.io.IOException;
import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.util.List;
import linea.plugin.acc.test.tests.web3j.generated.LogEmitter;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.Request;
import org.web3j.tx.RawTransactionManager;
import org.web3j.tx.TransactionManager;

public class TransactionEventDenialTest extends LineaPluginTestBasePrague {

  private static final BigInteger GAS_PRICE = BigInteger.TEN.pow(9);
  private static final BigInteger GAS_LIMIT = BigInteger.valueOf(210_000);
  private static final BigInteger VALUE = BigInteger.ZERO;

  @TempDir static Path tempDir;
  static Path denyEventListPath;

  @Override
  public List<String> getTestCliOptions() {
    denyEventListPath = tempDir.resolve("denyEventList.txt");
    try {
      if (!Files.exists(denyEventListPath)) {
        Files.createFile(denyEventListPath);
      }
    } catch (IOException e) {
      throw new RuntimeException("Failed to create deny list file", e);
    }

    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-events-deny-list-path=", denyEventListPath.toString())
        .set("--plugin-linea-events-bundle-deny-list-path=", denyEventListPath.toString())
        .build();
  }

  /** Test that a transaction emitting a denied topic is rejected. */
  @Test
  public void transactionWithDeniedTopicIsRejected() throws Exception {
    LogEmitter logEmitter = deployLogEmitter();
    Web3j web3j = minerNode.nodeRequests().eth();
    TransactionManager txManager = createTransactionManager(web3j);

    String blockedTopic = "0xaa";
    byte[] payload = "data".getBytes(StandardCharsets.UTF_8);

    addDenyListFilterAndReload(logEmitter.getContractAddress(), blockedTopic);

    String txData =
        logEmitter
            .log1(Bytes32.fromHexString(blockedTopic).toArray(), payload)
            .encodeFunctionCall();
    var transaction =
        txManager.sendTransaction(
            GAS_PRICE, GAS_LIMIT, logEmitter.getContractAddress(), txData, VALUE);

    // transfer used as canary to ensure a new block is mined without the invalid txs
    final var transferTxHash =
        accountTransactions
            .createTransfer(accounts.getSecondaryBenefactor(), accounts.getSecondaryBenefactor(), 1)
            .execute(minerNode.nodeRequests());

    // Canary should be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()));

    // Denied transaction should not be mined
    minerNode.verify(eth.expectNoTransactionReceipt(transaction.getTransactionHash()));

    // Because the transaction is denied based on its emitted event,
    // it should not even be in the pool
    assertTransactionNotInThePool(transaction.getTransactionHash());

    // Assert that the target string is contained in the block creation log
    final String blockLog = getAndResetLog();
    assertThat(blockLog)
        .contains(
            "Transaction %s is blocked due to contract address and event logs appearing on SDN or other legally prohibited list"
                .formatted(transaction.getTransactionHash()));
  }

  /** Test that a transaction emitting denied topics, with a wildcard, is rejected. */
  @Test
  public void transactionWithDeniedTopicsWithWildcardIsRejected() throws Exception {
    LogEmitter logEmitter = deployLogEmitter();
    Web3j web3j = minerNode.nodeRequests().eth();
    TransactionManager txManager = createTransactionManager(web3j);

    String blockedTopic1 = "0xaa";
    String blockedByWildcardTopic = "0xbb";
    String blockedTopic3 = "0xcc";
    String blockedTopic4 = "0xdd";
    byte[] payload = "data".getBytes(StandardCharsets.UTF_8);

    addDenyListFilterAndReload(
        logEmitter.getContractAddress(), blockedTopic1, null, blockedTopic3, blockedTopic4);

    String txData =
        logEmitter
            .log4(
                Bytes32.fromHexString(blockedTopic1).toArray(),
                Bytes32.fromHexString(blockedByWildcardTopic).toArray(),
                Bytes32.fromHexString(blockedTopic3).toArray(),
                Bytes32.fromHexString(blockedTopic4).toArray(),
                payload)
            .encodeFunctionCall();
    var transaction =
        txManager.sendTransaction(
            GAS_PRICE, GAS_LIMIT, logEmitter.getContractAddress(), txData, VALUE);

    // transfer used as canary to ensure a new block is mined without the invalid txs
    final var transferTxHash =
        accountTransactions
            .createTransfer(accounts.getSecondaryBenefactor(), accounts.getSecondaryBenefactor(), 1)
            .execute(minerNode.nodeRequests());

    // Canary should be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()));

    // Denied transaction should not be mined
    minerNode.verify(eth.expectNoTransactionReceipt(transaction.getTransactionHash()));

    // Because the transaction is denied based on its emitted event,
    // it should not even be in the pool
    assertTransactionNotInThePool(transaction.getTransactionHash());

    // Assert that the target string is contained in the block creation log
    final String blockLog = getAndResetLog();
    assertThat(blockLog)
        .contains(
            "Transaction %s is blocked due to contract address and event logs appearing on SDN or other legally prohibited list"
                .formatted(transaction.getTransactionHash()));
  }

  /** Test that a transaction emitting denied topics, with a wildcard, is rejected. */
  @Test
  public void transactionWithoutDeniedTopicsWithWildcardIsAllowed() throws Exception {
    LogEmitter logEmitter = deployLogEmitter();
    Web3j web3j = minerNode.nodeRequests().eth();
    TransactionManager txManager = createTransactionManager(web3j);

    String blockedTopic1 = "0xaa";
    String blockedTopic2 = null;
    String blockedTopic3 = "0xcc";
    String blockedTopic4 = "0xdd";
    String allowedTopic1 = "0x11";
    String allowedTopic2 = "0x22";
    String allowedTopic3 = "0x33";
    String allowedTopic4 = "0x44";
    byte[] payload = "data".getBytes(StandardCharsets.UTF_8);

    addDenyListFilterAndReload(
        logEmitter.getContractAddress(),
        blockedTopic1,
        blockedTopic2,
        blockedTopic3,
        blockedTopic4);

    String txData =
        logEmitter
            .log4(
                Bytes32.fromHexString(allowedTopic1).toArray(),
                Bytes32.fromHexString(allowedTopic2).toArray(),
                Bytes32.fromHexString(allowedTopic3).toArray(),
                Bytes32.fromHexString(allowedTopic4).toArray(),
                payload)
            .encodeFunctionCall();
    var transaction =
        txManager.sendTransaction(
            GAS_PRICE, GAS_LIMIT, logEmitter.getContractAddress(), txData, VALUE);

    // Transaction should be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transaction.getTransactionHash()));
  }

  private TransactionManager createTransactionManager(Web3j web3j) {
    Credentials credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    return new RawTransactionManager(web3j, credentials, CHAIN_ID);
  }

  // Helper method to add a filter to the deny list and reload configuration
  private void addDenyListFilterAndReload(String address, String... blockedTopics)
      throws IOException {
    // Add filter to deny list
    addFilterToDenyList(address, blockedTopics);
    reloadPluginConfiguration();
  }

  private void addFilterToDenyList(String address, String... blockedTopics) throws IOException {
    String entry =
        String.format(
            "%s,%s,%s,%s,%s",
            address,
            blockedTopics.length > 0 && blockedTopics[0] != null ? blockedTopics[0] : "",
            blockedTopics.length > 1 && blockedTopics[1] != null ? blockedTopics[1] : "",
            blockedTopics.length > 2 && blockedTopics[2] != null ? blockedTopics[2] : "",
            blockedTopics.length > 3 && blockedTopics[3] != null ? blockedTopics[3] : "");
    Files.writeString(denyEventListPath, entry + "\n", StandardOpenOption.APPEND);
  }

  private void reloadPluginConfiguration() {
    ReloadPluginConfigRequest request = new ReloadPluginConfigRequest();
    String result = request.execute(minerNode.nodeRequests());
    assertThat(result).isEqualTo("Success");
  }

  static class ReloadPluginConfigRequest implements Transaction<String> {
    @Override
    public String execute(NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "plugins_reloadPluginConfig",
                List.of("net.consensys.linea.sequencer.txselection.LineaTransactionSelectorPlugin"),
                nodeRequests.getWeb3jService(),
                ReloadPluginConfigResponse.class)
            .send()
            .getResult();
      } catch (IOException e) {
        throw new RuntimeException("Failed to reload plugin configuration", e);
      }
    }
  }

  static class ReloadPluginConfigResponse extends org.web3j.protocol.core.Response<String> {}
}
