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
import static org.web3j.crypto.Hash.sha3;

import java.util.List;
import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.utils.Numeric;

public class EthSendRawTransactionSimulationModExpLimitlessTest extends LineaPluginTestBase {

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        // set the module limits file
        .set(
            "--plugin-linea-module-limit-file-path=",
            getResourcePath("/moduleLimitsLimitless.toml"))
        // enabled the ZkCounter
        .set("--plugin-linea-limitless-enabled=", "true")
        .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "true")
        .build();
  }

  @Test
  public void validModExpCallsAreAccepted() throws Exception {
    final var modExp = deployModExp();

    final Bytes[] validInputs = {
      Bytes.EMPTY,
      Bytes.fromHexString("0000000000000000000000000000000000000000000000000000000000000000"),
      Bytes.fromHexString("000000000000000000000000000000000000000000000000000000000000013f"),
      Bytes.fromHexString("0000000000000000000000000000000000000000000000000000000000000200"),
      Bytes.fromHexString("00000000000000000000000000000000000000000000000000000000000002")
    };

    for (int i = 0; i < validInputs.length; i++) {

      final var mulmodOverflow =
          encodedCallModExp(modExp, accounts.getSecondaryBenefactor(), i, validInputs[i]);

      final Web3j web3j = minerNode.nodeRequests().eth();
      final EthSendTransaction resp =
          web3j.ethSendRawTransaction(Numeric.toHexString(mulmodOverflow)).send();
      assertThat(resp.hasError()).isFalse();

      minerNode.verify(eth.expectSuccessfulTransactionReceipt(resp.getTransactionHash()));
    }
  }

  @Test
  public void invalidModExpCallsAreRejected() throws Exception {
    final var modExp = deployModExp();

    final Bytes[] invalidInputs = {
      Bytes.fromHexString("0000000000000000000000000000000000000000000000000000000000000201"),
      Bytes.fromHexString("00000000000000000000000000000000000000000000000000000000000003"),
      Bytes.fromHexString("ff"),
      Bytes.fromHexString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
    };

    for (int i = 0; i < invalidInputs.length; i++) {

      final var mulmodOverflow =
          encodedCallModExp(modExp, accounts.getSecondaryBenefactor(), i, invalidInputs[i]);

      final Web3j web3j = minerNode.nodeRequests().eth();
      final EthSendTransaction resp =
          web3j.ethSendRawTransaction(Numeric.toHexString(mulmodOverflow)).send();

      assertThat(resp.hasError()).isTrue();
      assertThat(resp.getError().getMessage())
          .isEqualTo(
              "Transaction "
                  + Numeric.toHexString(sha3(mulmodOverflow))
                  + " line count for module MODEXP=2147483647 is above the limit 1");

      assertThat(getTxPoolContent()).isEmpty();
    }
  }
}
