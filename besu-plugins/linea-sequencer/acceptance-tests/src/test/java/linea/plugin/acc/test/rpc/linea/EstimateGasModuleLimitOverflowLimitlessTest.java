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
import linea.plugin.acc.test.LineaPluginPoSTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.web3j.tx.gas.DefaultGasProvider;

public class EstimateGasModuleLimitOverflowLimitlessTest extends LineaPluginPoSTestBase {
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
    final var bls12MapFPtoG1 = deployBLS12_MAP_FP_TO_G1();

    // Test cases from
    // https://github.com/hyperledger/besu/blob/main/evm/src/test/resources/org/hyperledger/besu/evm/precompile/fp_to_g1.csv
    final Bytes[] invalidInputs = {
      Bytes.fromHexString(
          "0000000000000000000000000000000014406e5bfb9209256a3820879a29ac2f62d6aca82324bf3ae2aa7d3c54792043bd8c791fccdb080c1a52dc68b8b69350"),
      Bytes.fromHexString(
          "000000000000000000000000000000000e885bb33996e12f07da69073e2c0cc880bc8eff26d2a724299eb12d54f4bcf26f4748bb020e80a7e3794a7b0e47a641"),
      Bytes.fromHexString(
          "000000000000000000000000000000000ba1b6d79150bdc368a14157ebfe8b5f691cf657a6bbe30e79b6654691136577d2ef1b36bfb232e3336e7e4c9352a8ed"),
      Bytes.fromHexString(
          "000000000000000000000000000000000f12847f7787f439575031bcdb1f03cfb79f942f3a9709306e4bd5afc73d3f78fd1c1fef913f503c8cbab58453fb7df2")
    };

    for (int i = 0; i < invalidInputs.length; i++) {
      final var calldata =
          bls12MapFPtoG1
              .callBLS12_MAP_FP_TO_G1(invalidInputs[i].toArrayUnsafe())
              .encodeFunctionCall();

      final EstimateGasTest.CallParams callParams =
          new EstimateGasTest.CallParams(
              null,
              accounts.getSecondaryBenefactor().getAddress(),
              null,
              bls12MapFPtoG1.getContractAddress(),
              null,
              calldata,
              "0",
              DefaultGasProvider.GAS_PRICE.toString(),
              null,
              null);

      final var reqLinea = new EstimateGasTest.BadLineaEstimateGasRequest(callParams);
      final var respLinea = reqLinea.execute(minerNode.nodeRequests());
      assertThat(respLinea.getCode()).isEqualTo(-32000);
      assertThat(respLinea.getMessage())
          .isEqualTo(
              "Transaction line count for module PRECOMPILE_BLS_MAP_FP_TO_G1_EFFECTIVE_CALLS=1 is above the limit 0");
    }
  }
}
