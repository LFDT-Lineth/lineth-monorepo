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

package net.consensys.linea.zktracer.forkSpecific.cancun;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class TransientTest extends TracerTestBase {

  private static final Bytes TLOAD_TSTORE_TLOAD = Bytes.fromHexString("0x60025C50600160025D60025C");
  // This bytecode is:
  // BytecodeCompiler.newProgram(chainConfig)
  // .push(2) // storage key
  //     .op(TLOAD)
  //     .op(POP) // value
  //     .push(1) // value
  //     .push(2) // storage key
  //     .op(TSTORE)
  //     .push(2) // storage key
  //     .op(TLOAD)
  //     .compile()

  private static final ToyAccount SMC_ACCOUNT_TLOAD_TSTORE_TLOAD =
      ToyAccount.builder()
          .address(Address.wrap(Bytes.fromHexString("0x73A71E0073A71E0073A71E0073A71E0073A71E00")))
          .code(TLOAD_TSTORE_TLOAD)
          .balance(Wei.fromEth(2))
          .build();

  private static final ToyAccount SMC_ACCOUNT_TLOAD_TSTORE_TLOAD_REVERT =
      ToyAccount.builder()
          .address(Address.wrap(Bytes.fromHexString("0x73A71E0073A71E0073A71E0073A71E0000000000")))
          .code(
              Bytes.concatenate(
                  TLOAD_TSTORE_TLOAD, Bytes.fromHexString("0x60006000FD"))) // Push 0 Push 0 Revert
          .balance(Wei.fromEth(2))
          .build();

  private static final Bytes PREPARESTACK =
      Bytes.concatenate(
          Bytes.fromHexString("0x600060006000600073"),
          SMC_ACCOUNT_TLOAD_TSTORE_TLOAD.getAddress().getBytes(),
          Bytes.fromHexString("613A98"));
  // This bytecode is:
  // BytecodeCompiler.newProgram(chainConfig)
  //     .push(0) // return size
  //     .push(0) // return offset
  //     .push(0) // arg size
  //     .push(0) // arg offset
  //     .push(SMC_ACCOUNT_TLOAD_TSTORE_TLOAD.getAddress()) // address
  //     .push(15000) // gas
  //     .compile();

  private static final Bytes STATIC_CALLER =
      Bytes.concatenate(PREPARESTACK, Bytes.fromHexString("0xFA"));

  private static final Bytes DELEGATE_CALLER =
      Bytes.concatenate(PREPARESTACK, Bytes.fromHexString("0xF4"));

  private static final Bytes CALL_CALLER =
      Bytes.concatenate(PREPARESTACK, Bytes.fromHexString("0xF1"));

  private static final Bytes CALLCODE_CALLER =
      Bytes.concatenate(PREPARESTACK, Bytes.fromHexString("0xF2"));

  public static Stream<Arguments> fourCalls() {
    final List<Arguments> arguments = new ArrayList<>();
    arguments.add(Arguments.of(CALL_CALLER));
    arguments.add(Arguments.of(CALLCODE_CALLER));
    arguments.add(Arguments.of(STATIC_CALLER));
    arguments.add(Arguments.of(DELEGATE_CALLER));
    return arguments.stream();
  }

  @Test
  void trivialTStoreTLoad(TestInfo testInfo) {
    BytecodeRunner.of(TLOAD_TSTORE_TLOAD).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @MethodSource("fourCalls")
  void differentCallsTStoreTLoad(Bytes callType, TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());

    final Address RECIPIENT_ADDRESS =
        Address.fromHexString("0x1122334455667788990011223344556677889900");

    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(5)).address(senderAddress).build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(5))
            .address(RECIPIENT_ADDRESS)
            .code(callType)
            .build();

    final Transaction transaction =
        ToyTransaction.builder()
            .sender(senderAccount)
            .gasLimit(150000L)
            .keyPair(senderKeyPair)
            .to(recipientAccount)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount, SMC_ACCOUNT_TLOAD_TSTORE_TLOAD))
        .transaction(transaction)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void multipleTransactionTStoreTLoad(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());

    final Address RECIPIENT_ADDRESS =
        Address.fromHexString("0x1122334455667788990011223344556677889900");

    final short firstNonce = 1;

    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(5))
            .address(senderAddress)
            .nonce(firstNonce)
            .build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(5))
            .address(RECIPIENT_ADDRESS)
            .code(TLOAD_TSTORE_TLOAD)
            .build();

    final Transaction transaction1 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .gasLimit(150000L)
            .keyPair(senderKeyPair)
            .to(recipientAccount)
            .nonce((long) firstNonce)
            .build();

    final Transaction transaction2 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .gasLimit(150000L)
            .keyPair(senderKeyPair)
            .to(recipientAccount)
            .nonce((long) firstNonce + 1)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount))
        .transactions(List.of(transaction1, transaction2))
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void revertingTStoreTLoad(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());

    final Address RECIPIENT_ADDRESS =
        Address.fromHexString("0x1122334455667788990011223344556677889900");

    final Bytes recipientCode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(0) // return size
            .push(0) // return offset
            .push(0) // arg size
            .push(0) // arg offset
            .push(SMC_ACCOUNT_TLOAD_TSTORE_TLOAD_REVERT.getAddress().getBytes()) // address
            .push(15000) // gas
            .op(OpCode.CALL)
            .push(0) // return size
            .push(0) // return offset
            .push(0) // arg size
            .push(0) // arg offset
            .push(SMC_ACCOUNT_TLOAD_TSTORE_TLOAD_REVERT.getAddress().getBytes()) // address
            .push(15000) // gas
            .op(OpCode.CALL)
            .compile();
    ;

    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(5)).address(senderAddress).build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(5))
            .address(RECIPIENT_ADDRESS)
            .code(recipientCode)
            .build();

    final Transaction transaction =
        ToyTransaction.builder()
            .sender(senderAccount)
            .gasLimit(150000L)
            .keyPair(senderKeyPair)
            .to(recipientAccount)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount, SMC_ACCOUNT_TLOAD_TSTORE_TLOAD_REVERT))
        .transaction(transaction)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }
}
