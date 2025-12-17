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

import static org.junit.jupiter.api.parallel.ExecutionMode.SAME_THREAD;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.parallel.Execution;

/*
 Test tracing per fork as well as fork switching, using a Besu node. The trace is then checked against the constraints.
*/
@Execution(SAME_THREAD)
public class ForkTracingAndSwitchingBesuTest extends TracerTestBase {

  /*
   Test tracing a simple transaction on different forks, each fork using an opcode introduced in that fork.
   We check that tracing and go-corset check pass on each fork with the correct sets of constraints (zkevm_fork.bin).
  */
  @Test
  void testPerFork(TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));

    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(1)).nonce(5).address(senderAddress).build();

    final BytecodeCompiler compiler =
        BytecodeCompiler.newProgram(chainConfig).push(32, 0xbeef).push(32, 0xdead).op(OpCode.ADD);

    switch (fork) {
      case OSAKA -> compiler.push(Bytes.fromHexString("0x07acaa")).op(OpCode.CLZ);
      default -> throw new IllegalArgumentException("Unsupported fork: " + fork);
    }

    final ToyAccount receiverAccount =
        ToyAccount.builder()
            .balance(Wei.ONE)
            .nonce(6)
            .address(Address.fromHexString("0x111111"))
            .code(compiler.compile())
            .build();

    final Transaction tx =
        ToyTransaction.builder().sender(senderAccount).to(receiverAccount).keyPair(keyPair).build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .runWithBesuNode(true)
        .build()
        .run();
  }

  @Disabled("update and reenable when tracer supports OSAKA and AMSTERDAM")
  @Test
  void testForkSwitchPragueToOsaka(TestInfo testInfo) {
    KeyPair keyPair = new SECP256K1().generateKeyPair();
    Address senderAddress = Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));

    ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(1)).nonce(5).address(senderAddress).build();

    BytecodeCompiler compilerMain =
        BytecodeCompiler.newProgram(chainConfig)
            .push(32, 0xbeef)
            .push(32, 0xdead)
            .op(OpCode.ADD)
            .push(Bytes.fromHexString("0x7F")) // value
            .push(32) // offset
            .op(OpCode.MSTORE)
            .push(32) // size (for MCOPY use)
            .push(32) // offset to trigger mem expansion  (for MCOPY use)
            .push(0); // dest offset  (for MCOPY use)

    // MCOPY
    Bytes codePrague = Bytes.concatenate(compilerMain.compile(), Bytes.fromHexString("0x5E"));

    // CLZ opcode
    Bytes codeOsaka = Bytes.concatenate(compilerMain.compile(), Bytes.fromHexString("0x1E"));

    // Uses same code as in Cancun, as no new opcode was introduced in Prague
    ToyAccount receiverAccountPrague = getReceiverAccount("0x111113", codePrague);

    ToyAccount receiverAccountOsaka = getReceiverAccount("0x111115", codeOsaka);

    ToyTransaction.ToyTransactionBuilder txBuilderPrague =
        ToyTransaction.builder().to(receiverAccountPrague).keyPair(keyPair);

    ToyTransaction.ToyTransactionBuilder txBuilderOsaka =
        ToyTransaction.builder().to(receiverAccountOsaka).keyPair(keyPair);

    // create transactions with the same sender, manages nonce
    final List<Transaction> transactions =
        ToyMultiTransaction.builder()
            .build(List.of(txBuilderPrague, txBuilderOsaka), senderAccount);

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccountPrague, receiverAccountOsaka))
        .transactions(transactions)
        .runWithBesuNode(true)
        .oneTxPerBlockOnBesuNode(true)
        .customBesuNodeGenesis(
            "BesuExecutionToolsGenesis_PragueToOsaka.json") /* Block 0 has totalDifficulty at 1, so TTD is set to 1 in genesis to have Block 1 on Paris fork */
        .build()
        .run();
  }

  private ToyAccount getReceiverAccount(String address, Bytes code) {
    return ToyAccount.builder()
        .balance(Wei.ONE)
        .nonce(6)
        .address(Address.fromHexString(address))
        .code(code)
        .build();
  }
}
