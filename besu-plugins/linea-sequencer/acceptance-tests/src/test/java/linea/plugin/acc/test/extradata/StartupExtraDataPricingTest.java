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
import java.util.Optional;

import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.Test;

public class StartupExtraDataPricingTest extends LineaPluginTestBase {
  private static final Wei VARIABLE_GAS_COST = Wei.of(1_200_300_000);
  private static final Wei MIN_GAS_PRICE = VARIABLE_GAS_COST.divide(2);
  private static final int WEI_IN_KWEI = 1000;

  @Override
  public List<String> getTestCliOptions() {
    return getTestCommandLineOptionsBuilder().build();
  }

  protected TestCommandLineOptionsBuilder getTestCommandLineOptionsBuilder() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-extra-data-pricing-enabled=", Boolean.TRUE.toString());
  }

  @Override
  protected Optional<Bytes32> maybeCustomGenesisExtraData() {
    final var genesisExtraData =
        createExtraDataPricingField(
            0, VARIABLE_GAS_COST.toLong() / WEI_IN_KWEI, MIN_GAS_PRICE.toLong() / WEI_IN_KWEI);

    return Optional.of(genesisExtraData);
  }

  @Test
  public void minGasPriceSetFromChainHeadExtraDataAtStartup() {
    // at startup the min gas price should be set from the current chain head block extra data
    assertThat(minerNode.getMiningParameters().getMinTransactionGasPrice())
        .isEqualTo(MIN_GAS_PRICE);

    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.createAccount("recipient");

    final TransferTransaction transferTx = accountTransactions.createTransfer(sender, recipient, 1);
    final var txHash = minerNode.execute(transferTx);

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toHexString()));
  }
}
