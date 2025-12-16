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

package net.consensys.linea.zktracer.exceptions;

import static net.consensys.linea.zktracer.module.hub.signals.TracedException.STATIC_FAULT;
import static net.consensys.linea.zktracer.opcode.OpCode.GAS;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

@ExtendWith(UnitTestWatcher.class)
public class StaticExceptionTest extends TracerTestBase {

  @ParameterizedTest
  @ValueSource(ints = {0, 1})
  void staticExceptionDueToCallWithNonZeroValueTest(int value, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push("ca11ee") // address
        .op(GAS)
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    calleeProgram
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push(value) // value
        .push(Address.ZERO) // address
        .op(GAS)
        .op(OpCode.CALL);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program);

    bytecodeRunner.run(List.of(calleeAccount), chainConfig, testInfo);

    if (value != 0) {
      assertEquals(
          STATIC_FAULT,
          bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
    }
  }

  @Test
  void staticExceptionDueToSStoreTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push("ca11ee") // address
        .push(1000) // gas
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    calleeProgram.push(0).push(0).op(OpCode.SSTORE);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());

    bytecodeRunner.run(List.of(calleeAccount), chainConfig, testInfo);

    assertEquals(
        STATIC_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
  }

  @Test
  void staticExceptionDueToSelfDestructTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push("ca11ee") // address
        .push(1000) // gas
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    calleeProgram.push(0).op(OpCode.SELFDESTRUCT);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());

    bytecodeRunner.run(List.of(calleeAccount), chainConfig, testInfo);

    assertEquals(
        STATIC_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
  }

  @ParameterizedTest
  @ValueSource(ints = {0, 1, 2, 3, 4})
  void staticExceptionDueToLogTest(int numberOfTopics, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push("ca11ee") // address
        .push(1000) // gas
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    for (int i = 0; i < numberOfTopics; i++) {
      calleeProgram.push(0);
    }
    // Construct appropriate LOG bytecode for the given number of topics.
    OpCode opCode = opcodes.of(OpCode.LOG0.getOpcode() + numberOfTopics).mnemonic();
    calleeProgram.push(0).push(0).op(opCode);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());

    bytecodeRunner.run(List.of(calleeAccount), chainConfig, testInfo);

    assertEquals(
        STATIC_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
  }

  @ParameterizedTest
  @ValueSource(strings = {"CREATE", "CREATE2"})
  void staticExceptionDueToCreateTest(String opCodeName, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push("ca11ee") // address
        .push(1000) // gas
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    if (opCodeName.equals("CREATE2")) {
      calleeProgram.push(0);
    }
    calleeProgram
        .push(0)
        .push(0)
        .push(0)
        .op(opCodeName.equals("CREATE") ? OpCode.CREATE : OpCode.CREATE2);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());

    bytecodeRunner.run(List.of(calleeAccount), chainConfig, testInfo);

    assertEquals(
        STATIC_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
  }
}
