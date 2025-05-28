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
package linea.plugin.acc.test.rpc.linea;

import static org.assertj.core.api.Assertions.assertThat;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.List;

import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.junit.jupiter.api.Test;

public class EstimateGasCompatibilityModeTest extends EstimateGasTest {
  private static final BigDecimal PRICE_MULTIPLIER = BigDecimal.valueOf(1.2);

  @Override
  public List<String> getTestCliOptions() {
    return getTestCommandLineOptionsBuilder()
        .set("--plugin-linea-estimate-gas-compatibility-mode-enabled=", "true")
        .set(
            "--plugin-linea-estimate-gas-compatibility-mode-multiplier=",
            PRICE_MULTIPLIER.toPlainString())
        .build();
  }

  @Override
  protected void assertIsProfitable(
      final Transaction tx,
      final Wei baseFee,
      final Wei estimatedMaxGasPrice,
      final long estimatedGasLimit) {
    final var minGasPrice = minerNode.getMiningParameters().getMinTransactionGasPrice();
    final var minPriorityFee = minGasPrice.subtract(baseFee);
    final var compatibilityMinPriorityFee =
        Wei.of(
            PRICE_MULTIPLIER
                .multiply(new BigDecimal(minPriorityFee.getAsBigInteger()))
                .setScale(0, RoundingMode.CEILING)
                .toBigInteger());

    // since we are in compatibility mode, we want to check that returned profitable priority fee is
    // the min priority fee per gas * multiplier + base fee
    final var expectedMaxGasPrice = baseFee.add(compatibilityMinPriorityFee);
    assertThat(estimatedMaxGasPrice).isEqualTo(expectedMaxGasPrice);
  }

  @Override
  protected void assertMinGasPriceLowerBound(final Wei baseFee, final Wei estimatedMaxGasPrice) {
    // since we are in compatibility mode, we want to check that returned profitable priority fee is
    // the min priority fee per gas * multiplier + base fee
    assertIsProfitable(null, baseFee, estimatedMaxGasPrice, 0);
  }

  @Test
  public void lineaEstimateGasPriorityFeeMinGasPriceLowerBound() {
    final Account sender = accounts.getSecondaryBenefactor();

    final CallParams callParams =
        new CallParams(null, sender.getAddress(), null, null, "", "", "0", null, null, null);

    final var reqLinea = new LineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests()).getResult();

    final var baseFee = Wei.fromHexString(respLinea.baseFeePerGas());
    final var estimatedPriorityFee = Wei.fromHexString(respLinea.priorityFeePerGas());
    final var estimatedMaxGasPrice = baseFee.add(estimatedPriorityFee);

    assertMinGasPriceLowerBound(baseFee, estimatedMaxGasPrice);
  }
}
