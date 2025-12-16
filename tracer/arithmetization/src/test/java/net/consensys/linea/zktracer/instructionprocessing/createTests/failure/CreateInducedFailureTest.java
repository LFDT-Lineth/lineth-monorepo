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

import static net.consensys.linea.testing.ToyTransaction.ToyTransactionBuilder;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.createTests.trivial.RootLevel.salt01;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyMultiTransaction;
import net.consensys.linea.testing.ToyTransaction;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/**
 * The present test case tests the ZKEVM's reaction to triggering the <b>Failure Condition F</b>
 * with a <b>CREATE</b> opcode. It's not straightforward. The sequence of events is as follows:
 *
 * <p>- we start with an account {@link #entryPoint};
 *
 * <p>- in {@link #transactionBuilderDeployingDelegateCaller} we call {@link #entryPoint} with empty
 * call data which leads to the deployment of {@link #delegateCaller};
 *
 * <p>- in {@link #transactionBuilderLeadingDelegateCallerToCreateAnAccount} we call {@link
 * #entryPoint} with nonempty call data which leads it calling {@link #delegateCaller} which leads
 * to it doing a <b>DELEGATECALL</b> to {@link #simpleCreator} thus deploying a new account with
 * {@link #delegateCaller}'s nonce =1;
 *
 * <p>- in {@link #transactionBuilderLeadingDelegateCallerToSelfDestruct} we call {@link
 * #entryPoint} with nonempty call data which leads it calling {@link #delegateCaller} which leads
 * to it doing a <b>DELEGATECALL</b> to {@link #simpleSelfDestructor} self destructing;
 *
 * <p>- in {@link #transactionBuilderDeployingDelegateCallerAgain} we call {@link #entryPoint} with
 * empty call data again which leads to the deployment of {@link #delegateCaller} <i>again</i>;
 *
 * <p>- in {@link
 * #transactionBuilderLeadingDelegateCallerToAttemptCreateAgainThusRaisingFailureConditionF} we call
 * {@link #entryPoint} with nonempty call data which leads it calling {@link #delegateCaller} which
 * leads to it doing a <b>DELEGATECALL</b> to {@link #simpleCreator} thus <i>attempting</i> to
 * redeploy at the same address where it did the first deployment; indeed {@link #delegateCaller}'s
 * nonce is again =1; deploying a new account at nonce 1;
 */
@ExtendWith(UnitTestWatcher.class)
public class CreateInducedFailureTest extends TracerTestBase {

  final Bytes tinyAddress1 = Bytes.fromHexString("badd1ec0de");
  final Bytes tinyAddress2 = Bytes.fromHexString("900d1ec0de");
  final Address address1 = Address.wrap(leftPadTo(tinyAddress1, 20));
  final Address address2 = Address.wrap(leftPadTo(tinyAddress2, 20));
  final Bytes leftPaddedAddress1 = leftPadTo(tinyAddress1, WORD_SIZE);
  final Bytes leftPaddedAddress2 = leftPadTo(tinyAddress2, WORD_SIZE);
  final Address targetAddress = Address.fromHexString("797add7e55");

  final BytecodeCompiler simpleSelfDestruct =
      BytecodeCompiler.newProgram(chainConfig).op(ORIGIN).op(SELFDESTRUCT);

  /**
   * Account that can only do one thing: do a <b>SELFDESTRUCT</b> sending the funds to the
   * <b>ORIGIN</b>.. Its <i>raison d'être</i> is to be called by the {@link #delegateCaller}.
   */
  final ToyAccount simpleSelfDestructor =
      ToyAccount.builder()
          .code(simpleSelfDestruct.compile())
          .nonce(91)
          .balance(Wei.of(1234L))
          .address(address1)
          .build();

  final BytecodeCompiler simpleCreate =
      BytecodeCompiler.newProgram(chainConfig)
          .push(0) // empty init code
          .push(0)
          .push(1) // value
          .op(CREATE);

  /**
   * Account that can only do one thing: do a <b>CREATE</b> with 1 Wei of value. Its <i>raison
   * d'être</i> is to be called by the {@link #delegateCaller}.
   */
  final ToyAccount simpleCreator =
      ToyAccount.builder()
          .code(simpleCreate.compile())
          .nonce(512)
          .balance(Wei.of(73L))
          .address(address2)
          .build();

  /** Does a <b>DELEGATECALL</b> to an address extracted from the call data. */
  final BytecodeCompiler delegateCaller =
      BytecodeCompiler.newProgram(chainConfig)
          .push(0) // rac
          .push(0) // rao
          .push(0) // cds
          .push(0) // cdo
          .push(0)
          .op(CALLDATALOAD) // should be an address
          .op(GAS)
          .op(DELEGATECALL);

  /** Initialization code that deploys {@link #delegateCaller}. */
  final BytecodeCompiler initCode =
      BytecodeCompiler.newProgram(chainConfig)
          .push(delegateCaller.compile())
          .push(8 * (32 - delegateCaller.compile().size()))
          .op(SHL)
          .push(0)
          .op(MSTORE)
          .push(delegateCaller.compile().size()) // code size
          .push(0)
          .op(RETURN);

  int keyToDeploymentAddress = 0xad;
  int emptyCallDataExecutionPathProgramCounter = 27;

