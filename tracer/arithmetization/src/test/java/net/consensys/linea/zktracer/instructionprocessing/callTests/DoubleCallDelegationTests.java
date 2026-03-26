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

import static net.consensys.linea.testing.BytecodeRunner.MAX_GAS_LIMIT;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendFullGasCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendInsufficientBalanceCall;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import kotlin.jvm.functions.Function3;
import kotlin.jvm.functions.Function5;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class DoubleCallDelegationTests extends TracerTestBase {

  /*
  https://github.com/Consensys/linea-monorepo/issues/2470

  In this test, a sender account sends a transaction to a root account, which executes a two CALL or STATICCALL
  instruction to a caller account, which itself executes a CALL instruction to a callee account. Then the root
  account executes some BALANCE instructions.
  Both the caller and the callee can either be simple smart contracts or delegate to other accounts doing the CALL.
  Every portion of code can optionally revert. Loop are also possible, either infinite or exiting after a certain number
  of iterations.

                                                  ┌--------------┐
                                                  |this is where |
                                                  |the scenario  |
                                                  |should apply  |
  tx    --->    root ---[CALL]--->    caller   ---[CALL-type inst]--->    callee
                 |                      |                                   |
                 |                      |                                   |
                 |                      V                                   V
                 |                [?] delegt SMC 1                    [?] delegt SMC 2
                 |
                 |
                 |
                 └------[CALL]--->    caller   ---[CALL-type inst]--->    callee
                 |                      |                                   |
                 |                      |                                   |
                 |                      V                                   V
                 |                [?] delegt SMC 1                    [?] delegt SMC 2
                 |
                 |
                 |
                 └------[further test]
   */

  final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
  final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
  final ToyAccount senderAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(10))
          .nonce(42)
          .address(senderAddress)
          .keyPair(senderKeyPair)
          .build();

  final ToyAccount rootAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(2))
          .nonce(67)
          .address(Address.fromHexString("0x40070000"))
          .build();

  final ToyAccount callerAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(3))
          .nonce(69)
          .address(Address.fromHexString("0xCA11E400"))
          .build();

  final ToyAccount calleeAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(4))
          .nonce(90)
          .address(Address.fromHexString("0xCA11EE00"))
          .build();

  final ToyAccount smcAccount1 =
      ToyAccount.builder()
          .balance(Wei.fromEth(5))
          .nonce(101)
          .address(Address.fromHexString("0xDE1E0FCA11E4"))
          .build();

  final ToyAccount smcAccount2 =
      ToyAccount.builder()
          .balance(Wei.fromEth(6))
          .nonce(666)
          .address(Address.fromHexString("0xDE1E0FCA11EE"))
          .build();

  Function5<
          TentativeCallScenario, ExceptionType, ToyAccount, LoopType, RevertType, BytecodeCompiler>
      callProgram =
          (tentativeCallScenario, exceptionType, targetAccount, loopType, revertType) ->
              BytecodeCompiler.newProgram(chainConfig)
                  .immediate(
                      loopType == LoopType.BOUNDED_LOOP,
                      BytecodeCompiler.newProgram(chainConfig)
                          .push(0)
                          .op(OpCode.SLOAD) // LOOP_DEPTH_CURRENT
                          .push(3) // LOOP_DEPTH_MAX
                          .op(OpCode.GT) // LOOP_DEPTH_MAX > LOOP_DEPTH_CURRENT
                          .push(10)
                          .op(
                              OpCode
                                  .JUMPI) // if LOOP_DEPTH_CURRENT < LOOP_DEPTH_MAX jump to JUMPDEST
                          // else STOP
                          .op(OpCode.STOP)
                          .op(OpCode.JUMPDEST) // PC = 10
                          .compile())
                  .immediate(
                      // we do not want to trigger STATICX due to SSTORE, but due to the CALL below
                      // Note: in case of STATICX the loop type is actually INFINITE_LOOP even if
                      // set to BOUNDED_LOOP
                      exceptionType != ExceptionType.STATICX,
                      BytecodeCompiler.newProgram(chainConfig)
                          .push(0)
                          .op(OpCode.SLOAD)
                          .push(1)
                          .op(OpCode.ADD)
                          .push(0)
                          .op(OpCode.SSTORE)
                          .compile()) // increment LOOP_DEPTH_CURRENT by 1
                  // execute the call
                  .apply(
                      program ->
                          switch (tentativeCallScenario) {
                            case CALL_EXCEPTION ->
                                switch (exceptionType) {
                                  case STATICX ->
                                      // The root previously executed a STATICCALL to the caller
                                      appendFullGasCall(
                                          program,
                                          OpCode.CALL,
                                          targetAccount.getAddress(),
                                          1, // non-zero value to trigger STATICX
                                          0,
                                          0,
                                          0,
                                          0);
                                  case OOGX -> {
                                    // rac = 2^20 gives a quadratic cost of (rac / 32)^2 / 512 =
                                    // 2^21 which exceeds the gas in the current frame, which is
                                    // 200_000
                                    final int rac = 1 << 20;
                                    yield appendFullGasCall(
                                        program,
                                        OpCode.CALL,
                                        targetAccount.getAddress(),
                                        0,
                                        0,
                                        0,
                                        0,
                                        rac);
                                  }
                                  case NONE ->
                                      throw new IllegalArgumentException(
                                          "Invalid exception type for CALL_EXCEPTION scenario");
                                };
                            case CALL_ABORT_WILL_REVERT, CALL_ABORT_WONT_REVERT ->
                                appendInsufficientBalanceCall(
                                    program, OpCode.CALL, targetAccount.getAddress(), 0, 0, 0, 0);
                            default ->
                                appendFullGasCall(
                                    program,
                                    OpCode.CALL,
                                    targetAccount.getAddress(),
                                    0,
                                    0,
                                    0,
                                    0,
                                    0);
                          })
                  // preparing for a potential revert
                  .push(0)
                  .push(0)
                  .op(revertType == RevertType.TERMINATES_ON_REVERT ? OpCode.REVERT : OpCode.STOP);

  Function3<ExceptionType, LoopType, RevertType, BytecodeCompiler> rootProgram =
      (exceptionType, loopType, revertType) ->
          BytecodeCompiler.newProgram(chainConfig)
              .immediate(
                  loopType == LoopType.BOUNDED_LOOP,
                  BytecodeCompiler.newProgram(chainConfig)
                      .push(0)
                      .op(OpCode.SLOAD) // LOOP_DEPTH_CURRENT
                      .push(3) // LOOP_DEPTH_MAX
                      .op(OpCode.GT) // LOOP_DEPTH_MAX > LOOP_DEPTH_CURRENT
                      .push(10)
                      .op(OpCode.JUMPI) // if LOOP_DEPTH_CURRENT < LOOP_DEPTH_MAX jump to JUMPDEST
                      // else STOP
                      .op(OpCode.STOP)
                      .op(OpCode.JUMPDEST) // PC = 10
                      .compile())
              .push(0)
              .op(OpCode.SLOAD)
              .push(1)
              .op(OpCode.ADD)
              .push(0)
              .op(OpCode.SSTORE) // increment LOOP_DEPTH_CURRENT by 1
              // execute 2 identical calls to callerAccount
              .apply(
                  program ->
                      appendCall(
                          program,
                          exceptionType == ExceptionType.STATICX ? OpCode.STATICCALL : OpCode.CALL,
                          200_000,
                          callerAccount.getAddress(),
                          1,
                          0,
                          0,
                          0,
                          0),
                  2)
              .push(rootAccount.getAddress().getBytes())
              .op(OpCode.BALANCE)
              .op(OpCode.POP)
              .push(callerAccount.getAddress().getBytes())
              .op(OpCode.BALANCE)
              .op(OpCode.POP)
              .push(calleeAccount.getAddress().getBytes())
              .op(OpCode.BALANCE)
              .op(OpCode.POP)
              // preparing for a potential revert
              .push(0)
              .push(0)
              .op(revertType == RevertType.TERMINATES_ON_REVERT ? OpCode.REVERT : OpCode.STOP);

  public enum CallerType {
    DELEGATED,
    SMC
  }

  public enum CalleeType {
    // the first few we don't really care about: they don't lead to execution
    // DELEGATED_TO_NON_EXISTENT,
    // DELEGATED_TO_EMPTY_CODE_ACCOUNT,
    // DELEGATED_TO_PRC,
    EOA,
    DELEGATED_TO_SELF,
    DELEGATED_TO_ROOT,
    DELEGATED_TO_CALLER,
    DELEGATED_TO_SMC,
    SMC;
  }

  // this should apply per smart contract
  public enum RevertType {
    TERMINATES_ON_REVERT,
    TERMINATES_ON_NON_REVERT;
  }

  // this should apply uniformly to all smart contracts
  public enum LoopType {
    INFINITE_LOOP,
    BOUNDED_LOOP;
  }

  /** In this family of tests we are interested only in static and out-of-gas exceptions. */
  public enum ExceptionType {
    NONE,
    STATICX,
    OOGX
  }

  /** In this family of tests we interested only in these call scenarios */
  public enum TentativeCallScenario {
    UNDEFINED,
    CALL_EXCEPTION,
    CALL_ABORT_WILL_REVERT,
    CALL_ABORT_WONT_REVERT,
    // Externally owned account call scenarios
    CALL_EOA_SUCCESS_WILL_REVERT,
    CALL_EOA_SUCCESS_WONT_REVERT,
    // Smart contract call scenarios:
    CALL_SMC_FAILURE_WILL_REVERT,
    CALL_SMC_FAILURE_WONT_REVERT,
    CALL_SMC_SUCCESS_WILL_REVERT,
    CALL_SMC_SUCCESS_WONT_REVERT;

    public boolean isAnyOf(TentativeCallScenario... scenarios) {
      for (TentativeCallScenario scenario : scenarios) {
        if (this == scenario) {
          return true;
        }
      }
      return false;
    }

    public boolean isFailureCallScenario() {
      return this.isAnyOf(CALL_SMC_FAILURE_WILL_REVERT, CALL_SMC_FAILURE_WONT_REVERT);
    }

    public boolean isWillRevertCallScenario() {
      return this.isAnyOf(
          CALL_ABORT_WILL_REVERT,
          CALL_EOA_SUCCESS_WILL_REVERT,
          CALL_SMC_FAILURE_WILL_REVERT,
          CALL_SMC_SUCCESS_WILL_REVERT);
    }

    public boolean isSmcCallScenario() {
      return this.isAnyOf(
          CALL_SMC_FAILURE_WILL_REVERT,
          CALL_SMC_FAILURE_WONT_REVERT,
          CALL_SMC_SUCCESS_WILL_REVERT,
          CALL_SMC_SUCCESS_WONT_REVERT);
    }

    public boolean isEoaCallScenario() {
      return this.isAnyOf(CALL_EOA_SUCCESS_WILL_REVERT, CALL_EOA_SUCCESS_WONT_REVERT);
    }
  }

  @ParameterizedTest
  @MethodSource("doubleCallDelegationTestsSource")
  public void doubleCallDelegationTests(
      TentativeCallScenario callerToCalleeTentativeCallScenario,
      ExceptionType exceptionType,
      CallerType callerType,
      CalleeType calleeType,
      RevertType rootCodeRevertType,
      RevertType calleeCodeRevertType,
      LoopType loopType,
      TestInfo testInfo) {
    /*
     Scenarios and how they are triggered:
     CALL_EXCEPTION:
      - STATICX: root STATICCALLs caller and caller CALLs callee with non-zero value
      - OOGX: caller CALLs callee and the upfront cost of the call exceeds the remaining gas
     CALL_ABORT_WILL_REVERT: caller CALLs to the callee with insufficient balance to cover value transfer, then reverts
     CALL_ABORT_WONT_REVERT: caller CALLs to the callee with insufficient balance to cover value transfer, but doesn't revert
     CALL_EOA_SUCCESS_WILL_REVERT: caller CALLs to an EOA which successfully processes the call but then reverts
     CALL_EOA_SUCCESS_WONT_REVERT: caller CALLs to an EOA which successfully processes the call and doesn't revert
     CALL_SMC_FAILURE_WILL_REVERT: caller CALLs to an SMC which fails (e.g., by executing the INVALID opcode) and reverts
     CALL_SMC_FAILURE_WONT_REVERT: caller CALLs to an SMC which fails (e.g., by executing the INVALID opcode) but doesn't revert
     CALL_SMC_SUCCESS_WILL_REVERT: caller CALLs to an SMC which successfully processes the call but then reverts
     CALL_SMC_SUCCESS_WONT_REVERT: caller CALLs to an SMC which successfully processes the call and doesn't revert
    */
    rootAccount.setCode(rootProgram.invoke(exceptionType, loopType, rootCodeRevertType).compile());

    switch (callerType) {
      case DELEGATED -> {
        callerAccount.delegateTo(smcAccount1);
        smcAccount1.setCode(
            callProgram
                .invoke(
                    callerToCalleeTentativeCallScenario,
                    exceptionType,
                    calleeAccount,
                    loopType,
                    callerToCalleeTentativeCallScenario.isWillRevertCallScenario()
                        ? RevertType.TERMINATES_ON_REVERT
                        : RevertType.TERMINATES_ON_NON_REVERT)
                .compile());
      }
      case SMC ->
          callerAccount.setCode(
              callProgram
                  .invoke(
                      callerToCalleeTentativeCallScenario,
                      exceptionType,
                      calleeAccount,
                      loopType,
                      callerToCalleeTentativeCallScenario.isWillRevertCallScenario()
                          ? RevertType.TERMINATES_ON_REVERT
                          : RevertType.TERMINATES_ON_NON_REVERT)
                  .compile());
    }

    switch (calleeType) {
      case EOA -> calleeAccount.setCode(Bytes.EMPTY); // do nothing as EOAs have no code
      case DELEGATED_TO_SELF -> calleeAccount.delegateTo(calleeAccount);
      case DELEGATED_TO_ROOT -> calleeAccount.delegateTo(rootAccount);
      case DELEGATED_TO_CALLER -> calleeAccount.delegateTo(callerAccount);
      case DELEGATED_TO_SMC -> {
        calleeAccount.delegateTo(smcAccount2);
        smcAccount2.setCode(
            callerToCalleeTentativeCallScenario.isFailureCallScenario()
                ? BytecodeCompiler.newProgram(chainConfig).op(OpCode.INVALID).compile()
                : callProgram
                    .invoke(
                        TentativeCallScenario.UNDEFINED,
                        ExceptionType.NONE,
                        callerAccount,
                        loopType,
                        calleeCodeRevertType)
                    .compile()); // This could be a call to anything
      }
      case SMC -> {
        calleeAccount.setCode(
            callerToCalleeTentativeCallScenario.isFailureCallScenario()
                ? BytecodeCompiler.newProgram(chainConfig).op(OpCode.INVALID).compile()
                : callProgram
                    .invoke(
                        TentativeCallScenario.UNDEFINED,
                        ExceptionType.NONE,
                        callerAccount,
                        loopType,
                        calleeCodeRevertType)
                    .compile()); // This could be a call to anything
      }
    }

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(rootAccount)
            .keyPair(senderKeyPair)
            .gasLimit(MAX_GAS_LIMIT)
            .build();

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(
                List.of(
                    senderAccount,
                    rootAccount,
                    callerAccount,
                    calleeAccount,
                    smcAccount1,
                    smcAccount2))
            .transaction(tx)
            .build();
    toyExecutionEnvironmentV2.run();
  }

  static Stream<Arguments> doubleCallDelegationTestsSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (TentativeCallScenario callerToCalleeTentativeCallScenario :
        new TentativeCallScenario[] {
          TentativeCallScenario.CALL_EXCEPTION,
          TentativeCallScenario.CALL_ABORT_WILL_REVERT,
          TentativeCallScenario.CALL_ABORT_WONT_REVERT,
          TentativeCallScenario.CALL_EOA_SUCCESS_WILL_REVERT,
          TentativeCallScenario.CALL_EOA_SUCCESS_WONT_REVERT,
          TentativeCallScenario.CALL_SMC_FAILURE_WILL_REVERT,
          TentativeCallScenario.CALL_SMC_FAILURE_WONT_REVERT,
          TentativeCallScenario.CALL_SMC_SUCCESS_WILL_REVERT,
          TentativeCallScenario.CALL_SMC_SUCCESS_WONT_REVERT,
        }) {
      for (ExceptionType exceptionType : ExceptionType.values()) {
        if (skipTest(callerToCalleeTentativeCallScenario, exceptionType)) {
          continue;
        }
        for (CalleeType calleeType : CalleeType.values()) {
          if (skipTest(callerToCalleeTentativeCallScenario, calleeType)) {
            continue;
          }
          for (CallerType callerType : CallerType.values()) {
            for (RevertType rootCodeRevertType : RevertType.values()) {
              for (RevertType calleeCodeRevertType : RevertType.values()) {
                for (LoopType loopType : LoopType.values()) {
                  arguments.add(
                      Arguments.of(
                          callerToCalleeTentativeCallScenario,
                          exceptionType,
                          callerType,
                          calleeType,
                          rootCodeRevertType,
                          calleeCodeRevertType,
                          loopType));
                }
              }
            }
          }
        }
      }
    }

    /*
    // For debugging:
    arguments.clear();
    arguments.add(
        Arguments.of(
            CALL_ABORT_WONT_REVERT,
            ExceptionType.NONE,
            CallerType.DELEGATED,
            CalleeType.DELEGATED_TO_ROOT,
            RevertType.TERMINATES_ON_REVERT,
            RevertType.TERMINATES_ON_NON_REVERT,
            LoopType.BOUNDED_LOOP));
    */

    return arguments.stream();
  }

  private static boolean skipTest(
      TentativeCallScenario callerToCalleeTentativeCallScenario, ExceptionType exceptionType) {
    return (callerToCalleeTentativeCallScenario == TentativeCallScenario.CALL_EXCEPTION
            && exceptionType == ExceptionType.NONE)
        || (callerToCalleeTentativeCallScenario != TentativeCallScenario.CALL_EXCEPTION
            && exceptionType != ExceptionType.NONE);
  }

  private static boolean skipTest(
      TentativeCallScenario callerToCalleeTentativeCallScenario, CalleeType calleeType) {
    return switch (calleeType) {
      case EOA -> callerToCalleeTentativeCallScenario.isSmcCallScenario();
      case DELEGATED_TO_SELF, DELEGATED_TO_ROOT, DELEGATED_TO_CALLER, DELEGATED_TO_SMC, SMC ->
          callerToCalleeTentativeCallScenario.isEoaCallScenario();
        // Note that caller always has non-empty bytecode
    };
  }
}
