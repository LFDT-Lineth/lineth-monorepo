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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecrecover;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.*;

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
 * For these tests to work as expected, the transaction should start out with sufficient gas. At
 * least 21k plus gas to cover the PUSH's and the (warm) CALL with potentially value transfer and
 * potentially account creation costs. Also there should be enough gas in the end for the 63/64
 * business not diminish the gas we provide the callee.
 *
 * <p>Something like 60k gas should cover all costs (21k transaction costs + 9k for potential value
 * transfer + 25k if value transfer leads to a precompile starting to exist in the state etc ... +
 * 3k for the callee + opcode costs on the order of 130 or so)
 */
@ExtendWith(UnitTestWatcher.class)
public class GasStipendTests extends TracerTestBase {

  // sufficient gas for PRC execution
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void zeroValueEcRecoverCallTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    validEcrecoverData(program);
    appendCall(
        program,
        callOpCode,
        3000,
        Address.fromHexString(Address.ECREC.toHexString()),
        0,
        0,
        0,
        0,
        0);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void nonzeroValueEcRecoverCallTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    validEcrecoverData(program);
    appendCall(
        program,
        callOpCode,
        3000,
        Address.fromHexString(Address.ECREC.toHexString()),
        1,
        0,
        0,
        0,
        0);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void nonzeroValueEcRecoverCallWillRevertTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    validEcrecoverData(program);
    appendCall(
        program,
        callOpCode,
        3000,
        Address.fromHexString(Address.ECREC.toHexString()),
        1,
        0,
        0,
        0,
        32);
    appendRevert(program, 0, 32);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE"})
  void stipendCompletesGasEcRecoverCallTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    validEcrecoverData(program);
    appendCall(
        program,
        callOpCode,
        700,
        Address.fromHexString(Address.ECREC.toHexString()),
        1,
        0,
        0,
        0,
        0);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  // insufficient gas for PRC execution
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void gasFallsShortForEcRecoverTest(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    validEcrecoverData(program);
    appendCall(
        program,
        callOpCode,
        2999,
        Address.fromHexString(Address.ECREC.toHexString()),
        0,
        0,
        0,
        0,
        0);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE"})
  void stipendFromValueFallsShortOfCompletingGasEcrecoverCallTest(
      OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    validEcrecoverData(program);
    appendCall(
        program,
        callOpCode,
        699, // value transfer adds G_stipend = 2_300 to this, falling short of the 3_000 required
        // gas
        Address.fromHexString(Address.ECREC.toHexString()),
        1,
        0,
        0,
        0,
        0);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }
}
