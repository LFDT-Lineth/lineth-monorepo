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

import static net.consensys.linea.zktracer.module.hub.signals.TracedException.OUT_OF_SSTORE;
import static org.junit.jupiter.api.Assertions.assertEquals;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

@ExtendWith(UnitTestWatcher.class)
public class OutOfSStoreExceptionTest extends TracerTestBase {

  @ParameterizedTest
  @ValueSource(
      longs = {
        0,
        Trace.GAS_CONST_G_CALL_STIPEND - 1,
        Trace.GAS_CONST_G_CALL_STIPEND,
        Trace.GAS_CONST_G_CALL_STIPEND + 1
      })
  void outOfSStoreExceptionTest(long remainingGasAfterPushes, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program.push(0).push(0).op(OpCode.SSTORE);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(21000L + 3L + 3L + remainingGasAfterPushes, chainConfig, testInfo);
    // 21000L is the intrinsic gas cost of a transaction and 3L is the gas cost of PUSH1

    if (remainingGasAfterPushes <= Trace.GAS_CONST_G_CALL_STIPEND) {
      assertEquals(
          OUT_OF_SSTORE,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    }
  }
}
