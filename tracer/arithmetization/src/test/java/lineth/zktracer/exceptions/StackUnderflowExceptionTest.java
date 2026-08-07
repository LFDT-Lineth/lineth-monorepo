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
package lineth.zktracer.exceptions;

import static lineth.zktracer.instructionprocessing.callTests.Utilities.randomSampleByCurrentCommitHash;
import static lineth.zktracer.module.hub.signals.TracedException.STACK_UNDERFLOW;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import lineth.UnitTestWatcher;
import lineth.reporting.TracerTestBase;
import lineth.testing.BytecodeCompiler;
import lineth.testing.BytecodeRunner;
import lineth.zktracer.opcode.OpCode;
import lineth.zktracer.opcode.OpCodeData;
import lineth.zktracer.opcode.OpCodes;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class StackUnderflowExceptionTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("stackUnderflowExceptionSource")
  void stackUnderflowExceptionTest(
      OpCode opCode, int nPushes, boolean triggersStackUnderflowExceptions, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    for (int i = 0; i < nPushes; i++) {
      program.push(0);
    }
    program.op(opCode);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // the number of pushed arguments is less than the number of arguments required by the opcode

    if (triggersStackUnderflowExceptions) {
      assertEquals(
          STACK_UNDERFLOW,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    }
  }

  static Stream<Arguments> stackUnderflowExceptionSource() {
    List<Arguments> arguments = new ArrayList<>();
    OpCodes opcodes = OpCodes.load(fork);
    //
    for (OpCodeData opCodeData : opcodes.iterator()) {
      if (opCodeData != null) {
        OpCode opCode = opCodeData.mnemonic();
        int delta = opCodeData.stackSettings().delta(); // number of items popped from the stack
        for (int nPushes = 0; nPushes <= delta; nPushes++) {
          arguments.add(Arguments.of(opCode, nPushes, nPushes < delta));
        }
      }
    }
    return randomSampleByCurrentCommitHash(arguments).stream();
  }
}
