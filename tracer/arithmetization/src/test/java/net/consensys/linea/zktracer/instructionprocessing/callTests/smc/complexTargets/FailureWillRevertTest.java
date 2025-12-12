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
package net.consensys.linea.zktracer.instructionprocessing.callTests.smc.complexTargets;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendRevert;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

@ExtendWith(UnitTestWatcher.class)
public class FailureWillRevertTest extends TracerTestBase {

  /**
   * The following uses a smart contract that calls itself but stops after one iteration. At which
   * point it reverts and its caller reverts, too. The test modifies storage of the underlying
   * account at both depths. These changes thus get reverted.
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  public void singleSelfCallFailureWillRevertTest(OpCode callOpCode, TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    sloadFrom(program, 0x00);
    addX(program, 0x01);
    duplicateTop(program);
    sstoreAt(program, 0x00);
    reduceMod2(program);
    negateBit(program);

    int currentSize = program.compile().size();

    jumpToXIfTopIsZero(program, currentSize + 4 + 5); // + 4
    // top is nonzero execution path
    appendRevert(program, 13, 9); // + 5
    // top is zero execution path
    prepareLanding(program);
    selfCall(program, callOpCode, 0xffff, 0x10);
    appendRevert(program, 31, 7);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  @Test
  public void banalAdditionTest(TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    sloadFrom(program, 0x00); // puts 0 on stack
    addX(program, 0x01);
    sstoreAt(program, 0x00);
    sloadFrom(program, 0x00); // puts 1 on stack
    addX(program, 0x01);
    sstoreAt(program, 0x00);
    appendRevert(program, 31, 7);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  /**
   * This test was written in the debugging of stack-consistency. It showed us that the "top" and
   * "bottom" elements involved in a <b>SWAPX</b> operation were mixed up in the implementation (of
   * the swap stack pattern.)
   */
  @Test
  public void banalSwapTest(TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program.push(1).push(2).op(SWAP1).push(3).op(SWAP1);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  /**
   * Similar to {@link #singleSelfCallFailureWillRevertTest(OpCode,TestInfo)} but with two self
   * calls.
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  public void thirdSelfCallBreaksTriggeringFailureWillRevertTest(
      OpCode callOpCode, TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    sloadFrom(program, 0x00);
    addX(program, 0x01);
    duplicateTop(program);
    sstoreAt(program, 0x00);
    compareToX(program, 3);

    int currentSize = program.compile().size();

    jumpToXIfTopIsZero(program, currentSize + 4 + 5); // + 4
    // top is nonzero execution path
    appendRevert(program, 13, 9); // + 5
    // top is zero execution path
    prepareLanding(program);
    selfCall(program, callOpCode, 0xffff, 0x01);
    appendRevert(program, 31, 7);

    BytecodeRunner.of(program).run(chainConfig, testInfo);
  }

  public void sloadFrom(BytecodeCompiler program, int storageKey) {
    program.push(storageKey).op(SLOAD);
  }

  public void addX(BytecodeCompiler program, int x) {
    program.push(x).op(ADD);
  }

  public void duplicateTop(BytecodeCompiler program) {
    program.op(DUP1);
  }

  public void sstoreAt(BytecodeCompiler program, int storageKey) {
    program.push(storageKey).op(SSTORE);
  }

  public void reduceModX(BytecodeCompiler program, int x) {
    program.push(x).op(SWAP1).op(MOD);
  }

  public void reduceMod2(BytecodeCompiler program) {
    reduceModX(program, 0x02);
  }

  public void jumpToXIfTopIsZero(BytecodeCompiler program, int x) {
    program.op(ISZERO).push(x).op(JUMPI);
  }

  public void prepareLanding(BytecodeCompiler program) {
    program.op(JUMPDEST);
  }

  public void selfCall(BytecodeCompiler program, OpCode callOpCode, int gas, int value) {
    program.push(0x20).push(0x33).push(0x46).push(0x59); // just some random bullshit
    if (opcodes.of(callOpCode).callHasValueArgument()) {
      program.push(value);
    }
    program.op(ADDRESS).push(gas).op(callOpCode);
  }

  /**
   * This method assumes that the top of the stack is a boolean value and negates it.
   *
   * @param program
   */
  public void negateBit(BytecodeCompiler program) {
    program.push(1).op(XOR);
  }

  public void compareToX(BytecodeCompiler program, int x) {
    program.push(x).op(EQ);
  }
}
