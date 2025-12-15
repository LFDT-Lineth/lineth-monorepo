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
package net.consensys.linea.zktracer.instructionprocessing.callTests.eoa;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.fullBalanceCall;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/**
 * In the arithmetization there are the following EOA specific scenarios:
 *
 * <p>- <b>scn/CALL_EOA_SUCCESS_WILL_REVERT</b>
 *
 * <p>- <b>scn/CALL_EOA_SUCCESS_WONT_REVERT</b>
 */
@ExtendWith(UnitTestWatcher.class)
public class SmcCallsEoaInRoot extends TracerTestBase {

  final String eoaAddress = "abcdef0123456789";

  @Test
  void transfersSomeValueWillRevertTest(TestInfo testInfo) {
    BytecodeCompiler bytecode = BytecodeCompiler.newProgram(chainConfig);
    appendCall(bytecode, CALL, 0, Address.fromHexString(eoaAddress), 13, 2, 3, 4, 5);
    bytecode.op(POP).push(6).push(7).op(REVERT).compile();
    BytecodeRunner.of(bytecode.compile()).run(chainConfig, testInfo);
  }

  @Test
  void transfersSomeValueWontRevertTest(TestInfo testInfo) {
    BytecodeCompiler bytecode = BytecodeCompiler.newProgram(chainConfig);
    appendCall(bytecode, CALL, 0, Address.fromHexString(eoaAddress), 13, 2, 3, 4, 5);
    BytecodeRunner.of(bytecode.compile()).run(chainConfig, testInfo);
  }

  @Test
  void transfersAllValueWillRevertTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    fullBalanceCall(program, CALL, Address.fromHexString(eoaAddress), 1, 2, 3, 4);
    program.push(6).push(7).op(REVERT);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @Test
  void transfersAllValueWontRevertTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    fullBalanceCall(program, CALL, Address.fromHexString(eoaAddress), 1, 2, 3, 4);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @Test
  void transfersNoValueWillRevertTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, CALL, 0, Address.fromHexString(eoaAddress), 0, 1, 2, 3, 4);
    program.op(POP).push(6).push(7).op(REVERT).compile();
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @Test
  void transfersNoValueWontRevertTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, CALL, 0, Address.fromHexString(eoaAddress), 0, 1, 2, 3, 4);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }
}
