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

import static net.consensys.linea.zktracer.utilities.Utils.*;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class SeveralAccountCreationTests extends TracerTestBase {

  private static final Bytes INIT_CODE =
      BytecodeCompiler.newProgram(chainConfig)
          .push(Bytes.fromHexString("0x703eda7a")) // value
          .push(Bytes32.repeat((byte) 12)) // key
          .op(OpCode.SSTORE)
          .push(0)
          .push(0)
          .op(OpCode.RETURN)
          .compile();

  private static final Bytes CREATE2_INITCODE_IS_CALLDATA =
      BytecodeCompiler.newProgram(chainConfig)
          .immediate(POPULATE_MEMORY)
          .push(Bytes32.leftPad(Bytes.fromHexString("0x7a17"))) // salt
          .op(OpCode.MSIZE) // size
          .push(0) // offset
          .push(33) // value
          .op(OpCode.CREATE2)
          .compile();

  private static final Bytes CREATE2_INITCODE_IS_CALLDATA_AND_REVERT =
      Bytes.concatenate(CREATE2_INITCODE_IS_CALLDATA, REVERT);

  private final ToyAccount create2Account =
      ToyAccount.builder()
          .address(Address.fromHexString("0x2222222222222222222222222222222222222222"))
          .code(CREATE2_INITCODE_IS_CALLDATA)
          .balance(Wei.fromEth(1))
          .build();

  private final ToyAccount create2AndRevertAccount =
      ToyAccount.builder()
          .address(Address.fromHexString("0x1111111111111111111111111111111111111111"))
          .code(CREATE2_INITCODE_IS_CALLDATA_AND_REVERT)
          .balance(Wei.fromEth(1))
          .build();

  @Test
  void multiTransactionsCreate2(TestInfo testInfo) {

    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.of(100000000)).address(senderAddress).build();

    final Transaction tx1 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .payload(INIT_CODE)
            .to(create2AndRevertAccount)
            .build();

    final Transaction tx2 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .payload(INIT_CODE)
            .to(create2Account)
            .nonce(senderAccount.getNonce() + 1)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, create2Account, create2AndRevertAccount))
        .transactions(List.of(tx1, tx2))
        .build()
        .run();
  }

  @Test
  void monoTransactionsCreate2(TestInfo testInfo) {

    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.of(100000000)).address(senderAddress).build();

    final ToyAccount recipientAccout =
        ToyAccount.builder()
            .code(
                Bytes.concatenate(
                    POPULATE_MEMORY,
                    call(create2AndRevertAccount.getAddress(), false),
                    call(create2Account.getAddress(), false),
                    REVERT))
            .balance(Wei.fromEth(2))
            .address(Address.fromHexString("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"))
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .payload(INIT_CODE)
            .to(recipientAccout)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccout, create2Account, create2AndRevertAccount))
        .transaction(tx)
        .build()
        .run();
  }
}
