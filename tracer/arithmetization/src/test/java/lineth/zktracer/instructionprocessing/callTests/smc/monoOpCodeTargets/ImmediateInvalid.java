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
package lineth.zktracer.instructionprocessing.callTests.smc.monoOpCodeTargets;

import static lineth.zktracer.instructionprocessing.utilities.Calls.appendCall;
import static lineth.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.*;
import static lineth.zktracer.opcode.OpCode.CALL;
import static lineth.zktracer.opcode.OpCode.REVERT;

import lineth.UnitTestWatcher;
import lineth.reporting.TracerTestBase;
import lineth.testing.BytecodeCompiler;
import lineth.testing.BytecodeRunner;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class ImmediateInvalid extends TracerTestBase {

  @Test
  void zeroValueTransferToInvalid(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendCall(program, CALL, 0, accountWhoseByteCodeIsASingleInvalid.getAddress(), 0, 0, 0, 0, 0);

    BytecodeRunner.of(program.compile()).run(accounts, chainConfig, testInfo);
  }

  @Test
  void nonZeroValueTransferToInvalidContract(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendCall(program, CALL, 0, accountWhoseByteCodeIsASingleInvalid.getAddress(), 1, 0, 0, 0, 0);

    BytecodeRunner.of(program.compile()).run(accounts, chainConfig, testInfo);
  }

  @Test
  void nonZeroValueTransferToInvalidContractRevertingTransaction(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendCall(program, CALL, 0, accountWhoseByteCodeIsASingleInvalid.getAddress(), 1, 0, 0, 0, 0);

    // we use the 1 on the stack after this successful CALL as the revert message size
    program.push(0).op(REVERT);

    BytecodeRunner.of(program.compile()).run(accounts, chainConfig, testInfo);
  }
}
