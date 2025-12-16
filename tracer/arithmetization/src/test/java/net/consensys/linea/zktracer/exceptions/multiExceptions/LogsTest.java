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

import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_TRANSACTION;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.*;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.getProgramStaticCallToCodeAddress;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.STATIC_FAULT;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.module.mxp.MxpTestUtils;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for Logs family opcodes.
List of the combinations tested below
STATIC & OOGX : LOG0, LOG1, LOG2, LOG3, LOG4
STATIC & MXPX : LOG0, LOG1, LOG2, LOG3, LOG4
STATIC & ROOB : LOG0, LOG1, LOG2, LOG3, LOG4
Note : As MXPX is a subcase of OOGX, we don't test MXPX & OOGX
 */

@ExtendWith(UnitTestWatcher.class)
public class LogsTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("logsOpCodesList")
  void staticAndOogExceptionsLogs(OpCode opCode, TestInfo testInfo) {

    BytecodeCompiler program = simpleProgram(opCode);
    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);
    long gasCostTx = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    int cornerCase = -1;
    // We calculate gas cost to trigger OOGX
    int gasCostMinusCornerCase = (int) gasCostTx - GAS_CONST_G_TRANSACTION + cornerCase;

    // We prepare a program with a static call to code account
    ToyAccount codeProviderAccount = getAccountForAddressWithBytecode(codeAddress, pgCompile);
    BytecodeCompiler pgStaticCallToCode = getProgramStaticCallToCodeAddress(gasCostMinusCornerCase);

    // Run with linea block gas limit so gas cost is passed to child without 63/64
    BytecodeRunner bytecodeRunnerStaticCall = BytecodeRunner.of(pgStaticCallToCode.compile());
    bytecodeRunnerStaticCall.run(List.of(codeProviderAccount), chainConfig, testInfo);

    // Static check happens before OOGX in tracer
    assertEquals(
        STATIC_FAULT,
        bytecodeRunnerStaticCall
            .getHub()
            .lastUserTransactionSection(2)
            .commonValues
            .tracedException());
  }

  @ParameterizedTest
  @MethodSource("logsOpCodesList")
  public void staticAndMxpExceptions(OpCode opCode, TestInfo testInfo) {
    boolean triggerMaxCodeSizeException = false;
    // We test with or without Roob
    boolean[] triggerRoob = new boolean[] {false, true};

    for (boolean roob : triggerRoob) {
      // We prepare a program with an MXPX for the opcode
      BytecodeCompiler pg = BytecodeCompiler.newProgram(chainConfig);
      new MxpTestUtils(opcodes)
          .triggerNonTrivialButMxpxOrRoobOrMaxCodeSizeExceptionForOpCode(
              fork, pg, roob, triggerMaxCodeSizeException, opCode);

      // We prepare a program to static call the code account
      ToyAccount codeProviderAccount = getAccountForAddressWithBytecode(codeAddress, pg.compile());
      BytecodeCompiler pgStaticCallToCode = getProgramStaticCallToCodeAccount();

      // We run the program to static call the account with MXPX code
      BytecodeRunner bytecodeRunnerStaticCall = BytecodeRunner.of(pgStaticCallToCode.compile());
      bytecodeRunnerStaticCall.run(List.of(codeProviderAccount), chainConfig, testInfo);

      // Static check happens before MXPX
      assertEquals(
          STATIC_FAULT,
          bytecodeRunnerStaticCall
              .getHub()
              .lastUserTransactionSection(2)
              .commonValues
              .tracedException());
    }
  }

  static Stream<OpCode> logsOpCodesList() {
    List<OpCode> opCodesListArgument =
        Arrays.asList(OpCode.LOG0, OpCode.LOG1, OpCode.LOG2, OpCode.LOG3, OpCode.LOG4);
    return opCodesListArgument.stream();
  }
}
