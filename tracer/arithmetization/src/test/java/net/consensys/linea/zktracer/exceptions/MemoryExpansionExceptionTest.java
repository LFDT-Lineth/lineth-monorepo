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

import static net.consensys.linea.zktracer.module.hub.signals.TracedException.MEMORY_EXPANSION_EXCEPTION;
import static net.consensys.linea.zktracer.module.mxp.MxpTestUtils.opCodesType2;
import static net.consensys.linea.zktracer.module.mxp.MxpTestUtils.opCodesType3;
import static net.consensys.linea.zktracer.module.mxp.MxpTestUtils.opCodesType4;
import static net.consensys.linea.zktracer.module.mxp.MxpTestUtils.opCodesType5;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.mxp.MxpTestUtils;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class MemoryExpansionExceptionTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("memoryExpansionExceptionTestSource")
  public void memoryExpansionExceptionTest(boolean triggerRoob, OpCode opCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    boolean triggerMaxCodeSizeException = false;
    new MxpTestUtils(opcodes)
        .triggerNonTrivialButMxpxOrRoobOrMaxCodeSizeExceptionForOpCode(
            fork, program, triggerRoob, triggerMaxCodeSizeException, opCode);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    assertEquals(
        MEMORY_EXPANSION_EXCEPTION,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());

    assertTrue(bytecodeRunner.getHub().mxp().operations().getLast().getMxpCall().isMxpx());
  }

  @Test
  public void soloMemoryExpansionTest(TestInfo testInfo) {
    boolean triggerRoob = false;
    boolean triggerMaxCodeSizeException = false;
    OpCode opCode = OpCode.CODECOPY;
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    new MxpTestUtils(opcodes)
        .triggerNonTrivialButMxpxOrRoobOrMaxCodeSizeExceptionForOpCode(
            fork, program, triggerRoob, triggerMaxCodeSizeException, opCode);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    assertEquals(
        MEMORY_EXPANSION_EXCEPTION,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());

    assertTrue(bytecodeRunner.getHub().mxp().operations().getLast().getMxpCall().isMxpx());
  }

  private static Stream<Arguments> memoryExpansionExceptionTestSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (OpCode opCode : opCodesType2) {
      arguments.add(Arguments.of(false, opCode));
      arguments.add(Arguments.of(true, opCode));
    }
    for (OpCode opCode : opCodesType3) {
      arguments.add(Arguments.of(false, opCode));
      arguments.add(Arguments.of(true, opCode));
    }
    for (OpCode opCode : opCodesType4) {
      arguments.add(Arguments.of(false, opCode));
      arguments.add(Arguments.of(true, opCode));
    }
    for (OpCode opCode : opCodesType5) {
      arguments.add(Arguments.of(false, opCode));
      arguments.add(Arguments.of(true, opCode));
    }
    return arguments.stream();
  }
}
