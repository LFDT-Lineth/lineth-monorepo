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
package net.consensys.linea.zktracer.instructionprocessing.callTests.abort;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendRecursiveSelfCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendRevert;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

/**
 * Attempt to trigger the maximum call stack depth abort. We put everything to 0 to avoid memory
 * expansion costs. We will want to revert so we transfer value to see the effect of reverting.
 */
@ExtendWith(UnitTestWatcher.class)
public class CallStackDepthAbortTests extends TracerTestBase {
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void attemptAtCallStackDepthAbortWillRevert(OpCode callOpCode, TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendRecursiveSelfCall(program, callOpCode);
    appendRevert(program, 6, 7);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void attemptAtCallStackDepthAbort(OpCode callOpCode, TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendRecursiveSelfCall(program, callOpCode);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }
}
