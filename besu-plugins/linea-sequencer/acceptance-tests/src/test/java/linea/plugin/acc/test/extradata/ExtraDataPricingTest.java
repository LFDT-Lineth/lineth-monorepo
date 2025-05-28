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
package linea.plugin.acc.test.extradata;

import static java.util.Map.entry;
import static net.consensys.linea.metrics.LineaMetricCategory.PRICING_CONF;
import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import java.math.BigInteger;
import java.util.List;

import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.crypto.RawTransaction;
import org.web3j.crypto.TransactionEncoder;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.Request;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.utils.Numeric;

public class ExtraDataPricingTest extends LineaPluginTestBase {
  protected static final Wei MIN_GAS_PRICE = Wei.of(1_000_000_000);
  protected static final int WEI_IN_KWEI = 1000;

  @Override
  public List<String> getTestCliOptions() {
    return getTestCommandLineOptionsBuilder().build();
  }

  protected TestCommandLineOptionsBuilder getTestCommandLineOptionsBuilder() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-extra-data-pricing-enabled=", Boolean.TRUE.toString());
  }

  @Test
  public void updateMinGasPriceViaExtraData() {
    minerNode.getMiningParameters().setMinTransactionGasPrice(MIN_GAS_PRICE);
    final var doubleMinGasPrice = MIN_GAS_PRICE.multiply(2);

    final var extraData =
        createExtraDataPricingField(
            0, MIN_GAS_PRICE.toLong() / WEI_IN_KWEI, doubleMinGasPrice.toLong() / WEI_IN_KWEI);
    final var reqSetExtraData = new MinerSetExtraDataRequest(extraData);
    final var respSetExtraData = reqSetExtraData.execute(minerNode.nodeRequests());

    assertThat(respSetExtraData).isTrue();

    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount("recipient");

    final TransferTransaction transferTx = accountTransactions.createTransfer(sender, recipient, 1);
    final var txHash = minerNode.execute(transferTx);

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toHexString()));

    assertThat(minerNode.getMiningParameters().getMinTransactionGasPrice())
        .isEqualTo(doubleMinGasPrice);
  }

  @Test
  public void updateProfitabilityParamsViaExtraData() throws IOException, InterruptedException {
    final Web3j web3j = minerNode.nodeRequests().eth();
    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount("recipient");
    minerNode.getMiningParameters().setMinTransactionGasPrice(MIN_GAS_PRICE);

    final var extraData =
        createExtraDataPricingField(
            MIN_GAS_PRICE.multiply(2).toLong() / WEI_IN_KWEI,
            MIN_GAS_PRICE.toLong() / WEI_IN_KWEI,
            MIN_GAS_PRICE.toLong() / WEI_IN_KWEI);
    final var reqSetExtraData = new ExtraDataPricingTest.MinerSetExtraDataRequest(extraData);
    final var respSetExtraData = reqSetExtraData.execute(minerNode.nodeRequests());

    assertThat(respSetExtraData).isTrue();

    // when this first tx is mined the above extra data pricing will have effect on following txs
    final TransferTransaction profitableTx =
        accountTransactions.createTransfer(sender, recipient, 1);
    final var profitableTxHash = minerNode.execute(profitableTx);

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(profitableTxHash.toHexString()));

    // this tx will be evaluated with the previously set extra data pricing to be unprofitable
    final RawTransaction unprofitableTx =
        RawTransaction.createTransaction(
            BigInteger.ZERO,
            MIN_GAS_PRICE.getAsBigInteger(),
            BigInteger.valueOf(21000),
            recipient.getAddress(),
            "");

    final byte[] signedUnprofitableTx =
        TransactionEncoder.signMessage(
            unprofitableTx, Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY));

    final EthSendTransaction signedUnprofitableTxResp =
        web3j.ethSendRawTransaction(Numeric.toHexString(signedUnprofitableTx)).send();

    assertThat(signedUnprofitableTxResp.hasError()).isTrue();
    assertThat(signedUnprofitableTxResp.getError().getMessage()).isEqualTo("Gas price too low");

    assertThat(getTxPoolContent()).isEmpty();

    final var fixedCostMetric =
        getMetricValue(PRICING_CONF, "values", List.of(entry("field", "fixed_cost_wei")));

    assertThat(fixedCostMetric).isEqualTo(MIN_GAS_PRICE.multiply(2).getValue().doubleValue());

    final var variableCostMetric =
        getMetricValue(PRICING_CONF, "values", List.of(entry("field", "variable_cost_wei")));

    assertThat(variableCostMetric).isEqualTo(MIN_GAS_PRICE.getValue().doubleValue());

    final var ethGasPriceMetric =
        getMetricValue(PRICING_CONF, "values", List.of(entry("field", "eth_gas_price_wei")));

    assertThat(ethGasPriceMetric).isEqualTo(MIN_GAS_PRICE.getValue().doubleValue());
  }

  static class MinerSetExtraDataRequest implements Transaction<Boolean> {
    private final Bytes32 extraData;

    public MinerSetExtraDataRequest(final Bytes32 extraData) {
      this.extraData = extraData;
    }

    @Override
    public Boolean execute(final NodeRequests nodeRequests) {
      try {
        return new Request<>(
                "miner_setExtraData",
                List.of(extraData.toHexString()),
                nodeRequests.getWeb3jService(),
                MinerSetExtraDataResponse.class)
            .send()
            .getResult();
      } catch (IOException e) {
        throw new RuntimeException(e);
      }
    }

    static class MinerSetExtraDataResponse extends org.web3j.protocol.core.Response<Boolean> {}
  }
}
