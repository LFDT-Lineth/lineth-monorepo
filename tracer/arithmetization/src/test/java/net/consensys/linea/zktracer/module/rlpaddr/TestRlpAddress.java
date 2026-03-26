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

package net.consensys.linea.zktracer.module.rlpaddr;

import java.util.List;
import java.util.Random;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.module.rlpcommon.RlpRandEdgeCase;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class TestRlpAddress extends TracerTestBase {
  private final Random rnd = new Random(666);
  private final RlpRandEdgeCase util = new RlpRandEdgeCase();

  @Test
  void randDeployment(TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(keyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.of(100000000))
            .nonce(util.randLong())
            .address(senderAddress)
            .build();

    final Bytes initCode =
        BytecodeCompiler.newProgram(chainConfig).push(1).push(1).op(OpCode.SLT).compile();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .transactionType(TransactionType.FRONTIER)
            .value(Wei.ONE)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .payload(initCode)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .build()
        .run();
  }

  @Test
  void failingCreateTest(TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(keyPair.getPublicKey());

    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1000))
            .nonce(util.randLong())
            .address(senderAddress)
            .build();

    final Address contractAddress = Address.fromHexString("0x000bad000000b077000");
    final ToyAccount contractAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1000))
            .nonce(10)
            .address(contractAddress)
            .code(
                BytecodeCompiler.newProgram(chainConfig)

                    // copy the entirety of the call data to RAM
                    .op(OpCode.CALLDATASIZE)
                    .push(0)
                    .push(0)
                    .op(OpCode.CALLDATACOPY)
                    .op(OpCode.CALLDATASIZE)
                    .push(0)
                    .push(rnd.nextInt(0, 50000)) // value
                    .op(OpCode.CREATE)
                    .op(OpCode.STOP)
                    .compile())
            .build();

    final Bytes initCodeReturnContractCode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(contractAddress.getBytes())
            .op(OpCode.EXTCODESIZE)
            .op(OpCode.DUP1)
            .push(0)
            .push(0)
            .op(OpCode.DUP5) // should provoke stack underflow
            .op(OpCode.EXTCODECOPY)
            .push(0)
            .push(0)
            .op(OpCode.RETURN)
            .compile();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(contractAccount)
            .keyPair(keyPair)
            .transactionType(TransactionType.FRONTIER)
            .gasLimit(1000000L)
            .payload(initCodeReturnContractCode)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, contractAccount))
        .transaction(tx)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  @Test
  void improvedCreateTest(TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(keyPair.getPublicKey());

    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1000))
            .nonce(util.randLong())
            .address(senderAddress)
            .build();

    final Address contractAddress = Address.fromHexString("0x000bad000000b077000");
    final ToyAccount callDataDeployerAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1000))
            .nonce(10)
            .address(contractAddress)
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // copy the entirety of the call data to RAM
                    .op(OpCode.CALLDATASIZE)
                    .push(0)
                    .push(0)
                    .op(OpCode.CALLDATACOPY)
                    .op(OpCode.CALLDATASIZE)
                    .push(0)
                    .push(rnd.nextInt(0, 50000)) // value
                    .op(OpCode.CREATE)
                    .op(OpCode.STOP)
                    .compile())
            .build();

    final BytecodeCompiler copyAndReturnSomeForeignContractsCode =
        BytecodeCompiler.newProgram(chainConfig);
    fullCopyOfForeignByteCode(copyAndReturnSomeForeignContractsCode, contractAddress);
    appendReturn(copyAndReturnSomeForeignContractsCode, 0);

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(callDataDeployerAccount)
            .keyPair(keyPair)
            .transactionType(TransactionType.FRONTIER)
            .gasLimit(1000000L)
            .payload(copyAndReturnSomeForeignContractsCode.compile())
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, callDataDeployerAccount))
        .transaction(tx)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  public static void fullCopyOfForeignByteCode(BytecodeCompiler program, Address foreignAddress) {
    program
        .push(foreignAddress.getBytes())
        .op(OpCode.EXTCODESIZE) // foreign address code size
        .push(0) // source offset
        .push(0) // target offset
        .push(foreignAddress.getBytes()) // foreign address
        .op(OpCode.EXTCODECOPY) // copy
    ;
  }

  public static void appendReturn(BytecodeCompiler program, int rdo) {
    program.op(OpCode.MSIZE).push(rdo).op(OpCode.RETURN);
  }
}
