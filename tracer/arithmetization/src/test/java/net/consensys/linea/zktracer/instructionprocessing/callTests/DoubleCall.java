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
package net.consensys.linea.zktracer.instructionprocessing.callTests;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

@ExtendWith(UnitTestWatcher.class)
public class DoubleCall extends TracerTestBase {

  /** Same selfDestructorAddress */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void doubleCallToSameAddressWontRevert(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress), 1, 0, 0, 0, 0);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress), 2, 0, 0, 0, 0);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void doubleCallToSameAddressWillRevert(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress), 1, 0, 0, 0, 0);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress), 2, 0, 0, 0, 0);
    program.op(REVERT); // N.B. The stack contains the two success bits

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  /** Different selfDestructorAddress */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void doubleCallTodifferentAddressesWontRevert(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress), 1, 0, 0, 0, 0);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress2), 2, 0, 0, 0, 0);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void doubleCallTodifferentAddressesWillRevert(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress), 1, 0, 0, 0, 0);
    appendCall(program, callOpCode, 0, Address.fromHexString(eoaAddress2), 2, 0, 0, 0, 0);
    program.push(13).push(71); // the stack already contains two items but why not ...
    program.op(REVERT);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }
}
