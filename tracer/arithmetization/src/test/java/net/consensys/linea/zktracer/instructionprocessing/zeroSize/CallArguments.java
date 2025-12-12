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
package net.consensys.linea.zktracer.instructionprocessing.zeroSize;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

/**
 * In the {@link CallArguments} tests, we test the extremal cases of CALLs with respect to "call
 * data" and "return at ..." parameters. This follows up on the recent change in constraints where,
 * focusing on CALLs only, we set CDO to zero whenever CDS ≡ 0, and similarly for R@0 and R@C.
 */
@ExtendWith(UnitTestWatcher.class)
public class CallArguments extends TracerTestBase {

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void emptyCallDataAndReturnAtCall(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    extremalCallContract(program, callOpCode, true, true);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void emptyReturnAtCall(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    extremalCallContract(program, callOpCode, false, true);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void emptyCallDataCall(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    extremalCallContract(program, callOpCode, true, false);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  private void extremalCallContract(
      BytecodeCompiler program, OpCode callOpCode, boolean emptyCallData, boolean emptyReturnAt) {
    appendExtremalCall(
        program,
        callOpCode,
        300_000,
        ToyAccount.builder().address(Address.fromHexString(eoaAddress)).build(),
        2048,
        emptyCallData,
        emptyReturnAt);
  }
}
