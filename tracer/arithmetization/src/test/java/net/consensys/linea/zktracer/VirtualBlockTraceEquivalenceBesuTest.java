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

package net.consensys.linea.zktracer;

import static org.junit.jupiter.api.Assumptions.assumeTrue;
import static org.junit.jupiter.api.parallel.ExecutionMode.SAME_THREAD;

import java.util.List;
import java.util.Optional;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BesuExecutionTools;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.parallel.Execution;

/**
 * Verifies that a virtual block trace produced by {@code
 * linea_generateVirtualBlockConflatedTracesToFileV1} passes corset constraint validation for a
 * simple single-transaction block.
 *
 * <p>The virtual trace is generated via {@link
 * org.hyperledger.besu.plugin.services.BlockSimulationService}, which replaces each transaction's
 * real ECDSA signature (R, S, V) with a fake one. This means the virtual trace is intentionally not
 * byte-for-byte identical to the canonical trace. The meaningful assertion is that the trace
 * satisfies the ZK arithmetic constraints — which is what corset validation checks.
 */
@Execution(SAME_THREAD)
public class VirtualBlockTraceEquivalenceBesuTest extends TracerTestBase {

  /**
   * Sends a simple ETH-transfer on a live Besu node, generates the canonical conflated trace and
   * the virtual block trace for the same single-transaction block, then corset-validates the
   * virtual trace.
   *
   * <p>Requires the {@code besu.traces.dir} system property — the directory where the Besu plugin
   * writes trace files.
   */
  @Test
  void virtualBlockTracePassesCorsetValidation(TestInfo testInfo) {
    assumeTrue(
        System.getProperty("besu.traces.dir") != null,
        "Skipping: besu.traces.dir system property not set — Besu node tests require this");

    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(keyPair.getPublicKey());

    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(1)).nonce(0).address(senderAddress).build();

    // Simple ETH transfer — no contract code, exercises the minimal EVM execution path
    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .toAddress(Address.fromHexString("0x000000000000000000000000000000000000dEaD"))
            .value(Wei.of(1))
            .keyPair(keyPair)
            .build();

    final BesuExecutionTools tools =
        new BesuExecutionTools(
            Optional.of(testInfo),
            chainConfig,
            ToyExecutionEnvironmentV2.DEFAULT_COINBASE_ADDRESS,
            List.of(senderAccount),
            List.of(tx),
            /* oneTxPerBlock= */ false,
            /* customGenesisFile= */ null);

    // After generating and corset-validating the canonical trace, also generate the virtual block
    // trace for the same block number and transactions, and corset-validate it.
    tools.setValidateVirtualBlockTraces(true);
    tools.executeTest();
  }
}
