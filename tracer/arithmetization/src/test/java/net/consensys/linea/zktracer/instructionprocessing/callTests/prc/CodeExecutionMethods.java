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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc;

import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.*;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.*;
import static net.consensys.linea.zktracer.opcode.OpCode.CALL;
import static net.consensys.linea.zktracer.opcode.OpCode.GAS;
import static org.hyperledger.besu.datatypes.TransactionType.FRONTIER;

import java.util.List;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.ChainConfig;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.TestInfo;

/**
 * The following class provides methods to run code in the following contexts:
 *
 * <p>- <b>MESSAGE_CALL_TRANSACTION</b>: at depth 0 in the root context of a message call
 * transaction
 *
 * <p>- <b>CONTRACT_DEPLOYMENT_TRANSACTION</b>: at depth 0 in the root context of a contract
 * deployment transaction
 *
 * <p>- <b>MESSAGE_CALL_FROM_ROOT</b>: at depth 1 as the byte code executed in a <b>CALL</b> to a
 * SMC
 *
 * <p>- <b>DURING_DEPLOYMENT</b>: at depth 1 as the init code of a <b>CREATE</b>
 *
 * <p>- <b>AFTER_DEPLOYMENT</b>: at depth 1, after deploying it with a <b>CREATE</b>, as the byte
 * code executed in a <b>CALL</b> to the newly deployed contract
 */
public class CodeExecutionMethods {

  public static final Address rootAddress = Address.fromHexString("7007");
  public static final ToyAccount.ToyAccountBuilder root =
      ToyAccount.builder().address(rootAddress).balance(Wei.of(0xff1122ccL)).nonce(1865);

  public static final Address chadPrcEnjoyerAddress = Address.fromHexString("cbad");
  public static final ToyAccount.ToyAccountBuilder chadPrcEnjoyer =
      ToyAccount.builder().address(chadPrcEnjoyerAddress).balance(Wei.of(0xff003300L)).nonce(64);

  public static final Address initCodeOwnerAddress = Address.fromHexString("1717");
  public static final ToyAccount.ToyAccountBuilder initCodeOwner =
      ToyAccount.builder().address(initCodeOwnerAddress).balance(Wei.of(0xff1337L)).nonce(127);

  public static final Address foreignCodeOwnerAddress = Address.fromHexString("f00d");
  public static final ToyAccount.ToyAccountBuilder foreignCodeOwner =
      ToyAccount.builder().address(foreignCodeOwnerAddress).balance(Wei.of(0xff1789L)).nonce(255);

  public static final Address memoryContentsHolderAddress1 = Address.fromHexString("d00d");
  public static final ToyAccount.ToyAccountBuilder memoryContentsHolder1 =
      ToyAccount.builder()
          .address(memoryContentsHolderAddress1)
          .balance(Wei.of(0xff2025L))
          .nonce(0x11aaff);

  public static final Address memoryContentsHolderAddress2 = Address.fromHexString("dada");
  public static final ToyAccount.ToyAccountBuilder memoryContentsHolder2 =
      ToyAccount.builder()
          .address(memoryContentsHolderAddress2)
          .balance(Wei.of(0xff1776L))
          .nonce(0x11aabb);

  public static final ToyTransaction.ToyTransactionBuilder transaction =
      ToyTransaction.builder()
          .sender(userAccount)
          .keyPair(keyPair)
          .transactionType(FRONTIER)
          .gasLimit(0xffffffL)
          .value(Wei.of(1_000_000_000L));

