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

import java.math.BigInteger;
import java.util.List;
import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import linea.plugin.acc.test.tests.web3j.generated.DummyAdder;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.junit.jupiter.api.Test;
import org.web3j.tx.gas.DefaultGasProvider;

public class EstimateGasModuleLimitOverflowTest extends LineaPluginTestBase {
  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set(
            "--plugin-linea-module-limit-file-path=",
            getResourcePath("/txOverflowModuleLimits.toml"))
        .build();
  }

  @Test
  public void estimateGasFailsForExceedingModuleLineCountTest() throws Exception {

    final Account sender = accounts.getPrimaryBenefactor();

    final DummyAdder dummyAdder = deployDummyAdder();
    final String txData = dummyAdder.add(BigInteger.valueOf(1)).encodeFunctionCall();

    final EstimateGasTest.CallParams callParams =
        new EstimateGasTest.CallParams(
            null,
            sender.getAddress(),
            null,
            dummyAdder.getContractAddress(),
            null,
            txData,
            "0",
            DefaultGasProvider.GAS_PRICE.toString(),
            null,
            null);

    final var reqLinea = new EstimateGasTest.BadLineaEstimateGasRequest(callParams);
    final var respLinea = reqLinea.execute(minerNode.nodeRequests());
    assertThat(respLinea.getCode()).isEqualTo(-32000);
    assertThat(respLinea.getMessage())
        .isEqualTo("Transaction line count for module WCP=349 is above the limit 306");
  }
}
