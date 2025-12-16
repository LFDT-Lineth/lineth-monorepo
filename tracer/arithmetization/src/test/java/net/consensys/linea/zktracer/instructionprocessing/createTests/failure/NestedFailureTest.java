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
package net.consensys.linea.zktracer.instructionprocessing.createTests.failure;

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.callCaller;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.identity.Tests.fullCodeCopyOf;
import static net.consensys.linea.zktracer.instructionprocessing.createTests.trivial.RootLevel.salt02;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

/**
 * The purpose of this class is to tests <b>Failure Condition F</b>'s arising from <i>nested</i>
 * <b>CREATE2</b>'s.
 */
@ExtendWith(UnitTestWatcher.class)
public class NestedFailureTest extends TracerTestBase {

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  public void nestedFailureTest(OpCode callOpCode, TestInfo testInfo) {

    final String deployedCode = "deadbeefc0de";
    final int deployedCodeLengthInBytes = deployedCode.length() / 2;
    final String errorString = "0e7707";
    final int errorStringLengthInBytes = errorString.length() / 2;

    BytecodeCompiler initCode = BytecodeCompiler.newProgram(chainConfig);
    callCaller(initCode, callOpCode, 0xffffff, 1, 0xa, 0xb, 0xc, 0xd);
    final int sizeUpToCall = initCode.compile().size();
    initCode
        .op(ISZERO)
        .push(sizeUpToCall + (1 + 2 + 1) + ((1 + deployedCodeLengthInBytes) + 2 + 1 + 2 + 2 + 1))
        .op(JUMPI)
        //////////////////////////
        // successful CALL path //
        //////////////////////////
        .push(deployedCode)
        .push(0)
        .op(MSTORE)
        .push(deployedCodeLengthInBytes)
        .push(WORD_SIZE - deployedCodeLengthInBytes)
        .op(RETURN)
        ////////////////////////////
        // unsuccessful CALL path //
        ////////////////////////////
        .op(JUMPDEST)
        .push(errorString)
        .push(0)
        .op(MSTORE)
        .push(errorStringLengthInBytes) // size
        .push(WORD_SIZE - errorStringLengthInBytes) // offset
        .op(REVERT);

    ToyAccount accountContainingInitCode =
        ToyAccount.builder()
            .address(Address.fromHexString("c0ffeec0deadd7"))
            .code(initCode.compile())
            .nonce(0xffff)
            .balance(Wei.of(0xffffff))
            .build();

    BytecodeCompiler entryPoint = BytecodeCompiler.newProgram(chainConfig);
    fullCodeCopyOf(entryPoint, accountContainingInitCode); // loading init code into memory
    entryPoint
        .push(salt02) // salt
        .op(MSIZE) // size
        .push(0) // offset
        .op(SELFBALANCE) // value
        .op(CREATE2);

    ToyAccount entryPointAccount =
        ToyAccount.builder()
            .address(Address.fromHexString("add7e55"))
            .balance(Wei.of(0xeeeeee))
            .nonce(0x777777)
            .code(entryPoint.compile())
            .build();

    Transaction transaction =
        ToyTransaction.builder()
            .sender(userAccount)
            .keyPair(keyPair)
            .to(entryPointAccount)
            .value(Wei.of(7_000_000_000L))
            .gasLimit(0xffffffL)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(userAccount, entryPointAccount, accountContainingInitCode))
        .transaction(transaction)
        .build()
        .run();
  }
}
