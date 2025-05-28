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

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;

import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;

public class ExtraDataPricingTestWithoutSetMinGasPrice extends ExtraDataPricingTest {

  @Override
  public List<String> getTestCliOptions() {
    return getTestCommandLineOptionsBuilder().build();
  }

  protected TestCommandLineOptionsBuilder getTestCommandLineOptionsBuilder() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-extra-data-pricing-enabled=", Boolean.TRUE.toString())
        .set("--plugin-linea-extra-data-set-min-gas-price-enabled=", Boolean.FALSE.toString());
  }

  @Disabled("disable since minGasPrice is not updated with this test")
  @Test
  public void updateMinGasPriceViaExtraData() {}

  @Test
  public void minGasPriceNotUpdatedViaExtraData() {
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
        .isEqualTo(MIN_GAS_PRICE);
  }
}
