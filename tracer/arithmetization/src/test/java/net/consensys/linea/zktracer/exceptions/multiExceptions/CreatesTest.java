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
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_TRANSACTION;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.*;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.*;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.module.hub.signals.TracedException;
import net.consensys.linea.zktracer.module.mxp.MxpTestUtils;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for CREATE/CREATE2 opcodes.
List of the combinations tested below
STATIC & OOGX : CREATE, CREATE2
STATIC & MXPX : CREATE, CREATE2
STATIC & ROOB : CREATE, CREATE2
(Post-Shanghai, we test subsets of possible exceptions with MAX_CODE_SIZE_EXCEPTION)
STATIC & MAX_CODE_SIZE_EXCEPTION : CREATE, CREATE2
OOGX & MAX_CODE_SIZE_EXCEPTION : CREATE, CREATE2
MXPX & MAX_CODE_SIZE_EXCEPTION : CREATE, CREATE2
Note : As MXPX is a subcase of OOGX, we don't test MXPX & OOGX
 */

@ExtendWith(UnitTestWatcher.class)
public class CreatesTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("createOpCodesList")
  void staticAndOogExceptionsCreates(OpCode opCode, TestInfo testInfo) {

    BytecodeCompiler program = simpleProgram(opCode);
    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);
    long gasCostTx = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    /*
    for CREATE/CREATE2, Static Exception happens before deployment, so we test OOGX before deployment
    gasCostTx is the gas cost calculated when the program, which we will later static call, goes to completion (simple create with init code in memory)
    We remove 6400 (depositFee) + deployment code exec cost (18) from total gas cost calculated as well as (1) to trigger OOGX on the Creates
     */
    int cornerCase = -6419;
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
  @MethodSource("createOpCodesList")
  public void staticAndMxpExceptionsCreates(OpCode opCode, TestInfo testInfo) {
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

  /** Post-shanghai, the following tests might trigger a MAX_CODE_SIZE_EXCEPTION */
  @ParameterizedTest
  @MethodSource("createOpCodesList")
  public void staticAndMaxCodeSizeExceptionsCreates(OpCode opCode, TestInfo testInfo) {
    Bytes32 initCodeChunk = Bytes32.repeat((byte) 0x30);
    BytecodeCompiler pg = getPgCreateWithInitCodeSize(opCode, initCodeChunk, 1537);

    // We prepare a program to static call the code account
    ToyAccount codeProviderAccount = getAccountForAddressWithBytecode(codeAddress, pg.compile());
    BytecodeCompiler pgStaticCallToCode = getProgramStaticCallToCodeAccount();

    // We run the program to static call the account with code that creates with an init code size
    // exception
    BytecodeRunner bytecodeRunnerStaticCall = BytecodeRunner.of(pgStaticCallToCode.compile());
    bytecodeRunnerStaticCall.run(List.of(codeProviderAccount), chainConfig, testInfo);

    // Static check happens before MAX_CODE_SIZE_EXCEPTION
    assertEquals(
        STATIC_FAULT,
        bytecodeRunnerStaticCall
            .getHub()
            .lastUserTransactionSection(2)
            .commonValues
            .tracedException());
  }

  @ParameterizedTest
  @MethodSource("createOpCodesList")
  public void OogAndMaxCodeSizeExceptionsCreates(OpCode opCode, TestInfo testInfo) {
    // Dummy init code, repeats ADDRESS opcode
    Bytes32 initCodeChunk = Bytes32.repeat((byte) 0x30);

    // To calculate the gas cost, we prepare a program with an init code size of exactly (1536 * 32)
    // = 49152 bytes to avoid Max code size exception
    BytecodeCompiler initCodeForGasCost = getInitCodeWithSize(initCodeChunk, 1536);
    // We run the program to calculate the amount of gas cost
    BytecodeRunner bytecodeRunnerInitCodeForGasCost =
        BytecodeRunner.of(initCodeForGasCost.compile());
    long gasCostForInitCodeWithoutMaxCodeSizeException =
        bytecodeRunnerInitCodeForGasCost.runOnlyForGasCost(chainConfig, testInfo);

    // We now prepare a create program with an init code of (1537 * 32) byte size that will trigger
    // a Max code size exception
    BytecodeCompiler pg = getPgCreateWithInitCodeSize(opCode, initCodeChunk, 1537);
    // We calculate the gas cost to trigger OOGX based on
    // gasCostForInitCodeWithoutMaxCodeSizeException
    // gasCostForCreateProgramOOGX = gasCostForInitCodeWithoutMaxCodeSizeException + 6L + 12L (2
    // PUSHES + MSTORE) to add the 1537th extra chunk in memory + 9L (3 PUSHES for creates
    // arguments) + 3L to PUSH an extra CREATE2 argument (salt) + 1L to enter the CREATE
    long extraPushCreate2 = (opCode == OpCode.CREATE2) ? 3L : 0L;
    long gasCostForCreateProgramOOGX =
        gasCostForInitCodeWithoutMaxCodeSizeException + 6L + 12L + 9L + extraPushCreate2 + 1L;
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pg.compile());
    bytecodeRunner.run(gasCostForCreateProgramOOGX, chainConfig, testInfo);

    // (Post-Shanghai) MAX_CODE_SIZE_EXCEPTION check happens before OOGX in tracer
    TracedException exceptionTriggered =
        isPostShanghai(fork) ? MAX_CODE_SIZE_EXCEPTION : OUT_OF_GAS_EXCEPTION;
    assertEquals(
        exceptionTriggered,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @ParameterizedTest
  @MethodSource("createOpCodesList")
  public void MxpAndMaxCodeSizeExceptionExceptionsCreates(OpCode opCode, TestInfo testInfo) {
    // We test with or without Roob
    boolean maxCodeSizeException = true;
    boolean[] triggerRoob = new boolean[] {false, true};

    for (boolean roob : triggerRoob) {
      // We prepare a program with an MXPX and MAX_CODE_SIZE_EXCEPTION for the opcode
      BytecodeCompiler pg = BytecodeCompiler.newProgram(chainConfig);
      new MxpTestUtils(opcodes)
          .triggerNonTrivialButMxpxOrRoobOrMaxCodeSizeExceptionForOpCode(
              fork, pg, roob, maxCodeSizeException, opCode);

      // We run the program
      BytecodeRunner bytecodeRunner = BytecodeRunner.of(pg.compile());
      bytecodeRunner.run(chainConfig, testInfo);

      // (Post-Shanghai) MAX_CODE_SIZE_EXCEPTION check is done prior to MXPX
      TracedException exceptionTriggered =
          isPostShanghai(fork) ? MAX_CODE_SIZE_EXCEPTION : MEMORY_EXPANSION_EXCEPTION;
      assertEquals(
          exceptionTriggered,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    }
  }

  static Stream<OpCode> createOpCodesList() {
    List<OpCode> opCodesListArgument = Arrays.asList(OpCode.CREATE, OpCode.CREATE2);
    return opCodesListArgument.stream();
  }
}
