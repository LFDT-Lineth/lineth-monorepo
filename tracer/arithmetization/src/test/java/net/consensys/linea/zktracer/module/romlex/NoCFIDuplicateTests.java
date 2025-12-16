/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.romlex;

import static com.google.common.base.Preconditions.checkArgument;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class NoCFIDuplicateTests extends TracerTestBase {

  /**
   * This test checks that the ROM_LEX doesn't create duplicate of CFI when calling twice the same
   * contract.
   */
  @Test
  void noCfiDuplicate(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig).push(256).push(255).op(OpCode.SLT).compile();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(0xffff))
            .nonce(128)
            .address(Address.fromHexString("0x1234567890"))
            .code(bytecode)
            .build();

    final Transaction tx1 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .to(recipientAccount)
            .build();

    final Transaction tx2 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .to(recipientAccount)
            .nonce(senderAccount.getNonce() + 1)
            .build();

    final ToyExecutionEnvironmentV2 test =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, recipientAccount))
            .transactions(List.of(tx1, tx2))
            .zkTracerValidator(zkTracer -> {})
            .build();

    test.run();

    checkArgument(test.getHub().romLex().lineCount() == 1);
  }
}
