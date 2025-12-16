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
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.OUT_OF_GAS_EXCEPTION;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;

public class ExceptionUtils extends TracerTestBase {

  public static Address codeAddress = Address.fromHexString("c0de");
  public static Bytes topic1 =
      Bytes.fromHexString("0x1FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");
  public static Bytes topic2 =
      Bytes.fromHexString("0x2FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");
  public static Bytes topic3 =
      Bytes.fromHexString("0x3FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");
  public static Bytes topic4 =
      Bytes.fromHexString("0x4FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");
  public static Bytes salt =
      Bytes.fromHexString("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef");
  public static Bytes return32BytesFFBytecode =
      Bytes.fromHexString(
          "7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff60005260206000f3");

  static void assertEqualsOutOfGasIfCornerCaseMinusOneElseAssertNotEquals(
      int cornerCase, BytecodeRunner bytecodeRunner) {
    if (cornerCase == -1) {
      assertEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    } else {
      assertNotEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    }
  }

  public static ToyAccount getAccountForAddressWithBytecode(Address addr, Bytes bytecode) {
    return ToyAccount.builder()
        .balance(Wei.fromEth(1))
        .nonce(10)
        .address(addr)
        .code(bytecode)
        .build();
  }

  public static BytecodeCompiler getProgramStaticCallToCodeAddress(int gas) {
    return BytecodeCompiler.newProgram(chainConfig)
        .push(0) // byte size of return data
        .push(0) // retOffset
        .push(0) // byte size calldata
        .push(0) // argsOffset
        .push("c0de") // Address of account
        .push(gas) // gas
        .op(OpCode.STATICCALL);
  }

  public static BytecodeCompiler getProgramStaticCallToCodeAccount() {
    return BytecodeCompiler.newProgram(chainConfig)
        .push(0) // byte size of return data
        .push(0) // retOffset
        .push(0) // byte size calldata
        .push(0) // argsOffset
        .push("c0de") // Address of account
        .op(OpCode.GAS) // gas
        .op(OpCode.STATICCALL);
  }

  public static BytecodeCompiler getProgramRDCFromStaticCallToCodeAccount(
      boolean withRDCX, boolean withMXPX) {
    // if withMXPX, we set an offset to trigger MXPX else regular MXP
    Bytes offsetRDC =
        withMXPX
            ? Bytes.fromHexStringLenient("0x0100000000")
            : Bytes.ofUnsignedLong(65).trimLeadingZeros();
    // 1. Execute static call
    BytecodeCompiler programStartWithStaticCall = getProgramStaticCallToCodeAccount();
    // 2. Clean the stack
    programStartWithStaticCall.op(OpCode.POP).op(OpCode.RETURNDATASIZE);
    // if withRDCX is true, we add the code to trigger the exception
    // 3. Trigger exceptional return data copy
    if (withRDCX) {
      programStartWithStaticCall
          .push(1)
          .op(OpCode.ADD); // size = RDS + 1, which will trigger the `returnDataCopyException`
    }
    programStartWithStaticCall
        .push(0) // offset
        .push(offsetRDC) // destoffset, trigger mem expansion
        .op(OpCode.RETURNDATACOPY);
    return programStartWithStaticCall;
  }

  public static BytecodeCompiler getPgPushInitCodeToMem() {
    Bytes initCodePart1 =
        Bytes.fromHexString("0x7F7EFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");
    Bytes initCodePart2 =
        Bytes.fromHexString("0xFF60005260206000F30000000000000000000000000000000000000000000000");

    return BytecodeCompiler.newProgram(chainConfig)
        .push(initCodePart1) // value
        .push(0) // offset
        .op(OpCode.MSTORE)
        .push(initCodePart2) // value
        .push(32) // offset
        .op(OpCode.MSTORE);
  }