  /**
   * This bytecode behaves in two different possible ways:
   *
   * <p>- if called with empty call data it performs a predetermined <b>CREATE2</b> using {@link
   * #initCode} as initialization code, which deploys {@link #delegateCaller}, and then stores the
   * deployment address at key {@link #keyToDeploymentAddress};
   *
   * <p>- if called with nonempty call data it writes the call data into memory, <b>SLOAD</b>'s an
   * item from storage (the aforementioned <b>CREATE2</b> deployment address stored at {@link
   * #keyToDeploymentAddress}), and performs a <b>CALL</b> to said address providing it with the
   * same call data it was given itself; this assumes that call data fit into a single EVM word;
   *
   * <p>Thus the {@link #entryPoint} serves as the entry point for the {@link #delegateCaller}
   * account
   *
   * <p><b>N.B.</b> For the <b>SLOAD</b> to work as expected it is necessary that the <b>CREATE2</b>
   * step have been taken first. Otherwise, it will <b>SLOAD</b> the value <b>0x00 ... 00</b> which
   * is of no use.
   */
  final BytecodeCompiler entryPointByteCode =
      BytecodeCompiler.newProgram(chainConfig)
          .op(CALLDATASIZE) // + 1
          .op(ISZERO) // [CALLDATASIZE == 0] // + 1
          .push(emptyCallDataExecutionPathProgramCounter) // + 2
          .op(JUMPI) // + 1
          ////////////////////////////////////////////
          // The nonempty call data execution route //
          ////////////////////////////////////////////
          .push(0) // + 2
          .op(CALLDATALOAD) // extracts (address) from call data // + 1
          .push(0) // + 2
          .op(MSTORE) // + 1
          .push(0) // rac // + 2
          .push(0) // rao // + 2
          .push(0x20) // cds // + 2
          .push(0) // cdo // + 2
          .push(keyToDeploymentAddress) // + 2
          .op(SLOAD) // extract stored address // + 1
          .push(0xef) // value // + 2
          .op(GAS) // gas // + 1
          .op(CALL) // + 1
          .op(STOP) // + 1
          /////////////////////////////////////////
          // The empty call data execution route //
          /////////////////////////////////////////
          .op(JUMPDEST) // PC = 27
          .push(initCode.compile())
          .push(8 * (32 - initCode.compile().size()))
          .op(SHL)
          .push(0)
          .op(MSTORE)
          .push(salt01)
          .push(initCode.compile().size()) // init code size
          .push(0) // offset
          .push(0xff) // value
          .op(CREATE2)
          .push(keyToDeploymentAddress)
          .op(SSTORE);

  final ToyAccount entryPoint =
      ToyAccount.builder()
          .code(entryPointByteCode.compile())
          .nonce(1337)
          .balance(Wei.of(73L))
          .address(targetAddress)
          .build();

  final ToyTransactionBuilder transactionBuilderDeployingDelegateCaller =
      ToyTransaction.builder()
          .to(entryPoint)
          .keyPair(keyPair)
          .value(Wei.of(0xffff))
          .gasLimit(1_000_000L)
          .gasPrice(Wei.of(8));

  final ToyTransactionBuilder transactionBuilderLeadingDelegateCallerToCreateAnAccount =
      ToyTransaction.builder()
          .to(entryPoint)
          .keyPair(keyPair)
          .value(Wei.of(0xeeee))
          .gasLimit(1_000_000L)
          .gasPrice(Wei.of(8))
          .payload(leftPaddedAddress1);

  final ToyTransactionBuilder transactionBuilderLeadingDelegateCallerToSelfDestruct =
      ToyTransaction.builder()
          .to(entryPoint)
          .keyPair(keyPair)
          .value(Wei.of(0xdddd))
          .gasLimit(1_000_000L)
          .gasPrice(Wei.of(8))
          .payload(leftPaddedAddress2);

  final ToyTransactionBuilder transactionBuilderDeployingDelegateCallerAgain =
      ToyTransaction.builder()
          .to(entryPoint)
          .keyPair(keyPair)
          .value(Wei.of(0xcccc))
          .gasLimit(1_000_000L)
          .gasPrice(Wei.of(8));

  final ToyTransactionBuilder
      transactionBuilderLeadingDelegateCallerToAttemptCreateAgainThusRaisingFailureConditionF =
          ToyTransaction.builder()
              .to(entryPoint)
              .keyPair(keyPair)
              .value(Wei.of(0xbbbb))
              .gasLimit(1_000_000L)
              .gasPrice(Wei.of(8))
              .payload(leftPaddedAddress1);

  final List<ToyTransactionBuilder> toyTransactionBuilders =
      List.of(
          transactionBuilderDeployingDelegateCaller,
          transactionBuilderLeadingDelegateCallerToCreateAnAccount,
          transactionBuilderLeadingDelegateCallerToSelfDestruct,
          transactionBuilderDeployingDelegateCallerAgain,
          transactionBuilderLeadingDelegateCallerToAttemptCreateAgainThusRaisingFailureConditionF);

  final List<Transaction> transactions =
      ToyMultiTransaction.builder().build(toyTransactionBuilders, userAccount);

  final List<ToyAccount> accounts =
      List.of(userAccount, entryPoint, simpleSelfDestructor, simpleCreator);

  @Test
  void complexFailureConditionTest(TestInfo testInfo) {

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transactions(transactions)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }
}
