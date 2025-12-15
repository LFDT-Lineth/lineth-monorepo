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
package net.consensys.linea.zktracer.instructionprocessing.callTests.smc.monoOpCodeTargets;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/**
 * Simplest case where we enter a smart contract. The called smart contract executes a single STOP
 * opcode (which is free of charge). No value is transferred.
 */
@ExtendWith(UnitTestWatcher.class)
public class singleStop extends TracerTestBase {

  /** This test should trigger the <b>scenario/CALL_TO_SMC_SUCCESS_WONT_REVERT</b> scenario. */
  @Test
  void zeroValueTransferToContractThatStops(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendCall(program, CALL, 0, accountWhoseByteCodeIsASingleStop.getAddress(), 0, 0, 0, 0, 0);

    BytecodeRunner.of(program.compile()).run(accounts, chainConfig, testInfo);
  }

  /** This test should trigger the <b>scenario/CALL_TO_SMC_SUCCESS_WONT_REVERT</b> scenario. */
  @Test
  void nonZeroValueTransferToContractThatStops(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendCall(program, CALL, 0, accountWhoseByteCodeIsASingleStop.getAddress(), 1, 0, 0, 0, 0);

    BytecodeRunner.of(program.compile()).run(accounts, chainConfig, testInfo);
  }

  /** This test should trigger the <b>scenario/CALL_TO_SMC_SUCCESS_WILL_REVERT</b> scenario. */
  @Test
  void nonZeroValueTransferToContractThatStopsRevertingTransaction(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendCall(program, CALL, 0, accountWhoseByteCodeIsASingleStop.getAddress(), 1, 0, 0, 0, 0);

    // we use the 1 on the stack after this successful CALL as the revert message size
    program.push(0).op(REVERT);

    BytecodeRunner.of(program.compile()).run(accounts, chainConfig, testInfo);
  }
}