  /**
   * Construct transaction with {@code transactionInitCode} as its init code.
   *
   * @param rootCode
   * @param testInfo
   */
  public static void runMessageCallTransactionWithProvidedCodeAsRootCode(
      BytecodeCompiler rootCode, ChainConfig chainConfig, TestInfo testInfo) {

    root.code(rootCode.compile());

    transaction.to(root.build());

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .transaction(transaction.build())
        .accounts(listOfAccounts())
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  /**
   * Construct transaction with {@code transactionInitCode} as its init code.
   *
   * @param transactionInitCode
   * @param testInfo
   */
  public static void runDeploymentTransactionWithProvidedCodeAsInitCode(
      BytecodeCompiler transactionInitCode, ChainConfig chainConfig, TestInfo testInfo) {

    transaction.payload(transactionInitCode.compile()); // init code

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .transaction(transaction.build())
        .accounts(listOfAccounts())
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  /**
   * - We provide {@link CodeExecutionMethods#foreignCodeOwner} with {@code foreignCode} as its byte
   * code
   *
   * <p>- We provide {@link CodeExecutionMethods#root} with byte code that <b>(a)</b> copies the
   * code of {@link CodeExecutionMethods#foreignCodeOwner} to RAM, <b>(b)</b> runs it as the init
   * code of a <b>CREATE</b> and <b>(c)</b> optionally reverts.
   *
   * @param foreignCode
   * @param embedRevertIntoInitCode
   * @param testInfo
   */
  public static void runForeignByteCodeAsInitCode(
      BytecodeCompiler foreignCode,
      boolean embedRevertIntoInitCode,
      ChainConfig chainConfig,
      TestInfo testInfo) {

    foreignCodeOwner.code(foreignCode.compile());

    BytecodeCompiler rootCode = BytecodeCompiler.newProgram(chainConfig);
    copyForeignCodeAndRunItAsInitCode(rootCode, foreignCodeOwnerAddress);
    if (embedRevertIntoInitCode) revertWith(rootCode, 0, 0);
    root.code(rootCode.compile());

    transaction.to(root.build());

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(listOfAccounts())
        .transaction(transaction.build())
        .build()
        .run();
  }

  /**
   * - We provide {@link CodeExecutionMethods#chadPrcEnjoyer} with {@code providedCode} as its byte
   * code
   *
   * <p>- We provide {@code root} with byte code that calls {@link
   * CodeExecutionMethods#chadPrcEnjoyer} and optionally reverts.
   *
   * @param providedCode
   * @param revertRoot
   * @param testInfo
   */
  public static void runMessageCallToAccountEndowedWithProvidedCode(
      BytecodeCompiler providedCode,
      boolean revertRoot,
      ChainConfig chainConfig,
      TestInfo testInfo) {

    chadPrcEnjoyer.code(providedCode.compile());

    BytecodeCompiler rootCode = BytecodeCompiler.newProgram(chainConfig);
    appendCallTo(rootCode, CALL, chadPrcEnjoyerAddress);
    if (revertRoot) revertWith(rootCode, 0, 0); // we let the ROOT revert
    root.code(rootCode.compile());

    transaction.to(root.build());

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(listOfAccounts())
        .transaction(transaction.build())
        .build()
        .run();
  }

  /**
   * - We provide {@link CodeExecutionMethods#root} with byte code that (<b>a</b>) copies {@link
   * CodeExecutionMethods#initCodeOwner}'s code and runs it as the init code of a <b>CREATE</b>
   * (<b>b</b>) <b>CALL</b>'s into the newly deployed contract (<b>c</b>) and optionally reverts.
   *
   * <p>- We provide {@link CodeExecutionMethods#initCodeOwner} with byte code that copies the code
   * of {@link CodeExecutionMethods#foreignCodeOwnerAddress} and <b>RETURN</b>'s it.
   *
   * <p>- We provide {@link CodeExecutionMethods#foreignCodeOwner} with {@code foreignCode} as its
   * byte code.
   *
   * @param foreignCode
   * @param rootReverts
   * @param testInfo
   */
  public static void runCreateDeployingForeignCodeAndCallIntoIt(
      BytecodeCompiler foreignCode,
      boolean rootReverts,
      ChainConfig chainConfig,
      TestInfo testInfo) {

    // ROOT code
    int key = 65537; // 0x 01 00 01
    BytecodeCompiler rootCode = BytecodeCompiler.newProgram(chainConfig);
    copyForeignCodeAndRunItAsInitCode(rootCode, initCodeOwnerAddress);
    sstoreTopOfStackTo(rootCode, key); // store deployment address
    pushSeveral(rootCode, 0, 0, 0, 0, 0); // zero value
    sloadFrom(rootCode, key);
    rootCode.op(GAS).op(CALL); // call the deployed contract
    if (rootReverts) revertWith(rootCode, 0, 0);
    root.code(rootCode.compile());

    // init code owner code
    BytecodeCompiler initCode = BytecodeCompiler.newProgram(chainConfig);
    copyForeignCodeAndReturnIt(initCode, foreignCodeOwnerAddress);
    initCodeOwner.code(initCode.compile());

    // foreign code owner code
    foreignCodeOwner.code(foreignCode.compile());

    transaction.to(root.build());

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(listOfAccounts())
        .transaction(transaction.build())
        .build()
        .run();
  }

  private static List<ToyAccount> listOfAccounts() {
    return List.of(
        userAccount,
        root.build(),
        initCodeOwner.build(),
        foreignCodeOwner.build(),
        chadPrcEnjoyer.build(),
        memoryContentsHolder1.build(),
        memoryContentsHolder2.build());
  }
}
