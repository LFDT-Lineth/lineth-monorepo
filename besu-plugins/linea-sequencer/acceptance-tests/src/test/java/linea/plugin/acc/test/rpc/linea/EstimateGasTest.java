/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea;

import static com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL;
import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.annotation.JsonInclude;
import java.io.IOException;
import java.math.BigInteger;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import linea.plugin.acc.test.tests.web3j.generated.SimpleStorage;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.config.LineaProfitabilityCliOptions;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.rpc.methods.LineaEstimateGas;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt64;
import org.bouncycastle.crypto.digests.KeccakDigest;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.response.RpcErrorType;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.web3j.protocol.core.Request;
import org.web3j.protocol.http.HttpService;

public class EstimateGasTest extends LineaPluginTestBase {
  protected static final int FIXED_GAS_COST_WEI = 0;
  protected static final int VARIABLE_GAS_COST_WEI = 1_000_000_000;
  protected static final double MIN_MARGIN = 1.0;
  protected static final double ESTIMATE_GAS_MIN_MARGIN = 1.1;
  protected static final Wei MIN_GAS_PRICE = Wei.of(1_000_000_000);
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
        .set("--plugin-linea-estimate-gas-min-margin=", String.valueOf(ESTIMATE_GAS_MIN_MARGIN))
        .set("--plugin-linea-max-tx-gas-limit=", String.valueOf(MAX_TRANSACTION_GAS_LIMIT));
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
            .estimateGasMinMargin(ESTIMATE_GAS_MIN_MARGIN)
            .build();
  }

  @Test
  public void lineaEstimateGasMatchesEthEstimateGas() {

    final Account sender = accounts.getSecondaryBenefactor();

    final CallParams callParams =
        new CallParams(
            null,
            sender.getAddress(),
            null,
            sender.getAddress(),
            null,
            Bytes.EMPTY.toHexString(),
            "0",
            null,
            null,
            null);

    final var reqEth = new RawEstimateGasRequest(callParams);
    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respEth = reqEth.execute(minerNode.nodeRequests());
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respEth).isEqualTo(respLinea.getResult().gasLimit());
  }

  @Test
  public void passingGasPriceFieldWorks() {

    final Account sender = accounts.getSecondaryBenefactor();

    final CallParams callParams =
        new CallParams(
            null,
            sender.getAddress(),
            null,
            sender.getAddress(),
            null,
            Bytes.EMPTY.toHexString(),
            "0",
            "0x1234",
            null,
            null);

    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.hasError()).isFalse();
    assertThat(respLinea.getResult()).isNotNull();
  }

  @Test
  public void passingChainIdFieldWorks() {

    final Account sender = accounts.getSecondaryBenefactor();

    final CallParams callParams =
        new CallParams(
            "0x539",
            sender.getAddress(),
            null,
            sender.getAddress(),
            null,
            Bytes.EMPTY.toHexString(),
            "0",
            "0x1234",
            null,
            null);

    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.hasError()).isFalse();
    assertThat(respLinea.getResult()).isNotNull();
  }

  @Test
  public void passingEIP1559FieldsWorks() {

    final Account sender = accounts.getSecondaryBenefactor();

    final CallParams callParams =
        new CallParams(
            null,
            sender.getAddress(),
            null,
            sender.getAddress(),
            null,
            Bytes.EMPTY.toHexString(),
            "0",
            null,
            "0x1234",
            "0x1");

    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.hasError()).isFalse();
    assertThat(respLinea.getResult()).isNotNull();
  }

  @Test
  public void passingChainIdAndEIP1559FieldsWorks() {

    final Account sender = accounts.getSecondaryBenefactor();

    final CallParams callParams =
        new CallParams(
            "0x539",
            sender.getAddress(),
            null,
            sender.getAddress(),
            null,
            Bytes.EMPTY.toHexString(),
            "0",
            null,
            "0x1234",
            null);

    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.hasError()).isFalse();
    assertThat(respLinea.getResult()).isNotNull();
  }

  @Test
  public void passingStateOverridesWorks() {

    final Account sender = accounts.getSecondaryBenefactor();

    final var actualBalance = minerNode.execute(ethTransactions.getBalance(sender));

    assertThat(actualBalance).isGreaterThan(BigInteger.ONE);

    final CallParams callParams =
        new CallParams(
            "0x539",
            sender.getAddress(),
            null,
            sender.getAddress(),
            "1",
            Bytes.EMPTY.toHexString(),
            "0",
            "0x1234",
            null,
            null);

    final var zeroBalance = Map.of("balance", Wei.ZERO.toHexString());

    final var stateOverrides = Map.of(accounts.getSecondaryBenefactor().getAddress(), zeroBalance);

    final var reqLinea = new LineaEstimateGasRequest(callParams, stateOverrides);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.hasError()).isTrue();
    assertThat(respLinea.getError().getCode()).isEqualTo(-32004);
    assertThat(respLinea.getError().getMessage())
        .isEqualTo(
            "Upfront cost exceeds account balance (transaction up-front cost 0x208cbab601 exceeds transaction sender account balance 0x0)");
  }

  @Test
  public void passingNonceWorks() {

    final Account sender = accounts.getSecondaryBenefactor();

    final CallParams callParams =
        new CallParams(
            null,
            sender.getAddress(),
            "0",
            sender.getAddress(),
            null,
            Bytes.EMPTY.toHexString(),
            "0",
            null,
            "0x1234",
            null);

    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.hasError()).isFalse();
    assertThat(respLinea.getResult()).isNotNull();

    // try with a future nonce
    final CallParams callParamsFuture =
        new CallParams(
            null,
            sender.getAddress(),
            "10",
            sender.getAddress(),
            null,
            Bytes.EMPTY.toHexString(),
            "0",
            null,
            "0x1234",
            null);

    final var reqLineaFuture = new LineaEstimateGasRequest(callParamsFuture);
    final var respLineaFuture = reqLineaFuture.execute(minerNode.nodeRequests());
    assertThat(respLineaFuture.hasError()).isFalse();
    assertThat(respLineaFuture.getResult()).isNotNull();
  }

  @Test
  public void lineaEstimateGasIsProfitable() {

    final Account sender = accounts.getSecondaryBenefactor();

    final KeccakDigest keccakDigest = new KeccakDigest(256);
    final StringBuilder txData = new StringBuilder();
    txData.append("0x");
    for (int i = 0; i < 5; i++) {
      keccakDigest.update(new byte[] {(byte) i}, 0, 1);
      final byte[] out = new byte[32];
      keccakDigest.doFinal(out, 0);
      txData.append(new BigInteger(out).abs());
    }
    final var payload = Bytes.wrap(txData.toString().getBytes(StandardCharsets.UTF_8));

    final CallParams callParams =
        new CallParams(
            null,
            sender.getAddress(),
            null,
            sender.getAddress(),
            null,
            payload.toHexString(),
            "0",
            null,
            null,
            null);

    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests()).getResult();

    final var estimatedGasLimit = UInt64.fromHexString(respLinea.gasLimit()).toLong();
    final var baseFee = Wei.fromHexString(respLinea.baseFeePerGas());
    final var estimatedPriorityFee = Wei.fromHexString(respLinea.priorityFeePerGas());
    final var estimatedMaxGasPrice = baseFee.add(estimatedPriorityFee);

    final var tx =
        org.hyperledger.besu.ethereum.core.Transaction.builder()
            .sender(Address.fromHexString(sender.getAddress()))
            .to(Address.fromHexString(sender.getAddress()))
            .gasLimit(estimatedGasLimit)
            .gasPrice(estimatedMaxGasPrice)
            .chainId(BigInteger.valueOf(CHAIN_ID))
            .value(Wei.ZERO)
            .payload(payload)
            .signature(LineaEstimateGas.FAKE_SIGNATURE_FOR_SIZE_CALCULATION)
            .build();

    assertIsProfitable(tx, baseFee, estimatedMaxGasPrice, estimatedGasLimit);
  }

  protected void assertIsProfitable(
      final org.hyperledger.besu.ethereum.core.Transaction tx,
      final Wei baseFee,
      final Wei estimatedMaxGasPrice,
      final long estimatedGasLimit) {

    final var minGasPrice = minerNode.getMiningParameters().getMinTransactionGasPrice();

    final var profitabilityCalculator = new TransactionProfitabilityCalculator(profitabilityConf);

    assertThat(
            profitabilityCalculator.isProfitable(
                "Test",
                tx,
                profitabilityConf.minMargin(),
                baseFee,
                estimatedMaxGasPrice,
                estimatedGasLimit,
                minGasPrice))
        .isTrue();
  }

  @Test
  public void invalidParametersLineaEstimateGasRequestReturnErrorResponse() {
    final Account sender = accounts.getSecondaryBenefactor();
    final CallParams callParams =
        new CallParams(
            null,
            sender.getAddress(),
            null,
            null,
            "",
            "",
            String.valueOf(Integer.MAX_VALUE),
            null,
            null,
            null);
    final var reqLinea = new BadLineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.getCode()).isEqualTo(RpcErrorType.INVALID_PARAMS.getCode());
    assertThat(respLinea.getMessage()).isEqualTo(RpcErrorType.INVALID_PARAMS.getMessage());
  }

  @Test
  public void revertedTransactionReturnErrorResponse() throws Exception {
    final SimpleStorage simpleStorage = deploySimpleStorage();
    final Account sender = accounts.getSecondaryBenefactor();
    final var reqLinea =
        new BadLineaEstimateGasRequest(
            new CallParams(
                null,
                sender.getAddress(),
                null,
                simpleStorage.getContractAddress(),
                "",
                "",
                "0",
                null,
                null,
                null));
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.getCode()).isEqualTo(-32000);
    assertThat(respLinea.getMessage()).isEqualTo("Execution reverted");
    assertThat(respLinea.getData()).isEqualTo("\"0x\"");
  }

  @Test
  public void failedTransactionReturnErrorResponse() {
    final Account sender = accounts.getSecondaryBenefactor();
    final var reqLinea =
        new BadLineaEstimateGasRequest(
            new CallParams(
                null,
                sender.getAddress(),
                null,
                null,
                "",
                Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY,
                "0",
                null,
                null,
                null));
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.getCode()).isEqualTo(-32000);
    assertThat(respLinea.getMessage())
        .isEqualTo(
            "Transaction processing could not be completed due to an exception (Invalid opcode: 0xc8)");
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
            {"jsonrpc":"2.0","method":"linea_estimateGas","params":[malformed json],"id":53}
            """))
            .build();
    final var errorResponse = httpClient.send(badJsonRequest, HttpResponse.BodyHandlers.ofString());
    assertThat(errorResponse.body())
        .isEqualTo(
            """
        {"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}""");
  }

  protected void assertMinGasPriceLowerBound(final Wei baseFee, final Wei estimatedMaxGasPrice) {
    final var minGasPrice = minerNode.getMiningParameters().getMinTransactionGasPrice();
    assertThat(estimatedMaxGasPrice).isEqualTo(minGasPrice);
  }

  static class LineaEstimateGasRequest
      implements Transaction<LineaEstimateGasRequest.LineaEstimateGasResponse> {
    private final CallParams callParams;
    private final Map<String, Map<String, String>> stateOverrides;

    public LineaEstimateGasRequest(final CallParams callParams) {
      this(callParams, null);
    }

    public LineaEstimateGasRequest(
        final CallParams callParams, final Map<String, Map<String, String>> stateOverrides) {
      this.callParams = callParams;
      this.stateOverrides = stateOverrides;
    }

    @Override
    public LineaEstimateGasResponse execute(final NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "linea_estimateGas",
                Arrays.asList(callParams, stateOverrides),
                nodeRequests.getWeb3jService(),
                LineaEstimateGasResponse.class)
            .send();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }

    static class LineaEstimateGasResponse extends org.web3j.protocol.core.Response<Response> {}

    record Response(String gasLimit, String baseFeePerGas, String priorityFeePerGas) {}
  }

  static class BadLineaEstimateGasRequest
      implements Transaction<org.web3j.protocol.core.Response.Error> {
    private final CallParams badCallParams;

    public BadLineaEstimateGasRequest(final CallParams badCallParams) {
      this.badCallParams = badCallParams;
    }

    @Override
    public org.web3j.protocol.core.Response.Error execute(final NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "linea_estimateGas",
                List.of(badCallParams),
                nodeRequests.getWeb3jService(),
                BadLineaEstimateGasResponse.class)
            .send()
            .getError();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }

    static class BadLineaEstimateGasResponse extends org.web3j.protocol.core.Response<Response> {}

    record Response(String gasLimit, String baseFeePerGas, String priorityFeePerGas) {}
  }

  static class RawEstimateGasRequest implements Transaction<String> {
    private final CallParams callParams;

    public RawEstimateGasRequest(final CallParams callParams) {
      this.callParams = callParams;
    }

    @Override
    public String execute(final NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "eth_estimateGas",
                List.of(callParams),
                nodeRequests.getWeb3jService(),
                RawEstimateGasResponse.class)
            .send()
            .getResult();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }

    static class RawEstimateGasResponse extends org.web3j.protocol.core.Response<String> {}
  }

  @JsonInclude(NON_NULL)
  record CallParams(
      String chainId,
      String from,
      String nonce,
      String to,
      String value,
      String data,
      String gas,
      String gasPrice,
      String maxFeePerGas,
      String maxPriorityFeePerGas) {}

  record StateOverride(String account, String balance) {}
}
