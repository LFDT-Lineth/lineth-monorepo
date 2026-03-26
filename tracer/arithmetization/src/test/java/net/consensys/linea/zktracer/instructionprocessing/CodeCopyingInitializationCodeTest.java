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
package net.consensys.linea.zktracer.instructionprocessing;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
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

/**
 * The purpose of {@link CodeCopyingInitializationCodeTest} is to test the coherence between the
 * CODE_SIZE as expected by the zkEVM of an account under deployment, that is the initialization
 * code's size, and the codeSize contained in the account's snapshot.
 *
 * <p>We test both <b>deployment transactions</b> and <b>CREATE's</b>.
 *
 * <p>This test answers the point raised in <a
 * href="https://github.com/Consensys/linea-tracer/issues/1482">this issue</a>.
 */
@ExtendWith(UnitTestWatcher.class)
public class CodeCopyingInitializationCodeTest extends TracerTestBase {

  final Bytes initCodeSimple =
      BytecodeCompiler.newProgram(chainConfig)
          .op(OpCode.CODESIZE)
          .push(0)
          .push(0)
          .op(OpCode.CODECOPY)
          .compile();

  final Bytes initCodeWithMload =
      BytecodeCompiler.newProgram(chainConfig)
          .op(OpCode.CODESIZE)
          .push(0)
          .push(0)
          .op(OpCode.CODECOPY)
          .push(0)
          .op(OpCode.MLOAD)
          .compile();

  final Bytes initCodeDeploysItself =
      BytecodeCompiler.newProgram(chainConfig)
          .op(OpCode.CODESIZE)
          .push(0)
          .push(0)
          .op(OpCode.CODECOPY)
          .op(OpCode.CODESIZE)
          .push(0)
          .op(OpCode.RETURN)
          .compile();

  final Bytes deployerOfInitCodeSimple = deployerOf(initCodeSimple);
  final Bytes deployerOfInitCodeWithMload = deployerOf(initCodeWithMload);
  final Bytes deployerOfInitCodeDeploysItself = deployerOf(initCodeDeploysItself);

  /** We test <b>deployment transactions</b>. */
  @Test
  void testDeploymentTransactionCodeCopiesItself(TestInfo testInfo) {
    Transaction deploymentTransaction = deploymentTansactionFromInitCode(initCodeSimple);
    runTransaction(deploymentTransaction, testInfo);
  }

  @Test
  void testDeploymentTransactionCodeCopiesItselfAndFinishesOnMload(TestInfo testInfo) {
    Transaction deploymentTransaction = deploymentTansactionFromInitCode(initCodeWithMload);
    runTransaction(deploymentTransaction, testInfo);
  }

  @Test
  void testDeploymentTransactionDeploysOwnInitCodeThroughCodeCopy(TestInfo testInfo) {
    Transaction deploymentTransaction = deploymentTansactionFromInitCode(initCodeDeploysItself);
    runTransaction(deploymentTransaction, testInfo);
  }

  /** We test <b>CREATE's</b>. */
  @Test
  void testCreateContractFromInitCodeSimple(TestInfo testInfo) {
    Transaction messageCallTransaction =
        messageCallTransactionToDeployerAccount(accountInitCodeSimple);
    runTransaction(messageCallTransaction, testInfo);
  }

  @Test
  void testCreateContractFromInitCodeWithMload(TestInfo testInfo) {
    Transaction messageCallTransaction =
        messageCallTransactionToDeployerAccount(accountInitCodeWithMload);
    runTransaction(messageCallTransaction, testInfo);
  }

  @Test
  void testCreateContractFromInitCodeThatDeploysItself(TestInfo testInfo) {
    Transaction messageCallTransaction =
        messageCallTransactionToDeployerAccount(accountInitCodeThatDeploysItself);
    runTransaction(messageCallTransaction, testInfo);
  }

  KeyPair keyPair = new SECP256K1().generateKeyPair();
  Address userAddress = Address.extract(keyPair.getPublicKey());
  ToyAccount userAccount =
      ToyAccount.builder().balance(Wei.fromEth(100)).nonce(1).address(userAddress).build();
  ToyAccount accountInitCodeSimple =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(13)
          .address(Address.fromHexString("1337"))
          .code(deployerOfInitCodeSimple)
          .build();
  ToyAccount accountInitCodeWithMload =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(81)
          .address(Address.fromHexString("add7e550"))
          .code(deployerOfInitCodeWithMload)
          .build();
  ToyAccount accountInitCodeThatDeploysItself =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(255)
          .address(Address.fromHexString("69420"))
          .code(deployerOfInitCodeDeploysItself)
          .build();

  List<ToyAccount> accounts =
      List.of(
          userAccount,
          accountInitCodeWithMload,
          accountInitCodeSimple,
          accountInitCodeThatDeploysItself);

  Transaction deploymentTansactionFromInitCode(Bytes initCode) {
    return ToyTransaction.builder()
        .sender(userAccount)
        .payload(initCode)
        .transactionType(TransactionType.FRONTIER)
        .value(Wei.ZERO)
        .keyPair(keyPair)
        .gasLimit(100_000L)
        .gasPrice(Wei.of(8))
        .build();
  }

  Transaction messageCallTransactionToDeployerAccount(ToyAccount deployerAccount) {
    return ToyTransaction.builder()
        .sender(userAccount)
        .to(deployerAccount)
        .transactionType(TransactionType.FRONTIER)
        .value(Wei.ONE)
        .keyPair(keyPair)
        .gasLimit(100_000L)
        .gasPrice(Wei.of(8))
        .build();
  }

  private void runTransaction(Transaction transaction, TestInfo testInfo) {
    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(transaction)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  private Bytes deployerOf(Bytes initCode) {
    return BytecodeCompiler.newProgram(chainConfig)
        .push(initCode)
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(initCode.size()) // size
        .push(32 - initCode.size()) // offset
        .push(1234) // value
        .op(OpCode.CREATE)
        .op(OpCode.EXTCODESIZE) // get code size of newly deployed smart contract
        .compile();
  }
}
