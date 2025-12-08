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
package net.consensys.linea.zktracer.exceptions.multiExceptions;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for Jump family opcodes.
List of the combinations tested below
JUMP & OOGX : JUMP, JUMPI
 */

import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_TRANSACTION;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_VERY_LOW;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.OUT_OF_GAS_EXCEPTION;
import static org.junit.jupiter.api.Assertions.assertEquals;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for CALL opcode.
List of the combinations tested below
JUMPX & OOGX : JUMP, JUMPI
    - triggering JUMPX by jumping outside of the bytecode
    - triggering JUMPX by jumping within the bytecode but not landing on a valid jump destination
 */
@ExtendWith(UnitTestWatcher.class)
public class JumpTest extends TracerTestBase {
  /**
   * Trigger a jump exception and an out of gas exception. Jump exception can be triggered by a jump
   * to an invalid destination (here 5) or outside of codesize (here 6)
   */
  @ParameterizedTest
  @ValueSource(ints = {5, 6})
  void jumpAndOogExceptionsJump(int jumpCounter, TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(jumpCounter) // pc: 0 - 5 i/o 4, Trigger Jump Exception
            .op(OpCode.JUMP) // pc: 2
            .op(OpCode.INVALID) // pc: 3
            .op(OpCode.JUMPDEST) // pc: 4
            .push(OpCode.JUMPDEST.byteValue()) // pc: 5
            .compile();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    // JUMP needs JUMPDEST to jump to
    // Calculate the gas cost to trigger OOGX on JUMP and not on the last but one opcode
    long gasCost = GAS_CONST_G_TRANSACTION + GAS_CONST_G_VERY_LOW;

    bytecodeRunner.run(gasCost, chainConfig, testInfo);

    // OOGX check happens before JUMPX in tracer
    assertEquals(
        OUT_OF_GAS_EXCEPTION,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  /**
   * Trigger a jump exception and an out of gas exception. Jump exception can be triggered by a
   * jumpi to an invalid destination (here 6) or outside of codesize (here 9)
   */
  @ParameterizedTest
  @ValueSource(ints = {6, 9})
  void jumpAndOogExceptionsJumpi(int jumpCounter, TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(1) // pc = 0, 1
            .push(jumpCounter) // pc = 2, 3, i/o 7, Trigger Jump Exception
            .op(OpCode.JUMPI) // pc = 4
            .op(OpCode.JUMPDEST) // pc = 5
            .op(OpCode.INVALID) // pc = 6
            .op(OpCode.JUMPDEST) // pc = 7
            .push(1) // pc = 8
            .compile();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);

    // JUMPI needs JUMPDEST to jump to
    // Calculate the gas cost to trigger OOGX on JUMPI and not on the last but one opcode
    long gasCost = GAS_CONST_G_TRANSACTION + 2 * GAS_CONST_G_VERY_LOW;

    bytecodeRunner.run(gasCost, chainConfig, testInfo);

    // JUMPX check happens before OOGX in tracer
    assertEquals(
        OUT_OF_GAS_EXCEPTION,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }
}
