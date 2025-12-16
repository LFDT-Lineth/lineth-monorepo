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
package net.consensys.linea.zktracer.instructionprocessing.selfdestructTests;

import static net.consensys.linea.zktracer.instructionprocessing.selfdestructTests.Heir.*;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.*;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;

import java.util.List;
import java.util.Optional;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

/**
 * Basic tests for SELFDESTRUCT opcode. They test interactions with REVERT opcode. We consider the
 * case of 3 successful SELFDESTRUCT's in a row at the same selfDestructorAddress. The
 * selfDestructorAddress parameter decides whether the SELFDESTRUCT targets the same
 * selfDestructorAddress or not. We consider the reverted vs unreverted cases.
 */
@ExtendWith(UnitTestWatcher.class)
public class RepeatedSelfDestructsOfSameAccountTests extends TracerTestBase {

  private ToyAccount toAccount;

  private BytecodeCompiler toAccountCode(ChainConfig chainConfig) {
    return BytecodeCompiler.newProgram(chainConfig);
  }

  private ToyAccount selfDestructorAccount;

  private void buildToAccount(ChainConfig chainConfig) {
    toAccount =
        ToyAccount.builder()
            .address(Address.fromHexString("0x1234567890"))
            .balance(Wei.fromEth(2))
            .code(toAccountCode(chainConfig).compile())
            .nonce(23)
            .build();
  }

  private Transaction transaction() {
    return ToyTransaction.builder()
        .keyPair(keyPair)
        .sender(userAccount)
        .to(toAccount)
        .transactionType(TransactionType.FRONTIER)
        .gasLimit(500_000L)
        .value(Wei.ONE)
        .build();
  }

  private void run(ChainConfig chainConfig, TestInfo testInfo, Heir heir) {
    selfDestructorAccount = basicSelfDestructor(heir, Optional.empty(), chainConfig);
    buildToAccount(chainConfig);
    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(userAccount, toAccount, selfDestructorAccount))
        .transaction(transaction())
        .build()
        .run();
  }

  /**
   * The root contract CALL's the selfdestructor thrice, each time providing him with new balance.
   */
  @ParameterizedTest
  @EnumSource(Heir.class)
  public void sameAccountSelfDestructsThrice(Heir heir, TestInfo testInfo) {

    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        19,
        0,
        3,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        26,
        0,
        2,
        0,
        0);

    run(chainConfig, testInfo, heir);
  }

  @ParameterizedTest
  @EnumSource(Heir.class)
  public void sameAccountSelfDestructsThriceReverted(Heir heir, TestInfo testInfo) {

    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        19,
        0,
        3,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        26,
        0,
        2,
        0,
        0);
    appendRevert(toAccountCode(chainConfig), 0, 0);

    run(chainConfig, testInfo, heir);
  }

  /**
   * DELEGATECALL induces SELFDESTRUCT in the caller. A second DELEGATECALL does it again.
   *
   * <p>The second SELFDESTRUCT should go through due to DELEGATECALL not transferring value.
   */
  @ParameterizedTest
  @EnumSource(Heir.class)
  public void calleeInducesSelfDestructInCallerViaDelegateCall(Heir heir, TestInfo testInfo) {

    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        19,
        0,
        3,
        0,
        0);

    run(chainConfig, testInfo, heir);
  }

  @ParameterizedTest
  @EnumSource(Heir.class)
  public void calleeInducesSelfDestructInCallerViaDelegateCallReverted(
      Heir heir, TestInfo testInfo) {

    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        19,
        0,
        3,
        0,
        0);
    appendRevert(toAccountCode(chainConfig), 0, 0);

    run(chainConfig, testInfo, heir);
  }

  /** The second call should abort due to not having any funds left. */
  @ParameterizedTest
  @EnumSource(Heir.class)
  public void calleeInducesSelfDestructInCallerViaCallCode(Heir heir, TestInfo testInfo) {
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALLCODE,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALLCODE,
        100_000,
        Heir.selfDestructorAddress,
        19,
        0,
        3,
        0,
        0);

    run(chainConfig, testInfo, heir);
  }

  @ParameterizedTest
  @EnumSource(Heir.class)
  public void calleeInducesSelfDestructInCallerViaCallCodeReverted(Heir heir, TestInfo testInfo) {
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALLCODE,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALLCODE,
        100_000,
        Heir.selfDestructorAddress,
        19,
        0,
        3,
        0,
        0);
    appendRevert(toAccountCode(chainConfig), 0, 0);

    run(chainConfig, testInfo, heir);
  }

  /**
   * DELEGATECALL into account that induces SELFDESTRUCT in caller followed by the caller CALL'ing
   * the callee again inducing SELFDESTRUCT in callee this time.
   *
   * <p>Both reverted and unreverted versions
   */
  @ParameterizedTest
  @EnumSource(Heir.class)
  public void callerThenCalleeSelfDestruct(Heir heir, TestInfo testInfo) {
    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        0,
        0,
        3,
        0,
        0);

    run(chainConfig, testInfo, heir);
  }

  @ParameterizedTest
  @EnumSource(Heir.class)
  public void callerThenCalleeSelfDestructReverted(Heir heir, TestInfo testInfo) {
    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        0,
        0,
        3,
        0,
        0);

    run(chainConfig, testInfo, heir);
  }

  /**
   * CALL into account which SELFDESTRUCT's followed by the caller DELEGATECALL'ing the callee again
   * inducing SELFDESTRUCT in caller this time.
   *
   * <p>Both reverted and unreverted versions
   */
  @ParameterizedTest
  @EnumSource(Heir.class)
  public void calleeThenCallerSelfDestruct(Heir heir, TestInfo testInfo) {
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        25,
        0,
        3,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);

    run(chainConfig, testInfo, heir);
  }

  @ParameterizedTest
  @EnumSource(Heir.class)
  public void calleeThenCallerSelfDestructReverted(Heir heir, TestInfo testInfo) {
    appendCall(
        toAccountCode(chainConfig),
        OpCode.CALL,
        100_000,
        Heir.selfDestructorAddress,
        25,
        0,
        3,
        0,
        0);
    appendCall(
        toAccountCode(chainConfig),
        OpCode.DELEGATECALL,
        100_000,
        Heir.selfDestructorAddress,
        12,
        0,
        4,
        0,
        0);
    appendRevert(toAccountCode(chainConfig), 0, 0);

    run(chainConfig, testInfo, heir);
  }

  @ParameterizedTest
  @EnumSource(Heir.class)
  public void basicSelfDestruct(Heir heir, TestInfo testInfo) {
    selfDestructorAccount = basicSelfDestructor(heir, Optional.empty(), chainConfig);
    buildToAccount(chainConfig);
    Transaction transaction =
        ToyTransaction.builder()
            .keyPair(keyPair)
            .sender(userAccount)
            .to(selfDestructorAccount)
            .transactionType(TransactionType.FRONTIER)
            .gasLimit(500_000L)
            .value(Wei.of(0xeeff))
            .build();
    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(userAccount, toAccount, selfDestructorAccount))
        .transaction(transaction)
        .build()
        .run();
  }
}
