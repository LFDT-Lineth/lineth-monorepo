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

import java.util.List;
import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.web3j.tx.gas.DefaultGasProvider;

public class EstimateGasModuleLimitOverflowLimitlessTest extends LineaPluginTestBase {
  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        // set the module limits file
        .set(
            "--plugin-linea-module-limit-file-path=",
            getResourcePath("/moduleLimitsLimitless.toml"))
        // enabled the ZkCounter
        .set("--plugin-linea-limitless-enabled=", "true")
        .build();
  }

  @Test
  public void estimateGasFailsForExceedingModuleLineCountTest() throws Exception {
    final var modExp = deployModExp();

    final Bytes[] invalidInputs = {
      Bytes.fromHexString("0000000000000000000000000000000000000000000000000000000000000201"),
      Bytes.fromHexString("00000000000000000000000000000000000000000000000000000000000003"),
      Bytes.fromHexString("ff"),
      Bytes.fromHexString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
    };

    for (int i = 0; i < invalidInputs.length; i++) {
      final var modExpCalldata =
          modExp.callModExp(invalidInputs[i].toArrayUnsafe()).encodeFunctionCall();

      final EstimateGasTest.CallParams callParams =
          new EstimateGasTest.CallParams(
              null,
              accounts.getSecondaryBenefactor().getAddress(),
              null,
              modExp.getContractAddress(),
              null,
              modExpCalldata,
              "0",
              DefaultGasProvider.GAS_PRICE.toString(),
              null,
              null);

      final var reqLinea = new EstimateGasTest.BadLineaEstimateGasRequest(callParams);
      final var respLinea = reqLinea.execute(minerNode.nodeRequests());
      assertThat(respLinea.getCode()).isEqualTo(-32000);
      assertThat(respLinea.getMessage())
          .isEqualTo("Transaction line count for module MODEXP=2147483647 is above the limit 1");
    }
  }
}
