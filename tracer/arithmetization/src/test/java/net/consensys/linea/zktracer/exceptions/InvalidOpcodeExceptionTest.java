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

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.hub.signals.TracedException;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class InvalidOpcodeExceptionTest extends TracerTestBase {

  @Test
  void invalidOpcodeExceptionTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program.op(OpCode.INVALID);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    assertEquals(
        TracedException.INVALID_OPCODE,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @ParameterizedTest
  @MethodSource("nonOpcodeExceptionSource")
  void nonOpcodeExceptionTest(int value, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program.immediate(value);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    assertEquals(
        TracedException.INVALID_OPCODE,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  static Stream<Arguments> nonOpcodeExceptionSource() {
    List<Arguments> arguments = new ArrayList<>();
    //
    for (int value = 0; value < 256; value++) {
      // If value is not in the map, then it is not an OpCode
      if (!opcodes.isValid(value)) {
        arguments.add(Arguments.of(value));
      }
    }
    return arguments.stream();
  }
}
