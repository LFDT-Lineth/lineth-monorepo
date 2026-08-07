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
import static lineth.zktracer.module.hub.signals.TracedException.STACK_OVERFLOW;
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
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class StackOverflowExceptionTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("stackOverflowExceptionSource")
  void stackOverflowExceptionTest(OpCode opCode, int alpha, int delta, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    for (int i = 0; i < 1024; i++) {
      program.push(0);
    }
    program.op(opCode);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // the opcode pushes more arguments than the stack can handle

    assertEquals(
        STACK_OVERFLOW,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  static Stream<Arguments> stackOverflowExceptionSource() {
    List<Arguments> arguments = new ArrayList<>();

    for (OpCodeData opCodeData : opcodes.iterator()) {
      if (opCodeData != null) {
        OpCode opCode = opCodeData.mnemonic();
        int alpha = opCodeData.stackSettings().alpha(); // number of items pushed onto the stack
        int delta = opCodeData.stackSettings().delta(); // number of items popped from the stack
        if (alpha > delta) {
          arguments.add(Arguments.of(opCode, alpha, delta));
        }
      }
    }
    return randomSampleByCurrentCommitHash(arguments).stream();
  }
}
