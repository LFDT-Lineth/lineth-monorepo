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

import com.google.common.base.Strings;
import java.io.IOException;
import java.math.BigInteger;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.List;
import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import net.consensys.linea.config.LineaProfitabilityCliOptions;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.bouncycastle.crypto.digests.KeccakDigest;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.Request;
import org.web3j.protocol.core.Response;
import org.web3j.protocol.http.HttpService;
import org.web3j.tx.RawTransactionManager;
import org.web3j.tx.TransactionManager;

public class SetExtraDataTest extends LineaPluginTestBase {
  protected static final int FIXED_GAS_COST_WEI = 0;
  protected static final int VARIABLE_GAS_COST_WEI = 1_000_000_000;
  protected static final double MIN_MARGIN = 1.5;
  protected static final Wei MIN_GAS_PRICE = Wei.of(1_000_000);
  protected static final int MAX_TRANSACTION_GAS_LIMIT = 30_000_000;
  protected LineaProfitabilityConfiguration profitabilityConf;

  @Override
  public List<String> getTestCliOptions() {
    return getTestCommandLineOptionsBuilder().build();
  }

  protected TestCommandLineOptionsBuilder getTestCommandLineOptionsBuilder() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-fixed-gas-cost-wei=", String.valueOf(FIXED_GAS_COST_WEI))
        .set("--plugin-linea-variable-gas-cost-wei=", String.valueOf(VARIABLE_GAS_COST_WEI))
        .set("--plugin-linea-min-margin=", String.valueOf(MIN_MARGIN))
        .set("--plugin-linea-max-tx-gas-limit=", String.valueOf(MAX_TRANSACTION_GAS_LIMIT))
        .set("--plugin-linea-extra-data-pricing-enabled=", "true");
  }

  @BeforeEach
  public void setMinGasPrice() {
    minerNode.getMiningParameters().setMinTransactionGasPrice(MIN_GAS_PRICE);
  }

  @BeforeEach
  public void createDefaultConfigurations() {
    profitabilityConf =
        LineaProfitabilityCliOptions.create().toDomainObject().toBuilder()
            .fixedCostWei(FIXED_GAS_COST_WEI)
            .variableCostWei(VARIABLE_GAS_COST_WEI)
            .minMargin(MIN_MARGIN)
            .build();
  }

  @Test
  public void setUnsupportedExtraDataReturnsError() {
    final var unsupportedExtraData = Bytes32.ZERO;

    final var reqLinea = new FailingLineaSetExtraDataRequest(unsupportedExtraData);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.getMessage())
        .isEqualTo(
            "Unsupported extra data field 0x0000000000000000000000000000000000000000000000000000000000000000");
  }

  @Test
  public void setTooLongExtraDataReturnsError() {
    final var tooLongExtraData = Bytes.concatenate(Bytes.of(1), Bytes32.ZERO);

    final var reqLinea = new FailingLineaSetExtraDataRequest(tooLongExtraData);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.getMessage()).isEqualTo("Expected 32 bytes but got 33");
  }

  @Test
  public void setTooShortExtraDataReturnsError() {
    final var tooShortExtraData = Bytes32.ZERO.slice(1);

    final var reqLinea = new FailingLineaSetExtraDataRequest(tooShortExtraData);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.getMessage()).isEqualTo("Expected 32 bytes but got 31");
  }

  @Test
  public void successfulSetExtraData() {
    final var extraData =
        Bytes32.fromHexString("0x0100000000000000000000000000000000000000000000000000000000000000");

    final var reqLinea = new LineaSetExtraDataRequest(extraData);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea).isTrue();
  }

  @Test
  public void successfulUpdateMinGasPrice() {
    final var doubledMinGasPriceKWei = MIN_GAS_PRICE.multiply(2).divide(1000);
    final var hexMinGasPrice =
        Strings.padStart(doubledMinGasPriceKWei.toShortHexString().substring(2), 8, '0');
    final var extraData =
        Bytes32.fromHexString(
            "0x010000000000000000" + hexMinGasPrice + "00000000000000000000000000000000000000");

    final var reqLinea = new LineaSetExtraDataRequest(extraData);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea).isTrue();
    assertThat(minerNode.getMiningParameters().getMinTransactionGasPrice())
        .isEqualTo(MIN_GAS_PRICE.multiply(2));
  }

  @Test
  public void successfulUpdatePricingParameters() throws IOException {
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
            MIN_GAS_PRICE.getAsBigInteger(),
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

    final var zeroFixedCostKWei = "00000000";
    final var minimalVariableCostKWei = "00000001";
    final var minimalMinGasPriceKWei = "00000002";
    final var extraData =
        Bytes32.fromHexString(
            "0x01"
                + zeroFixedCostKWei
                + minimalVariableCostKWei
                + minimalMinGasPriceKWei
                + "00000000000000000000000000000000000000");

    final var reqLinea = new LineaSetExtraDataRequest(extraData);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea).isTrue();
    assertThat(minerNode.getMiningParameters().getMinTransactionGasPrice()).isEqualTo(Wei.of(2000));
    // assert that tx is confirmed now
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txUnprofitable.getTransactionHash()));
  }

  @Test
  public void parseErrorLineaEstimateGasRequestReturnErrorResponse()
      throws IOException, InterruptedException {
    final var httpService = (HttpService) minerNode.nodeRequests().getWeb3jService();
    final var httpClient = HttpClient.newHttpClient();
    final var badJsonRequest =
        HttpRequest.newBuilder(URI.create(httpService.getUrl()))
            .headers("Content-Type", "application/json")
            .POST(
                HttpRequest.BodyPublishers.ofString(
                    """
            {"jsonrpc":"2.0","method":"linea_setExtraData","params":[malformed json],"id":53}
            """))
            .build();
    final var errorResponse = httpClient.send(badJsonRequest, HttpResponse.BodyHandlers.ofString());
    assertThat(errorResponse.body())
        .isEqualTo(
            """
        {"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}""");
  }

  static class LineaSetExtraDataRequest implements Transaction<Boolean> {
    private final Bytes32 extraData;

    public LineaSetExtraDataRequest(final Bytes32 extraData) {
      this.extraData = extraData;
    }

    @Override
    public Boolean execute(final NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "linea_setExtraData",
                List.of(extraData.toHexString()),
                nodeRequests.getWeb3jService(),
                LineaSetExtraDataResponse.class)
            .send()
            .getResult();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }
  }

  static class FailingLineaSetExtraDataRequest implements Transaction<Response.Error> {
    private final Bytes extraData;

    public FailingLineaSetExtraDataRequest(final Bytes extraData) {
      this.extraData = extraData;
    }

    @Override
    public Response.Error execute(final NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "linea_setExtraData",
                List.of(extraData.toHexString()),
                nodeRequests.getWeb3jService(),
                LineaSetExtraDataResponse.class)
            .send()
            .getError();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }
  }

  static class LineaSetExtraDataResponse extends org.web3j.protocol.core.Response<Boolean> {}
}
