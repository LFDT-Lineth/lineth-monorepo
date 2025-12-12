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

/**
 * The arithmetization has a two aborting scenarios for CALL's
 *
 * <p>- <b>scn/CALL_ABORT_WILL_REVERT</b>
 *
 * <p>- <b>scn/CALL_ABORT_WONT_REVERT</b> The main point being: (unexceptional) aborted CALL's warm
 * up the target account.
 */
@ExtendWith(UnitTestWatcher.class)
public class BalanceAbortTests extends TracerTestBase {

  final String eoaAddress = "abcdef0123456789";

  /**
   * This test has different behaviour for <b>CALL</b> and <b>CALLCODE</b> vs the other CALL-type
   * instructions. The point being: these two can transfer value, the others can't. As such they are
   * the only instructions that can trigger the desired <b>INSUFFICIENT_BALANCE_ABORT</b>.
   *
   * <p>This test should trigger <b>scenario/CALL_ABORT_WONT_REVERT</b> for both <b>CALL</b> and
   * <b>CALLCODE</b>.
   *
   * @param callOpCode
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE"})
  void insufficientBalanceAbortWarmsUpTarget(OpCode callOpCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendInsufficientBalanceCall(
        program, callOpCode, 1000, Address.fromHexString(eoaAddress), 4, 3, 2, 1);
    program
        .op(POP)
        .push(eoaAddress) // address
        .op(EXTCODESIZE) // discounted pricing since warm
        .compile();

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  /**
   * The same comments apply as for {@link #insufficientBalanceAbortWarmsUpTarget(OpCode,TestInfo)}.
   * In this test we further impose a REVERT which will affect the warmth of the target address.
   *
   * <p>This test should trigger <b>scenario/CALL_ABORT_WILL_REVERT</b> for both <b>CALL</b> and
   * <b>CALLCODE</b>.
   *
   * @param callOpCode
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE"})
  void insufficientBalanceAbortWillRevert(OpCode callOpCode, TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    appendInsufficientBalanceCall(
        program, callOpCode, 1000, Address.fromHexString(eoaAddress), 0, 0, 0, 0);
    program.push(6).push(7).op(REVERT);
    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }
}
