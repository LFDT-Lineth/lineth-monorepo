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

import java.math.BigInteger;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

import linea.plugin.acc.test.LineaPluginTestBase;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import linea.plugin.acc.test.tests.web3j.generated.ModExp;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.junit.jupiter.api.Test;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.utils.Numeric;

public class ModExpLimitsTest extends LineaPluginTestBase {

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        // disable line count validation to accept excluded precompile txs in the txpool
        .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "false")
        // set the module limits file
        .set("--plugin-linea-module-limit-file-path=", getResourcePath("/moduleLimits.toml"))
        .build();
  }

  /**
   * Tests the ModExp PRECOMPILE_MODEXP_EFFECTIVE_CALLS limit, that is the number of times the
   * corresponding circuit may be invoked in a single block.
   */
  @Test
  public void modExpLimitTest() throws Exception {
    Map<String, Integer> moduleLimits =
        ModuleLineCountValidator.createLimitModules(
            new LineaTracerConfiguration(getResourcePath("/moduleLimits.toml")));
    final int PRECOMPILE_MODEXP_EFFECTIVE_CALLS =
        moduleLimits.get("PRECOMPILE_MODEXP_EFFECTIVE_CALLS");

    /*
     * nTransactions: the number of transactions to try to include in the same block. The last
     *     one is not supposed to fit as it exceeds the limit, thus it is included in the next block
     * input: input data for each transaction
     * target: the expected string to be found in the blocks log
     */
    final int nTransactions = PRECOMPILE_MODEXP_EFFECTIVE_CALLS + 1;
    final String input =
        "000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001aabbcc";
    final String target =
        "Cumulated line count for module PRECOMPILE_MODEXP_EFFECTIVE_CALLS="
            + (PRECOMPILE_MODEXP_EFFECTIVE_CALLS + 1)
            + " is above the limit "
            + PRECOMPILE_MODEXP_EFFECTIVE_CALLS
            + ", stopping selection";

    // Deploy the ModExp contract
    final ModExp modExp = deployModExp();

    // Create an account to send the transactions
    Account modExpSender = accounts.createAccount("modExpSender");

    // Fund the account using secondary benefactor
    String fundTxHash =
        accountTransactions
            .createTransfer(accounts.getSecondaryBenefactor(), modExpSender, 1, BigInteger.ZERO)
            .execute(minerNode.nodeRequests())
            .toHexString();
    // Verify that the transaction for transferring funds was successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(fundTxHash));

    String[] txHashes = new String[nTransactions];
    for (int i = 0; i < nTransactions; i++) {
      // With decreasing nonce we force the transactions to be included in the same block
      // i     = 0                , 1                , ..., nTransactions - 1
      // nonce = nTransactions - 1, nTransactions - 2, ..., 0
      int nonce = nTransactions - 1 - i;

      // Craft the transaction data
      final byte[] encodedCallEcRecover =
          encodedCallModExp(modExp, modExpSender, nonce, Bytes.fromHexString(input));

      // Send the transaction
      final Web3j web3j = minerNode.nodeRequests().eth();
      final EthSendTransaction resp =
          web3j.ethSendRawTransaction(Numeric.toHexString(encodedCallEcRecover)).send();

      // Store the transaction hash
      txHashes[nonce] = resp.getTransactionHash();
    }

    // Transfer used as sentry to ensure a new block is mined
    final Hash transferTxHash =
        accountTransactions
            .createTransfer(
                accounts.getPrimaryBenefactor(),
                accounts.getSecondaryBenefactor(),
                1,
                BigInteger.ONE) // nonce is 1 as primary benefactor also deploys the contract
            .execute(minerNode.nodeRequests());
    // Wait for the sentry to be mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()));

    // Assert that all the transactions involving the EcPairing precompile, but the last one, were
    // included in the same block
    assertTransactionsMinedInSameBlock(
        minerNode.nodeRequests().eth(), Arrays.asList(txHashes).subList(0, nTransactions - 1));

    // Assert that the last transaction was included in another block
    assertTransactionsMinedInSeparateBlocks(
        minerNode.nodeRequests().eth(), List.of(txHashes[0], txHashes[nTransactions - 1]));

    // Assert that the target string is contained in the blocks log
    final String blockLog = getAndResetLog();
    assertThat(blockLog).contains(target);
  }
}
