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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.hash;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendRevert;
import static net.consensys.linea.zktracer.opcode.OpCode.RETURNDATASIZE;

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

/**
 * All of the tests below have in common that they attempt to hash 0x00...00 ∈ B_321 with SHA256.
 * The test vectors vary accoring to
 *
 * <p>- the kind of CALL instruction used
 *
 * <p>- whether the operation is reverted or not (they all transfer value)
 */
@ExtendWith(UnitTestWatcher.class)
public class GasTests extends TracerTestBase {

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void sha2ProvidedWithLittleToNoneGasTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 0, Address.SHA256, 1_000_000, 0, 32 * 10 + 1, 7, 32);
    program.op(RETURNDATASIZE); // should return 0

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void sha2ProvidedWithLittleToNoneGasWillRevertTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 0, Address.SHA256, 1_000_000, 0, 32 * 10, 7, 32);
    program.op(RETURNDATASIZE); // should return 0
    appendRevert(program, 1, 34);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void sha2ProvidedWithPlentifulGasTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 1_000_000, Address.SHA256, 1_000_000, 0, 32 * 10, 7, 32);
    program.op(RETURNDATASIZE); // should return 32
    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void sha2ProvidedWithPlentifulGasWillRevertTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendCall(program, callOpCode, 1_000_000, Address.SHA256, 1_000_000, 0, 32 * 10, 7, 32);
    program.op(RETURNDATASIZE); // should return 32
    appendRevert(program, 1, 34);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }
}
