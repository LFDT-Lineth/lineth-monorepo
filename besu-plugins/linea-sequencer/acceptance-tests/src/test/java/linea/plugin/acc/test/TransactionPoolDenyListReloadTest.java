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

import java.io.FileOutputStream;
import java.io.IOException;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.Request;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.tx.RawTransactionManager;
import org.web3j.utils.Convert;

public class TransactionPoolDenyListReloadTest extends LineaPluginTestBasePrague {

  private static final BigInteger GAS_PRICE = Convert.toWei("20", Convert.Unit.GWEI).toBigInteger();
  private static final BigInteger GAS_LIMIT = BigInteger.valueOf(210000);
  private static final BigInteger VALUE = BigInteger.ONE; // 1 wei

  @TempDir static Path tempDir;
  static Path tempDenyList;

  @Override
  public List<String> getTestCliOptions() {
    tempDenyList = tempDir.resolve("denyList.txt");
    if (!Files.exists(tempDenyList)) {

      try {
        Files.createFile(tempDenyList);
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-deny-list-path=", tempDenyList.toString())
        .build();
  }

  @Test
  public void testEmptyDenyList() throws Exception {
    Credentials allowedCredentials = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY);
    final Web3j miner = minerNode.nodeRequests().eth();

    RawTransactionManager transactionManager =
        new RawTransactionManager(miner, allowedCredentials, CHAIN_ID);
    assertAddressAllowed(transactionManager, allowedCredentials.getAddress());
  }

  @Test
  public void testEmptyDenyList_thenDenySender_cannotAddTxToPool() throws Exception {
    Credentials willBeDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY);
    final Web3j miner = minerNode.nodeRequests().eth();
    RawTransactionManager transactionManager =
        new RawTransactionManager(miner, willBeDenied, CHAIN_ID);

    assertAddressAllowed(transactionManager, willBeDenied.getAddress());

    addAddressToDenyList(willBeDenied.getAddress());
    reloadPluginConfig();

    assertAddressNotAllowed(transactionManager, willBeDenied.getAddress());
  }

  @Test
  public void testDenySender_cannotAddTxToPool_thenAllowSender() throws Exception {
    Credentials willBeDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY);
    final Web3j miner = minerNode.nodeRequests().eth();
    RawTransactionManager transactionManager =
        new RawTransactionManager(miner, willBeDenied, CHAIN_ID);

    addAddressToDenyList(willBeDenied.getAddress());
    reloadPluginConfig();
    assertAddressNotAllowed(transactionManager, willBeDenied.getAddress());

    emptyDenyList();
    reloadPluginConfig();
    assertAddressAllowed(transactionManager, willBeDenied.getAddress());
  }

  private void addAddressToDenyList(final String address) throws IOException {
    Files.writeString(tempDenyList, address);
  }

  private void emptyDenyList() throws IOException {
    FileOutputStream fileOutputStream = new FileOutputStream(tempDenyList.toFile(), false);
    fileOutputStream.write(new byte[] {});
    fileOutputStream.flush();
    fileOutputStream.close();
  }

  private void assertAddressAllowed(
      final RawTransactionManager transactionManager, final String address) throws IOException {
    EthSendTransaction transactionResponse =
        transactionManager.sendTransaction(GAS_PRICE, GAS_LIMIT, address, "", VALUE);
    assertThat(transactionResponse.getTransactionHash()).isNotNull();
    assertThat(transactionResponse.getError()).isNull();
  }

  private void assertAddressNotAllowed(
      final RawTransactionManager transactionManager, final String address) throws IOException {
    EthSendTransaction transactionResponse =
        transactionManager.sendTransaction(GAS_PRICE, GAS_LIMIT, address, "", VALUE);

    assertThat(transactionResponse.getTransactionHash()).isNull();
    assertThat(transactionResponse.getError().getMessage())
        .isEqualTo(
            "sender "
                + address
                + " is blocked as appearing on the SDN or other legally prohibited list");
  }

  private void reloadPluginConfig() {
    final var reqLinea = new ReloadPluginConfigRequest();
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea).isEqualTo("Success");
  }

  static class ReloadPluginConfigRequest implements Transaction<String> {

    public ReloadPluginConfigRequest() {}

    @Override
    public String execute(final NodeRequests nodeRequests) {
      try {
        // plugin name is class name
        return new Request<>(
                "plugins_reloadPluginConfig",
                List.of(
                    "net.consensys.linea.sequencer.txpoolvalidation.LineaTransactionPoolValidatorPlugin"),
                nodeRequests.getWeb3jService(),
                ReloadPluginConfigResponse.class)
            .send()
            .getResult();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }
  }

  static class ReloadPluginConfigResponse extends org.web3j.protocol.core.Response<String> {}
}
