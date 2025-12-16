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

import java.util.ArrayList;
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
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for CALL opcode.
List of the combinations tested below
STATIC & OOGX : CALL
STATIC & MXPX : CALL
STATIC & ROOB : CALL
Note : As MXPX is a subcase of OOGX, we don't test MXPX & OOGX
 */

@ExtendWith(UnitTestWatcher.class)
public class CallTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("addExistsAndIsWarmCallSource")
  void staticAndOogExceptionsCall(boolean targetAddressExists, boolean isWarm, TestInfo testInfo) {
    // value has to be > 0 for static exception to be triggered on CALL
    int value = 1;
    //  When value is transferred
    //   -> Add additional call stipend (2300) to avoid OOGX in order to complete the call
    // execution, even if no code is executed
    // call stipend - 1
    int cornerCase = 2299;
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    if (targetAddressExists && isWarm) {
      // Note: this is a possible way to warm the address
      program.push("ca11ee").op(OpCode.BALANCE);
    }

    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push(value) // value
        .push("ca11ee") // address
        .push(0) // gas for subcontext (floored at 2300)
        .op(OpCode.CALL);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);
    long gasCost;
    BytecodeRunner bytecodeRunnerStaticCall;

    ToyAccount CallProviderAccount = getAccountForAddressWithBytecode(codeAddress, pgCompile);

    if (targetAddressExists) {
      final ToyAccount calleeAccount =
          ToyAccount.builder()
              .balance(Wei.fromEth(1))
              .nonce(10)
              .address(Address.fromHexString("ca11ee"))
              .build();
      gasCost = bytecodeRunner.runOnlyForGasCost(List.of(calleeAccount), chainConfig, testInfo);
      // We calculate gas cost to trigger OOGX
      // We retrieve the gas cost of the transaction as it's the gas used for the static call, so
      // intrinsic gas cost already accounted
      int gasCostPlusCornerCase = (int) gasCost + cornerCase - GAS_CONST_G_TRANSACTION;
      BytecodeCompiler pgStaticCallToCode =
          getProgramStaticCallToCodeAddress(gasCostPlusCornerCase);
      bytecodeRunnerStaticCall = BytecodeRunner.of(pgStaticCallToCode.compile());
      bytecodeRunnerStaticCall.run(
          List.of(calleeAccount, CallProviderAccount), chainConfig, testInfo);
    } else {
      gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);
      // We calculate gas cost to trigger OOGX
      // We retrieve the gas cost of the transaction as it's the gas used for the static call, so
      // intrinsic gas cost already accounted
      int gasCostPlusCornerCase = (int) gasCost + cornerCase - GAS_CONST_G_TRANSACTION;
      BytecodeCompiler pgStaticCallToCode =
          getProgramStaticCallToCodeAddress(gasCostPlusCornerCase);
      bytecodeRunnerStaticCall = BytecodeRunner.of(pgStaticCallToCode.compile());
      bytecodeRunnerStaticCall.run(
          gasCost + cornerCase, List.of(CallProviderAccount), chainConfig, testInfo);
    }

    assertEquals(
        STATIC_FAULT,
        bytecodeRunnerStaticCall
            .getHub()
            .lastUserTransactionSection(2)
            .commonValues
            .tracedException());
  }

  static Stream<Arguments> addExistsAndIsWarmCallSource() {
    List<Arguments> arguments = new ArrayList<>();
    arguments.add(Arguments.of(true, true));
    arguments.add(Arguments.of(true, false));
    arguments.add(Arguments.of(false, false));
    return arguments.stream();
  }

  @Test
  public void staticAndMxpExceptionsCall(TestInfo testInfo) {
    boolean triggerMaxCodeSizeException = false;
    // We test with or without Roob
    boolean[] triggerRoob = new boolean[] {false, true};

    for (boolean roob : triggerRoob) {
      // We prepare a program with an MXPX for the opcode
      BytecodeCompiler pg = BytecodeCompiler.newProgram(chainConfig);
      new MxpTestUtils(opcodes)
          .triggerNonTrivialButMxpxOrRoobOrMaxCodeSizeExceptionForOpCode(
              fork, pg, roob, triggerMaxCodeSizeException, OpCode.CALL);

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
}
