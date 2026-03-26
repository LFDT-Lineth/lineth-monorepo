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

package net.consensys.linea.zktracer.module.limits;

import static net.consensys.linea.zktracer.module.limits.Keccak.numberOfKeccakBloc;
import static org.junit.jupiter.api.Assertions.assertEquals;

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
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class KeccakBlocksTests extends TracerTestBase {

  @Test
  void keccakCountTests(TestInfo testInfo) {

    // sender account
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(12).address(senderAddress).build();

    // receiver account
    final ToyAccount recipient =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(Address.wrap(Bytes.repeat((byte) 1, Address.SIZE)))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // CREATE
                    .push(0) // size
                    .push(0) // offset
                    .push(0) // value
                    .op(OpCode.CREATE)
                    // CREATE 2
                    .push(0) // salt
                    .push(0) // size
                    .push(0) // offset
                    .push(0) // value
                    .op(OpCode.CREATE2)
                    .compile())
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipient)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .value(Wei.of(1000))
            .build();

    final ToyExecutionEnvironmentV2 toyWorld =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, recipient))
            .transaction(tx)
            .zkTracerValidator(zkTracer -> {})
            .build();

    toyWorld.run();

    final Keccak keccak = toyWorld.getHub().keccak();
    final L1BlockSize l1BlockSize = toyWorld.getHub().l1BlockSize();

    // check lineCount of Keccak
    final int txRlpSize = tx.encoded().size();
    final int txKeccak =
        2 * numberOfKeccakBloc(txRlpSize)
            + 1; // 2 * numberOfKeccakBloc(txRlpSize) + 1 for EcRecover
    final int rlpAddrKeccak = 1 + 1; // 1 for CREATE, 1 for CREATE2
    assertEquals(txKeccak + rlpAddrKeccak, keccak.lineCount());

    // check lineCount of l2Block
    assertEquals(
        txRlpSize
            // nbTransaction * Address.SIZE
            + Address.SIZE
            // nbBlock * (TIMESTAMP_BYTESIZE + Hash.SIZE + NB_TX_IN_BLOCK_BYTESIZE)
            + 38,
        l1BlockSize.lineCount());
  }
}
