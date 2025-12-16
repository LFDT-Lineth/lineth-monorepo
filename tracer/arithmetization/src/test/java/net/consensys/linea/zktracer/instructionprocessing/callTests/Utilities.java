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
package net.consensys.linea.zktracer.instructionprocessing.callTests;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.Calendar;
import java.util.Collections;
import java.util.List;
import java.util.Random;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class Utilities {

  public static final String fullEoaAddress = "000000000000000000000000abcdef0123456789";
  public static final String toTrim12 = "aaaaaaaaaaaaaaaaaaaaaaaa";
  public static final String untrimmedEoaAddress = toTrim12 + fullEoaAddress;
  public static final String eoaAddress = "c0ffeef00d";
  public static final String eoaAddress2 = "badbeef";

  public static void fullGasCall(
      BytecodeCompiler program,
      OpCode callOpcode,
      Address to,
      int value,
      int cdo,
      int cds,
      int rao,
      int rac) {
    program.push(rac).push(rao).push(cds).push(cdo);
    if (program.opCodeData(callOpcode).callHasValueArgument()) {
      program.push(value);
    }
    program.push(to).op(GAS).op(callOpcode).op(POP);
  }

  public static void simpleCall(
      BytecodeCompiler program,
      OpCode callOpcode,
      int gas,
      Address to,
      int value,
      int cdo,
      int cds,
      int rao,
      int rac) {
    program.push(rac).push(rao).push(cds).push(cdo);
    if (program.opCodeData(callOpcode).callHasValueArgument()) {
      program.push(value);
    }
    program.push(to).push(gas).op(callOpcode);
  }

  public static void simpleCallAndReturnDataSize(
      BytecodeCompiler program,
      OpCode callOpcode,
      int gas,
      Address to,
      int value,
      int cdo,
      int cds,
      int rao,
      int rac) {
    simpleCall(program, callOpcode, gas, to, value, cdo, cds, rao, rac);
    program.op(RETURNDATASIZE);
  }

  public static void callCaller(
      BytecodeCompiler program,
      OpCode callOpcode,
      int gas,
      int value,
      int cdo,
      int cds,
      int rao,
      int rac) {
    program.push(rac).push(rao).push(cds).push(cdo);
    if (program.opCodeData(callOpcode).callHasValueArgument()) {
      program.push(value);
    }
    program.op(CALLER).push(gas).op(callOpcode);
  }

  public static void appendInsufficientBalanceCall(
      BytecodeCompiler program,
      OpCode callOpcode,
      int gas,
      Address to,
      int cdo,
      int cds,
      int rao,
      int rac) {
    checkArgument(program.opCodeData(callOpcode).callHasValueArgument());
    program
        .push(rac)
        .push(rao)
        .push(cds)
        .push(cdo)
        .op(SELFBALANCE)
        .push(1)
        .op(ADD) // puts balance + 1 on the stack
        .push(to)
        .push(gas)
        .op(callOpcode);
  }

  /**
   * Pushing, in order: h, v, r, s; values produce a valid signature.
   *
   * @param program
   */
  public static void validEcrecoverData(BytecodeCompiler program) {
    program
        .push("279d94621558f755796898fc4bd36b6d407cae77537865afe523b79c74cc680b")
        .push(0)
        .op(MSTORE)
        .push("1b")
        .push(32)
        .op(MSTORE)
        .push("c2ff96feed8749a5ad1c0714f950b5ac939d8acedbedcbc2949614ab8af06312")
        .push(64)
        .op(MSTORE)
        .push("1feecd50adc6273fdd5d11c6da18c8cfe14e2787f5a90af7c7c1328e7d0a2c42")
        .push(96)
        .op(MSTORE);
  }

  public static void fullReturnDataCopyAt(BytecodeCompiler program, int targetOffset) {
    program.op(RETURNDATASIZE).push(0).push(targetOffset);
  }

  /**
   * Copies <b>1 / 2</b> of the return data starting at offset <b>RDS / 3</b> (internal to return
   * data) into memory at targetOffset.
   *
   * @param program
   * @param targetOffset
   */
  public static void copyHalfOfReturnDataOmittingTheFirstThirdOfIt(
      BytecodeCompiler program, int targetOffset) {
    pushRdsOverArgOntoTheStack(program, 2); // source size   ≡ rds/2
    pushRdsOverArgOntoTheStack(program, 3); // source offset ≡ rds/3
    program.push(targetOffset);
  }

  /**
   * Loads the (right 0 padded) first <b>RDS ∧ 32</b> bytes from return data onto the stack.
   *
   * <p><b>Note.</b> if <b>RDS < 32</b>.
   *
   * @param program
   * @param offset
   */
  public static void loadFirstReturnDataWordOntoStack(BytecodeCompiler program, int offset) {
    squashMemoryWordAtOffset(program, offset);
    pushMinOfRdsAnd32OntoStack(program); // leaves min(RDS, 32) on the stack
    program.push(0).push(offset).op(RETURNDATACOPY).push(offset).op(MLOAD);
  }

  /**
   * Pushes the integer <b>RDS ∧ 32 = min(RDS, 32)</b> onto the stack.
   *
   * @param program
   */
  public static void pushMinOfRdsAnd32OntoStack(BytecodeCompiler program) {
    program
        .push(WORD_SIZE)
        .op(RETURNDATASIZE)
        .op(LT) // stack: | ... | c ], where c ≡ [RDS < 32]
        .op(DUP1)
        .push(1)
        .op(SUB) // stack: | ... | c | d ], where d ≡ ¬c ≡ [RDS ≥ 32]
        .push(WORD_SIZE)
        .op(MUL) // stack: | ... | c | (d ? 32 : 0) ]
        .op(SWAP1) // stack:  | ... | (d ? 32 : 0) | c ]
        .op(RETURNDATASIZE)
        .op(MUL) // stack: | ... | (d ? 32 : 0) | (c ? RDS : 0) ]
        .op(ADD) // stack: | ... | min(RDS, 32) ]
    ;
  }

  /**
   * Squashes the word in memory at (byte)<b>offset</b>, i.e. replaces it with <b>0x 00 .. 00</b>.
   *
   * @param program
   * @param offset
   */
  public static void squashMemoryWordAtOffset(BytecodeCompiler program, int offset) {
    program.push(0).push(offset).op(MSTORE);
  }

  /**
   * Pushes <b>RDS / arg</b> onto the stack.
   *
   * @param program
   * @param arg
   */
  public static void pushRdsOverArgOntoTheStack(BytecodeCompiler program, int arg) {
    program.push(arg).op(RETURNDATASIZE).op(DIV);
  }

  /**
   * Populates memory with 6 words of data, namely
   *
   * <p><b>0x aa ... aa bb ... bb cc ... cc dd ... dd ee ... ee ff ... ff</b>
   *
   * <p>starting at offset 0. This provides 192 = 6*32 nonzero bytes in RAM.
   */
  public static void populateMemory(BytecodeCompiler program) {
    populateMemory(program, 6, 0);
  }

  /**
   * {@link #populateMemory} populates memory with <b>nWords</b> chosen cyclically from the set of 6
   * EVM words obtained by repeating the strings <b>aa</b>, <b>bb</b>, ..., <b>ff</b> 32 times.
   *
   * @param program
   * @param nWords
   */
  public static void populateMemory(BytecodeCompiler program, int nWords, int offset) {
    List<String> abcdef = List.of("aa", "bb", "cc", "dd", "ee", "ff");
    for (int i = 0; i < nWords; i++) {
      program
          .push(abcdef.get(i % abcdef.size()).repeat(WORD_SIZE)) // value, a 32 byte word
          .push(offset + i * WORD_SIZE) // offset
          .op(MSTORE);
    }
  }

  /**
   * Appends byte code to the {@code program} that loads the first word of call data onto the stack
   * and interprets it as an address like so:
   *
   * <p><b>[address | xx ... xx]</b>
   *
   * <p>It then calls into this address.
   *
   * @param program
   * @param callOpCode
   */
  public static void appendCallTo(BytecodeCompiler program, OpCode callOpCode, Address address) {
    OpCodeData callInfo = program.opCodeData(callOpCode);
    checkArgument(callInfo.isCall());

    pushSeveral(program, 0, 0, 0, 0);
    if (callInfo.callHasValueArgument()) {
      program.push(256); // value
    }
    program.push(address).op(GAS).op(callOpCode);
  }

  /**
   * Performs a full copy of the code at {@code foreignAddress} and runs it as initialization code.
   *
   * @param program
   * @param foreignAddress
   */
  public static void copyForeignCodeAndRunItAsInitCode(
      BytecodeCompiler program, Address foreignAddress) {

    program.push(foreignAddress).op(EXTCODESIZE); // ] EXTCS ]
    pushSeveral(program, 0, 0);
    program.push(foreignAddress); // ] EXTCS | 0 | 0 | foreignAddress ]
    program.op(EXTCODECOPY);
    program.op(MSIZE);
    pushSeveral(program, 0, 0); // ] MSIZE | 0 | 0 ]
    program.op(CREATE);
  }

  /**
   * <b>EXTCODECOPY</b>'s all of {@code foreignAddress}'s byte code and returns it.
   *
   * @param program
   * @param foreignAddress
   */
  public static void copyForeignCodeAndReturnIt(BytecodeCompiler program, Address foreignAddress) {
    copyForeignCodeToRam(program, foreignAddress);
    program.op(MSIZE).push(0).op(RETURN); // return memory in full
  }

  public static void copyForeignCodeToRam(BytecodeCompiler program, Address foreignAddress) {
    program.push(foreignAddress).op(EXTCODESIZE); // ] EXTCS ]
    pushSeveral(program, 0, 0);
    program.push(foreignAddress); // ] EXTCS | 0 | 0 | foreignAddress ]
    program.op(EXTCODECOPY); // full copy of foreign code
  }

  public static void sstoreTopOfStackTo(BytecodeCompiler program, int storageKey) {
    program.push(storageKey).op(SSTORE);
  }

  public static void sloadFrom(BytecodeCompiler program, int storageKey) {
    program.push(storageKey).op(SLOAD);
  }

  public static void revertWith(BytecodeCompiler program, int offset, int size) {
    program.push(size).push(offset).op(REVERT);
  }

  public static void pushSeveral(BytecodeCompiler program, int... values) {
    for (int value : values) {
      program.push(value);
    }
  }

  /**
   * Sample exactly n items at random from a given input list. If that list has fewer than n items,
   * then the list is returned unchanged. The Random Number Generated (RNG) is seeded with the
   * day-of-the-month. The idea here is that we benefit from different seeds (i.e. by testing
   * different inputs), but should a failure occur we can (in principle) recreate it (provided we
   * know what day of the month it failed on).
   *
   * @param n Number of items to sample
   * @param items Source of items to sample from
   * @param <T>
   * @return
   */
  public static <T> List<T> randomSampleByDayOfMonth(int n, List<T> items) {
    // Determine day of month
    int dayOfMonth = Calendar.getInstance().get(Calendar.DAY_OF_MONTH);
    // Seed rng with day of month.
    Random rng = new Random(dayOfMonth);
    // Randomly shuffle the items
    Collections.shuffle(items, rng);
    //
    return items.subList(0, n);
  }
}
