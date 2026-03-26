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
package net.consensys.linea.zktracer.exceptions;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Fork.isPostShanghai;
import static net.consensys.linea.zktracer.Trace.EIP_3541_MARKER;
import static net.consensys.linea.zktracer.Trace.MAX_CODE_SIZE;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.getPgCreateWithInitCodeSize;
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
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.module.hub.signals.TracedException;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.AddressUtils;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class InvalidCodePrefixAndMaxCodeSizeExceptionTest extends TracerTestBase {

  // Here it is attempted to trigger the INVALID_CODE_PREFIX exception using a deployment
  // transaction (fails)
  @Test
  void invalidCodePrefixExceptionForDeploymentTransactionTest(TestInfo testInfo) {
    KeyPair keyPair = new SECP256K1().generateKeyPair();
    Address userAddress = Address.extract(keyPair.getPublicKey());
    ToyAccount userAccount =
        ToyAccount.builder().balance(Wei.fromEth(1000)).nonce(1).address(userAddress).build();

    BytecodeCompiler initProgram = BytecodeCompiler.newProgram(chainConfig);

    initProgram
        .push(Integer.toHexString(EIP_3541_MARKER))
        .push(0)
        .op(OpCode.MSTORE8)
        .push(1)
        .push(0)
        .op(OpCode.RETURN);

    Transaction tx =
        ToyTransaction.builder()
            .sender(userAccount)
            .keyPair(keyPair)
            .transactionType(TransactionType.FRONTIER)
            .gasLimit(0xffffffL)
            .payload(initProgram.compile())
            .build();

    Address deployedAddress = AddressUtils.effectiveToAddress(tx);
    System.out.println("Deployed address: " + deployedAddress);

    checkArgument(tx.isContractCreation());

    ToyExecutionEnvironmentV2 toyExecutionEnvironment =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount))
            .transaction(tx)
            .build();

    toyExecutionEnvironment.run();

    assertEquals(
        INVALID_CODE_PREFIX,
        toyExecutionEnvironment
            .getHub()
            .lastUserTransactionSection(1)
            .commonValues
            .tracedException());
  }

  // Here it is attempted to trigger the MAX_CODE_SIZE exception using a deployment transaction
  // (fails)
  @Test
  void maxCodeSizeExceptionForDeploymentTransactionTest(TestInfo testInfo) {
    KeyPair keyPair = new SECP256K1().generateKeyPair();
    Address userAddress = Address.extract(keyPair.getPublicKey());
    ToyAccount userAccount =
        ToyAccount.builder().balance(Wei.fromEth(1000)).nonce(1).address(userAddress).build();

    BytecodeCompiler initProgram = BytecodeCompiler.newProgram(chainConfig);

    initProgram.push(MAX_CODE_SIZE + 1).push(0).op(OpCode.RETURN);

    Transaction tx =
        ToyTransaction.builder()
            .sender(userAccount)
            .keyPair(keyPair)
            .transactionType(TransactionType.FRONTIER)
            .gasLimit(0xffffffL)
            .payload(initProgram.compile())
            .build();

    Address deployedAddress = AddressUtils.effectiveToAddress(tx);
    System.out.println("Deployed address: " + deployedAddress);

    checkArgument(tx.isContractCreation());

    ToyExecutionEnvironmentV2 toyExecutionEnvironment =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount))
            .transaction(tx)
            .build();

    toyExecutionEnvironment.run();

    assertEquals(
        TracedException.MAX_CODE_SIZE_EXCEPTION,
        toyExecutionEnvironment
            .getHub()
            .lastUserTransactionSection(1)
            .commonValues
            .tracedException());
  }

  // Here it is attempted to trigger the INVALID_CODE_PREFIX exception using a CREATE transaction
  // (success)
  @Test
  void invalidCodePrefixExceptionForCreateTest(TestInfo testInfo) {
    BytecodeCompiler initProgram = BytecodeCompiler.newProgram(chainConfig);
    initProgram
        .push(Integer.toHexString(EIP_3541_MARKER))
        .push(0)
        .op(OpCode.MSTORE8)
        .push(1)
        .push(0)
        .op(OpCode.RETURN);
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
    bytecodeRunner.run(chainConfig, testInfo);

    assertEquals(
        INVALID_CODE_PREFIX,
        bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
  }

  // Here it is attempted to trigger the MAX_CODE_SIZE exception using a CREATE transaction
  // (success)
  @Test
  void maxCodeSizeExceptionForCreateTest(TestInfo testInfo) {
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
    bytecodeRunner.run(chainConfig, testInfo);

    assertEquals(
        MAX_CODE_SIZE_EXCEPTION,
        bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
  }

  @ParameterizedTest
  @MethodSource("createOpCodesList")
  public void MaxCodeSizeExceptionWithInitCodeForCreatesTest(OpCode opCode, TestInfo testInfo) {
    // Dummy init code, repeats ADDRESS opcode
    Bytes32 initCodeChunk = Bytes32.fromHexString("30".repeat(32));

    // We now prepare a create program with an init code of (1537 * 32) byte size that will trigger
    // a Max code size exception
    BytecodeCompiler pg = getPgCreateWithInitCodeSize(opCode, initCodeChunk, 1537);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pg.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // (Post-Shanghai) MAX_CODE_SIZE_EXCEPTION happens
    TracedException exceptionTriggered = isPostShanghai(fork) ? MAX_CODE_SIZE_EXCEPTION : NONE;
    assertEquals(
        exceptionTriggered,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  static Stream<OpCode> createOpCodesList() {
    List<OpCode> opCodesListArgument = Arrays.asList(OpCode.CREATE, OpCode.CREATE2);
    return opCodesListArgument.stream();
  }
}
