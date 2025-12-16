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

import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.*;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.OUT_OF_GAS_EXCEPTION;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static org.junit.jupiter.api.Assertions.*;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

@ExtendWith(UnitTestWatcher.class)
public class OutOfGasMemExpExceptionTest extends TracerTestBase {
  /**
   * Trigger out of gas exception for a MSTORE operation with a gas limit that is too low to cover
   * the memory expansion
   */
  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionMStore(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0xFF")) // value
        .push(0) // offset
        .op(OpCode.MSTORE);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  /**
   * Trigger out of gas exception for a MSTORE8 operation with a gas limit that is too low to cover
   * the memory expansion
   */
  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionMStore8(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0xFF")) // value
        .push(0) // offset
        .op(OpCode.MSTORE8);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  /**
   * Trigger out of gas exception for a MLOAD operation with a gas limit that is too low to cover
   * its memory expansion
   */
  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionMLoad(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(
            Bytes.fromHexString(
                "0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")) // value
        .push(0) // offset
        .op(OpCode.MSTORE);

    program
        .push(17) // value
        .op(OpCode.MLOAD);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  /**
   * the target offset in RAM is set to <b>OFF + 2</b> with OFF ≡ 6848. This is used to overcome the
   * call data floor price (starting with PRAGUE) in the final computation of the gas cost of the
   * whole transaction. The floor price does no longer overshadow the memory expansion cost, and so
   * we can trigger the desired <b>OUT_OF_GAS_EXCEPTION</b>.
   *
   * <ul>
   *   <li>works with <b>OFF</b> = 6848
   *   <li>fails with <b>OFF</b> = 6847
   * </ul>
   */
  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionCallDataCopy(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    Bytes calldata =
        Bytes.fromHexString("0x7FFFFFFFFFFFFF00FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");
    program
        .push(31) // size
        .push(1) // offset in call data
        .push(6848 + 2) // offset in RAM, triggers memory expansion
        .op(CALLDATACOPY);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(calldata, chainConfig, testInfo);

    bytecodeRunner.run(
        Wei.fromEth(1), gasCost + cornerCase, List.of(), calldata, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionCodeCopy(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0xFA")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(3) // size
        .push(0) // offset
        .push(32) // destoffset
        .op(OpCode.CODECOPY); // Should copy 60fa60 (first 3 bytes of the code)

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionWarmExtCodeCopy(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        // constructor
        .push(
            Bytes.fromHexString(
                "0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(
            Bytes.fromHexString(
                "0xFF60005260206000F30000000000000000000000000000000000000000000000")) // value
        .push(32) // offset
        .op(OpCode.MSTORE)
        // Create the contract
        .push(41)
        .push(0)
        .push(0)
        .op(OpCode.CREATE) // Address is warm
        .push(32) // size
        .push(0) // offset
        .push(33) // destoffset
        .op(OpCode.DUP4) //
        .op(OpCode.EXTCODECOPY);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {0})
  void outOfGasExceptionColdExtCodeCopy(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    final int foreignCodeSize = 70;
    final ToyAccount codeOwnerAccount =
        getAccountForAddressWithBytecode(
            codeAddress, Bytes.fromHexString("ff".repeat(foreignCodeSize)));

    program
        .push(foreignCodeSize + 3) // size
        .push(11) // offset
        .push(33) // destoffset
        .push("c0de") // Address is cold
        .op(OpCode.EXTCODECOPY);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost =
        bytecodeRunner.runOnlyForGasCost(List.of(codeOwnerAccount), chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, List.of(codeOwnerAccount), chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionReturn(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(
            Bytes.fromHexString(
                "0xF00000000000000000000000000000000000000000000000000000000000FFAA")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(3) // size
        .push(30) // offset
        .op(OpCode.RETURN);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionReturnDataCopy(int cornerCase, TestInfo testInfo) {

    final ToyAccount returnDataProviderAccount =
        getAccountForAddressWithBytecode(codeAddress, return32BytesFFBytecode);

    boolean RDCX = false;
    boolean MXPX = false;
    BytecodeCompiler program = getProgramRDCFromStaticCallToCodeAccount(RDCX, MXPX);
    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost =
        bytecodeRunner.runOnlyForGasCost(List.of(returnDataProviderAccount), chainConfig, testInfo);

    bytecodeRunner.run(
        gasCost + cornerCase, List.of(returnDataProviderAccount), chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionRevert(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(
            Bytes.fromHexString(
                "0xF00000000000000000000000000000000000000000000000000000000000FFAA")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(3) // value
        .push(30) // offset
        .op(OpCode.REVERT);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-6420, -6419, -6418, 100, 101, 102})
  /*
  Deployment code: "0x7F7EFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF60005260206000F3"
  1. OOGX for CREATE before deployment: remove 6400 (depositFee) + deployment code exec cost (18)
  2. OOGX for CREATE after deployment: enough gas for child creation, but not enough to complete deployment code or deposit
  3. No OOGX for CREATE after deployment: add 1/64th of 6418 to gasCost to account for gasAvailableForChildCreate
   */
  void outOfGasExceptionCreate(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler programInitCodeToMem = getPgPushInitCodeToMem();
    programInitCodeToMem
        // Create the contract
        .push(41)
        .push(0)
        .push(0)
        .op(OpCode.CREATE); // No constructor so code executed and runtime code set to return value

    Bytes pgCompile = programInitCodeToMem.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    if (cornerCase <= -6419) {
      assertEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    } else if (cornerCase <= 100) {
      assertNotEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
      assertEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
    } else {
      assertNotEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
      assertNotEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection(2).commonValues.tracedException());
    }
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionLog0(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0x7F")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(32) // size
        .push(1) // offset to trigger mem expansion
        .op(OpCode.LOG0);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {0})
  void outOfGasExceptionLog1(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0x7F")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(topic1) // Topic 1
        .push(32) // size
        .push(1) // offset to trigger mem expansion
        .op(OpCode.LOG1);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionLog2(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0x7F")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(topic2) // Topic 2
        .push(topic1) // Topic 1
        .push(32) // size
        .push(1) // offset to trigger mem expansion
        .op(OpCode.LOG2);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionLog3(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0x7F")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(topic3) // Topic 3
        .push(topic2) // Topic 2
        .push(topic1) // Topic 1
        .push(32) // size
        .push(1) // offset to trigger mem expansion
        .op(OpCode.LOG3);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionLog4(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push(Bytes.fromHexString("0x7F")) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(topic4) // Topic 4
        .push(topic3) // Topic 3
        .push(topic2) // Topic 2
        .push(topic1) // Topic 1
        .push(32) // size
        .push(1) // offset to trigger mem expansion
        .op(OpCode.LOG4);

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }

  @ParameterizedTest
  @ValueSource(ints = {-1, 0, 1})
  void outOfGasExceptionMCopy(int cornerCase, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    try {
      program
          .push(Bytes.fromHexString("0x7F")) // value
          .push(32) // offset
          .op(OpCode.MSTORE)
          .push(32) // size
          .push(32) // offset to trigger mem expansion
          .push(0) // dest offset
          .op(OpCode.MCOPY);
    } catch (IllegalArgumentException e) {
      // MCOPY is not supported prior to Cancun fork
      return;
    }

    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);

    long gasCost = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    bytecodeRunner.run(gasCost + cornerCase, chainConfig, testInfo);

    ExceptionUtils.assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
        cornerCase, bytecodeRunner);
  }
}
