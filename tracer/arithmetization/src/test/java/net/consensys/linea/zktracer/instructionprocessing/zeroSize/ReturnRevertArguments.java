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

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

@ExtendWith(UnitTestWatcher.class)
public class ReturnRevertArguments extends TracerTestBase {

  public String hugeOffset = "ff".repeat(32);

  /**
   * The following test does the following:
   *
   * <p>- the transaction targets a smart contract that immediately <b>RETURN</b>'s /
   * <b>REVERT</b>'s with zero size, huge offset
   *
   * @param opCode either {@link OpCode#RETURN} or {@link OpCode#REVERT}
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"RETURN", "REVERT"})
  void rootContextMessageCall(OpCode opCode, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    zeroSizeReturnOrRevert(program, opCode);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  /**
   * The following test does the following:
   *
   * <p>- a deployment transaction that runs init code that immediately <b>RETURN</b>'s /
   * <b>REVERT</b>'s with zero size, huge offset
   *
   * @param opCode either {@link OpCode#RETURN} or {@link OpCode#REVERT}
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"RETURN", "REVERT"})
  void rootContextDeploymentTransaction(OpCode opCode, TestInfo testInfo) {
    checkArgument(opCode.isAnyOf(RETURN, REVERT));

    BytecodeCompiler initCode = BytecodeCompiler.newProgram(chainConfig);
    zeroSizeReturnOrRevert(initCode, opCode);

    Transaction deploymentTransaction =
        ToyTransaction.builder()
            .sender(userAccount)
            .keyPair(keyPair)
            .gasPrice(Wei.of(8L))
            .gasLimit(1_000_000L)
            .value(Wei.of(1L))
            .payload(initCode.compile())
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(userAccount))
        .transaction(deploymentTransaction)
        .build()
        .run();
  }

  /**
   * The following test does the following:
   *
   * <p>- <b>CALL</b> a smart contract that immediately <b>RETURN</b>'s / <b>REVERT</b>'s with zero
   * size, huge offset
   *
   * <p>- back in the root context, check the <b>BALANCE</b> of both accounts for good measure
   *
   * @param opCode either {@link OpCode#RETURN} or {@link OpCode#REVERT}
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"RETURN", "REVERT"})
  void nonRootContextMessageCall(OpCode opCode, TestInfo testInfo) {
    checkArgument(opCode.isAnyOf(RETURN, REVERT));

    Address calleeAccountAddress = Address.fromHexString("ca11eec0def3fd");
    Bytes calleeAccountAddressBytes = calleeAccountAddress.getBytes();
    BytecodeCompiler calleeAccountCode = BytecodeCompiler.newProgram(chainConfig);
    zeroSizeReturnOrRevert(calleeAccountCode, opCode);

    ToyAccount calleeAccount =
        ToyAccount.builder()
            .address(calleeAccountAddress)
            .nonce(71)
            .balance(Wei.of(512L))
            .code(calleeAccountCode.compile())
            .build();

    Address callerAccountAddress = Address.fromHexString("ca11e7c0de");
    BytecodeCompiler callerAccountCode = BytecodeCompiler.newProgram(chainConfig);
    callerAccountCode
        .push(0) // r@c
        .push(0) // r@o
        .op(CALLDATASIZE) // cds
        .push(0) // cdo
        .push(255) // 0xff value
        .push(calleeAccountAddressBytes)
        .push(100_000) // gas
        .op(CALL)
        // then we check balances for good measure
        .push(calleeAccountAddressBytes)
        .op(BALANCE)
        .op(SELFBALANCE);

    ToyAccount callerAccount =
        ToyAccount.builder()
            .address(callerAccountAddress)
            .nonce(127)
            .balance(Wei.of(1_000_000L))
            .code(callerAccountCode.compile())
            .build();

    Transaction transaction =
        ToyTransaction.builder()
            .sender(userAccount)
            .keyPair(keyPair)
            .to(callerAccount)
            .gasPrice(Wei.of(8L))
            .gasLimit(1_000_000L)
            .value(Wei.of(1L))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(userAccount, callerAccount, calleeAccount))
        .transaction(transaction)
        .build()
        .run();
  }

  /**
   * The following test does the following:
   *
   * <p>- create a transaction with payload that will later be interpreted as init code
   *
   * <p>- perform a full copy of the call data to RAM
   *
   * <p>- perform a <b>CREATE</b> operation, using the call data as init code
   *
   * <p>- the init code does an immediate <b>RETURN</b> / <b>REVERT</b> with zero size, huge offset
   *
   * <p>- back in the root context, check the <b>BALANCE</b> of the createe account (or 0) for good
   * measure
   *
   * @param opCode either {@link OpCode#RETURN} or {@link OpCode#REVERT}
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"RETURN", "REVERT"})
  void nonRootContextDeployment(OpCode opCode, TestInfo testInfo) {
    checkArgument(opCode.isAnyOf(RETURN, REVERT));

    BytecodeCompiler creatorAccountCode = BytecodeCompiler.newProgram(chainConfig);
    loadTheFullCallDataToRam(creatorAccountCode, 0);
    creatorAccountCode
        .op(CALLDATASIZE) // init code size
        .push(0) // init code offset
        .push(255) // 0xff value
        .op(CREATE)
        .op(BALANCE) // should return 255 in the RETURN case, 0 in the REVERT case
    ;

    ToyAccount creatorAccount =
        ToyAccount.builder()
            .balance(Wei.of(1_000_000L))
            .code(creatorAccountCode.compile())
            .nonce(891)
            .address(Address.fromHexString("c0dec0ffeef3fd"))
            .build();

    BytecodeCompiler payload = BytecodeCompiler.newProgram(chainConfig);
    zeroSizeReturnOrRevert(payload, opCode);
    Transaction transaction =
        ToyTransaction.builder()
            .sender(userAccount)
            .to(creatorAccount)
            .keyPair(keyPair)
            .gasPrice(Wei.of(8L))
            .gasLimit(1_000_000L)
            .value(Wei.of(1L))
            .payload(payload.compile()) // here: call data, later: init code
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(userAccount, creatorAccount))
        .transaction(transaction)
        .build()
        .run();
  }

  /**
   * @param opCode
   * @return
   */
  private void zeroSizeReturnOrRevert(BytecodeCompiler program, OpCode opCode) {
    checkArgument(opCode.isAnyOf(RETURN, REVERT));
    program
        .push(0) // zero size
        .push(hugeOffset) // huge offset
        .op(opCode);
  }

  public void loadTheFullCallDataToRam(BytecodeCompiler program, int targetOffset) {
    program.op(CALLDATASIZE).push(0).push(targetOffset).op(CALLDATACOPY);
  }
}
