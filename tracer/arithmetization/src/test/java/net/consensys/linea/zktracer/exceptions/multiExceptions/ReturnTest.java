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

import static net.consensys.linea.zktracer.Fork.isPostShanghai;
import static net.consensys.linea.zktracer.Trace.EIP_3541_MARKER;
import static net.consensys.linea.zktracer.Trace.MAX_CODE_SIZE;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.getPgCreateInitCodeWithReturnStartByteAndSize;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.MAX_CODE_SIZE_EXCEPTION;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.OUT_OF_GAS_EXCEPTION;
import static org.junit.jupiter.api.Assertions.assertEquals;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for Logs family opcodes.
List of the combinations tested below
InvalidCodePrefix & OOGX : RETURN
InvalidCodePrefix & MaxCodeSize : RETURN
MaxCodeSize & OOGX : RETURN
InvalidCodePrefix & MaxCodeSize & OOGX : RETURN
 */

@ExtendWith(UnitTestWatcher.class)
public class ReturnTest extends TracerTestBase {
  @Test
  void invalidCodePrefixAndOogExceptionForCreate(TestInfo testInfo) {
    // We run gas cost calculation on program without Invalid Code Prefix exception
    int startByte = 0xee;
    BytecodeCompiler programWithoutICP =
        getPgCreateInitCodeWithReturnStartByteAndSize(startByte, 1);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(programWithoutICP.compile());
    long gascost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    // We prepare program with Invalid Code Prefix exception
    int startByteWithICPX = EIP_3541_MARKER;
    BytecodeCompiler programWithICP =
        getPgCreateInitCodeWithReturnStartByteAndSize(startByteWithICPX, 1);

    // We run program with Invalid Code Prefix and OOG exception
    long gasCostMinusOne = gascost - 2;
    BytecodeRunner bytecodeRunnerWithICP = BytecodeRunner.of(programWithICP.compile());
    bytecodeRunnerWithICP.run(gasCostMinusOne, chainConfig, testInfo);

    // OOGX check is done prior to Invalid Code Prefix exception in tracer
    assertEquals(
        OUT_OF_GAS_EXCEPTION,
        bytecodeRunnerWithICP
            .getHub()
            .lastUserTransactionSection(2)
            .commonValues
            .tracedException());
  }

  @Test
  void initCodePrefixAndMaxCodeSizeExceptionForCreate(TestInfo testInfo) {
    // We prepare program with Invalid Code Prefix and Max Code Size exceptions
    int startByteWithICPX = EIP_3541_MARKER;
    int returnSize = MAX_CODE_SIZE + 1;
    BytecodeCompiler programWithICPXAndMCSX =
        getPgCreateInitCodeWithReturnStartByteAndSize(startByteWithICPX, returnSize);

    BytecodeRunner bytecodeRunnerWithICPXAndMCSX =
        BytecodeRunner.of(programWithICPXAndMCSX.compile());
    bytecodeRunnerWithICPXAndMCSX.run(chainConfig, testInfo);

    // Max Code Size Exception check is done prior to Invalid Code Prefix exception in tracer
    assertEquals(
        MAX_CODE_SIZE_EXCEPTION,
        bytecodeRunnerWithICPXAndMCSX
            .getHub()
            .lastUserTransactionSection(2)
            .commonValues
            .tracedException());
  }

  @Test
  void maxCodeSizeAndOogExceptionForCreate(TestInfo testInfo) {
    BytecodeCompiler initProgram = BytecodeCompiler.newProgram(chainConfig);
    initProgram.push(MAX_CODE_SIZE + 1).push(0).op(OpCode.RETURN);
    final String initProgramAsString = initProgram.compile().toString().substring(2);
    final int initProgramByteSize = initProgram.compile().size();

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(initProgramAsString + "00".repeat(32 - initProgramByteSize))
        .push(0)
        .op(OpCode.MSTORE)
        .push(initProgramByteSize)
        .push(0)
        .push(0)
        .op(OpCode.CREATE);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    // We run the program with a gas cost that triggers OOGX
    // We calculate all opcodes gas before the RETURN opcode
    // 32027L = 3L PUSH + 3L PUSH + 6L MSTORE + 3L PUSH + 3L PUSH + 3L PUSH + 32000L CREATE
    // + (post-Shanghai) 2L INIT CODE COST
    // + ((32027-3-3-6-3-3-3-32000/64))(less than 0.5 so not adding gas) + 3L PUSH + 3L PUSH
    // 21000L for the intrinsic transaction cost
    var initCodeCost = isPostShanghai(fork) ? 2L : 0L;
    var gasCostBefReturn = 32027L + 21000L + initCodeCost;
    bytecodeRunner.run(gasCostBefReturn, chainConfig, testInfo);

    // Max Code Size Exception check before OOGX in tracer
    assertEquals(
        MAX_CODE_SIZE_EXCEPTION,
        bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
  }

  @Test
  void initCodePrefixAndMaxCodeSizeAndOogExceptionForCreate(TestInfo testInfo) {
    // We prepare program with Invalid Code Prefix and Max Code Size exceptions
    int startByteWithICPX = EIP_3541_MARKER;
    int returnSize = MAX_CODE_SIZE + 1;
    BytecodeCompiler programWithICPXAndMCSX =
        getPgCreateInitCodeWithReturnStartByteAndSize(startByteWithICPX, returnSize);

    BytecodeRunner bytecodeRunnerWithICPXAndMCSX =
        BytecodeRunner.of(programWithICPXAndMCSX.compile());
    // We run the program with a gas cost that triggers OOGX
    // We calculate all opcodes gas before the RETURN opcode
    // 32036L = 3L PUSH + 3L PUSH + 6L MSTORE + 3L PUSH + 3L PUSH + 3L PUSH + 32000L CREATE
    // + (post-Shanghai) 2L INIT CODE COST
    // + ((32036-3-3-6-3-3-3-32000)/64)(less than 0.5 so not adding gas) + 3L PUSH + 3L PUSH + 6L
    // MSTORE8 + 3L PUSH + 3L PUSH
    // 21000L for the intrinsic transaction cost
    var initCodeCost = isPostShanghai(fork) ? 2L : 0L;
    var gasCostBefReturn = 32039L + 21000L + initCodeCost;
    bytecodeRunnerWithICPXAndMCSX.run(gasCostBefReturn, chainConfig, testInfo);

    // Max Code Size Exception check is done prior to OOGX and Invalid Code Prefix exception in
    // tracer
    assertEquals(
        MAX_CODE_SIZE_EXCEPTION,
        bytecodeRunnerWithICPXAndMCSX
            .getHub()
            .lastUserTransactionSection(2)
            .commonValues
            .tracedException());
  }
}