  public static BytecodeCompiler simpleProgram(OpCode opCode) {
    BytecodeCompiler program =
        (opCode == OpCode.CREATE || opCode == OpCode.CREATE2)
            ? getPgPushInitCodeToMem()
            : BytecodeCompiler.newProgram(chainConfig);
    switch (opCode) {
      case LOG0 ->
          program
              .push(32) // size
              .push(1) //  offset
              .op(OpCode.LOG0);
      case LOG1 ->
          program
              .push(topic1) // Topic 1
              .push(32) // size
              .push(1) // offset to trigger mem expansion
              .op(OpCode.LOG1);
      case LOG2 ->
          program
              .push(topic2) // Topic 2
              .push(topic1) // Topic 1
              .push(32) // size
              .push(1) // offset to trigger mem expansion
              .op(OpCode.LOG2);
      case LOG3 ->
          program
              .push(topic3) // Topic 3
              .push(topic2) // Topic 2
              .push(topic1) // Topic 1
              .push(32) // size
              .push(1) // offset to trigger mem expansion
              .op(OpCode.LOG3);
      case LOG4 ->
          program
              .push(topic4) // Topic 4
              .push(topic3) // Topic 3
              .push(topic2) // Topic 2
              .push(topic1) // Topic 1
              .push(32) // size
              .push(1) // offset to trigger mem expansion
              .op(OpCode.LOG4);
      case SSTORE, TSTORE ->
          program
              .push(2) // value
              .push(1) // key
              .op(opCode);
      case SELFDESTRUCT -> program.push(0).op(OpCode.SELFDESTRUCT);
      case CREATE ->
          program
              // Create the contract
              .push(41)
              .push(0)
              .push(0)
              .op(OpCode.CREATE);
      case CREATE2 ->
          program
              // Create the contract
              .push(salt) // salt
              .push(41)
              .push(0)
              .push(0)
              .op(OpCode.CREATE2);
      default -> {}
    }

    return program;
  }

  /**
   * The {@code getInitCodeWithSize} pushes an init code program in memory with a byte size a
   * multiple of 32. Byte size of the init code program is 32 * {@code nbChunks}. There are 2 cases
   * of interest:
   *
   * <ul>
   *   <li>{@code nbChunks} is > 0 and <= 1536.
   *   <li>{@code nbChunks} is > 1536.
   * </ul>
   *
   * Only the second one should raise the <b>maxCodeSizeException</b> when used in a CREATE or
   * CREATE2. Indeed EIP-3860 limits and meters init code at ( 32 * 1536 ) = 49152.
   */
  public static BytecodeCompiler getInitCodeWithSize(Bytes32 initCodeChunk, int nbChunks) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    int decrNbChunks = nbChunks;

    while (decrNbChunks > 0) {
      program
          .push(initCodeChunk) // value
          .push((nbChunks - decrNbChunks) * 32) // offset
          .op(OpCode.MSTORE);
      decrNbChunks -= 1;
    }
    return program;
  }

  /**
   * The {@code getPgCreateWithInitCodeSize} returns a CREATE or CREATE2 program with a piloted init
   * code. It pushes a 32 Bytes {@code initCodeChunk} in memory as many times as described by @code
   * nbChunks}. Byte size of the init code program is 32 * {@code nbChunks}.
   */
  public static BytecodeCompiler getPgCreateWithInitCodeSize(
      OpCode opCode, Bytes32 initCodeChunk, int nbChunks) {
    checkArgument(opCode == OpCode.CREATE || opCode == OpCode.CREATE2);

    BytecodeCompiler program = getInitCodeWithSize(initCodeChunk, nbChunks);

    if (opCode == OpCode.CREATE2) {
      program.push(salt); // salt
    }

    program
        .push(nbChunks * 32) // size
        .push(0) // offset
        .push(0); // value

    program.op(opCode);

    return program;
  }

  /**
   * The {@code getPgCreateInitCodeWithReturnStartByteAndSize} inserts a single byte {@code
   * startByte} at offset 0 into RAM and returns {@code returnSize} bytes starting at offset 0.
   * There are four cases of interest:
   *
   * <ul>
   *   <li>{@code startByte} is <b>0xEF</b> and {@code returnSize} > 0
   *   <li>{@code startByte} is <b>0xEF</b> and {@code returnSize} = 0
   *   <li>{@code startByte} is anything but <b>0xEF</b> and {@code returnSize} > 0
   *   <li>{@code startByte} is anything but <b>0xEF</b> and {@code returnSize} = 0
   * </ul>
   *
   * Only the first one should raise the <b>invalidCodePrefixException</b>.
   */
  public static BytecodeCompiler getPgCreateInitCodeWithReturnStartByteAndSize(
      int startByte, int returnSize) {
    checkArgument(startByte >= 0);
    checkArgument(startByte < 256);

    BytecodeCompiler initProgram = BytecodeCompiler.newProgram(chainConfig);
    initProgram
        .push(Integer.toHexString(startByte))
        .push(0)
        .op(OpCode.MSTORE8)
        .push(returnSize)
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

    return program;
  }
}
