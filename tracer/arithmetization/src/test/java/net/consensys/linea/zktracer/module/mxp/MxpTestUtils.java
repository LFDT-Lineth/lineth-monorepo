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

package net.consensys.linea.zktracer.module.mxp;

import static net.consensys.linea.zktracer.Fork.isPostCancun;
import static net.consensys.linea.zktracer.opcode.OpCode.MSTORE8;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Random;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.OpCodes;
import net.consensys.linea.zktracer.opcode.gas.MxpType;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class MxpTestUtils {
  public static final EWord TWO_POW_128 = EWord.of(EWord.ONE.shiftLeft(128));
  public static final EWord TWO_POW_32 = EWord.of(EWord.ONE.shiftLeft(32));
  public static final int MAX_BYTE_SIZE =
      32; // To trigger MXPX we need at least 5 bytes, while ROOB at least 17 bytes

  public static final OpCode[] opCodesType1 = new OpCode[] {OpCode.MSIZE};
  public static final OpCode[] opCodesType2 = new OpCode[] {OpCode.MLOAD, OpCode.MSTORE};
  public static final OpCode[] opCodesType3 = new OpCode[] {MSTORE8};
  public static final OpCode[] opCodesType4ExcludingHalting =
      new OpCode[] {
        OpCode.LOG0,
        OpCode.LOG1,
        OpCode.LOG2,
        OpCode.LOG3,
        OpCode.LOG4,
        OpCode.SHA3,
        OpCode.CODECOPY,
        OpCode.CALLDATACOPY,
        OpCode.EXTCODECOPY,
        OpCode.CREATE,
        OpCode.CREATE2
      };
  public static final OpCode[] opCodesType4Halting = new OpCode[] {OpCode.RETURN, OpCode.REVERT};

  public static final OpCode[] opCodesType4 =
      Stream.concat(Arrays.stream(opCodesType4ExcludingHalting), Arrays.stream(opCodesType4Halting))
          .toArray(OpCode[]::new);

  public static final OpCode[] opCodesType5 =
      new OpCode[] {OpCode.CALL, OpCode.CALLCODE, OpCode.DELEGATECALL, OpCode.STATICCALL};

  /**
   * NOTE: Do not make this static as it will introduce non-deterministic behaviour into the testing
   * process.
   */
  private final Random RAND = new Random(123456789123456L);

  private final OpCodes opCodes;

  public MxpTestUtils(OpCodes opCodes) {
    this.opCodes = opCodes;
  }

  /**
   * Get the next integer between 0 (inclusive) and n (exclusive) from the random number generator.
   *
   * @return
   */
  public int nextRandomInt(int n) {
    return RAND.nextInt(n);
  }

  /**
   * Get the next integer between n (inclusive) and m (exclusive) from the random number generator.
   *
   * @return
   */
  public int nextRandomInt(int n, int m) {
    return RAND.nextInt(n, m);
  }

  /**
   * Get the next float between 0 (inclusive) and 1.0 (exclusive) from the random number generator.
   *
   * @return
   */
  public float nextRandomFloat() {
    return RAND.nextFloat();
  }

  public void triggerNonTrivialButMxpxOrRoobOrMaxCodeSizeExceptionForOpCode(
      Fork fork,
      BytecodeCompiler program,
      boolean triggerRoob,
      boolean triggerMaxCodeSizeException,
      OpCode opCode) {
    // Generate as many random values as needed at most
    EWord size1;
    EWord size2;
    EWord offset1;
    EWord offset2;
    boolean roob;
    boolean mxpx;
    EWord value = getRandomBigIntegerByBytesSize(0, 4);
    Address address = getRandomBigIntegerByBytesSize(20, 20).toAddress();
    EWord salt = getRandomBigIntegerByBytesSize(0, 4);
    EWord gas = EWord.of(1000);
    OpCodeData opCodeData = this.opCodes.of(opCode);
    // Keep generating random values until we are in the mxpx && roob case or in the mxpx && !roob
    // case
    do {
      // For creates, we trigger mxpx with the offset to avoid triggering a max code size exception
      // that takes precedence on mxpx, so size1 is set to a small value (1)
      size1 =
          (opCodeData.isCreate() && !triggerMaxCodeSizeException)
              ? EWord.of(1)
              : getRandomBigIntegerByBytesSize(0, MAX_BYTE_SIZE);
      size2 = getRandomBigIntegerByBytesSize(0, MAX_BYTE_SIZE);
      offset1 = getRandomBigIntegerByBytesSize(0, MAX_BYTE_SIZE);
      offset2 = getRandomBigIntegerByBytesSize(0, MAX_BYTE_SIZE);

      mxpx = isMxpx(fork, opCodeData, size1, offset1, size2, offset2);
      roob = isRoob(fork, opCodeData, size1, offset1, size2, offset2);
    } while (!(triggerRoob && mxpx && roob) && !(!triggerRoob && mxpx && !roob));

    switch (opCode) {
      case MLOAD -> appendOpCodeCall(List.of(offset1), opCode, program);
      case MSTORE, MSTORE8 -> appendOpCodeCall(List.of(value, offset1), opCode, program);
      case LOG0,
          SHA3,
          RETURN,
          REVERT -> // RETURN and REVERT are selected only when isHalting is true
          appendOpCodeCall(List.of(size1, offset1), opCode, program);
      case LOG1 ->
          appendOpCodeCall(
              Stream.concat(getRandomUpTo32BytesBigIntegers(1).stream(), Stream.of(size1, offset1))
                  .collect(Collectors.toList()),
              opCode,
              program);
      case LOG2 ->
          appendOpCodeCall(
              Stream.concat(getRandomUpTo32BytesBigIntegers(2).stream(), Stream.of(size1, offset1))
                  .collect(Collectors.toList()),
              opCode,
              program);
      case LOG3 ->
          appendOpCodeCall(
              Stream.concat(getRandomUpTo32BytesBigIntegers(3).stream(), Stream.of(size1, offset1))
                  .collect(Collectors.toList()),
              opCode,
              program);
      case LOG4 ->
          appendOpCodeCall(
              Stream.concat(getRandomUpTo32BytesBigIntegers(4).stream(), Stream.of(size1, offset1))
                  .collect(Collectors.toList()),
              opCode,
              program);
      case CODECOPY, CALLDATACOPY ->
          appendOpCodeCall(List.of(size1, offset2, offset1), opCode, program);
      case EXTCODECOPY ->
          appendOpCodeCall(
              List.of(size1, offset2, offset1, EWord.of(address.getBytes())), opCode, program);
      case CREATE, CREATE2 -> {
        if (opCode == OpCode.CREATE) {
          // CREATE
          appendOpCodeCall(List.of(size1, offset1, value), opCode, program);
        } else {
          // CREATE2
          appendOpCodeCall(List.of(salt, size1, offset1, value), opCode, program);
        }
      }
      case STATICCALL, DELEGATECALL ->
          appendOpCodeCall(
              List.of(size2, offset2, size1, offset1, address.getBytes(), gas), opCode, program);
      case CALL, CALLCODE ->
          appendOpCodeCall(
              List.of(size2, offset2, size1, offset1, value, address.getBytes(), gas),
              opCode,
              program);
      default -> {}
    }
  }

  public static boolean isRoob(
      Fork fork, OpCodeData opCode, EWord size1, EWord offset1, EWord size2, EWord offset2) {
    boolean roob;
    if (isPostCancun(fork)) {
      roob = isRoob(opCode, size1, offset1, size2, offset2);
    } else {
      MxpType randomMxpType = opCode.billing().type();
      roob = isRoobLondon(randomMxpType, size1, offset1, size2, offset2);
    }
    return roob;
  }

  private static boolean isRoobLondon(
      MxpType randomMxpType, EWord size1, EWord offset1, EWord size2, EWord offset2) {
    final boolean offsetIsEnormousAndSizeIsNonZero1 =
        offset1.compareTo(TWO_POW_128) >= 0 && !size1.isZero();
    final boolean offsetIsEnormousAndSizeIsNonZero2 =
        offset2.compareTo(TWO_POW_128) >= 0 && !size2.isZero();

    return switch (randomMxpType) {
      case TYPE_2, TYPE_3 -> offset1.compareTo(TWO_POW_128) >= 0;
      case TYPE_4 -> size1.compareTo(TWO_POW_128) >= 0 || offsetIsEnormousAndSizeIsNonZero1;
      case TYPE_5 ->
          size1.compareTo(TWO_POW_128) >= 0
              || offsetIsEnormousAndSizeIsNonZero1
              || size2.compareTo(TWO_POW_128) >= 0
              || offsetIsEnormousAndSizeIsNonZero2;
      default -> false;
    };
  }

  private static boolean isRoob(
      OpCodeData opCode, EWord size1, EWord offset1, EWord size2, EWord offset2) {
    final boolean offsetIsEnormousAndSizeIsNonZero1 =
        offset1.compareTo(TWO_POW_128) >= 0 && !size1.isZero();
    final boolean offsetIsEnormousAndSizeIsNonZero2 =
        offset2.compareTo(TWO_POW_128) >= 0 && !size2.isZero();

    return switch (opCode.mnemonic()) {
      case MLOAD, MSTORE, MSTORE8 -> offset1.compareTo(TWO_POW_128) >= 0;
      case LOG0,
          LOG1,
          LOG2,
          LOG3,
          LOG4,
          CREATE,
          CREATE2,
          RETURN,
          REVERT,
          SHA3,
          CALLDATACOPY,
          CODECOPY,
          EXTCODECOPY,
          RETURNDATACOPY ->
          size1.compareTo(TWO_POW_128) >= 0 || offsetIsEnormousAndSizeIsNonZero1;
      case CALL, CALLCODE, DELEGATECALL, STATICCALL ->
          size1.compareTo(TWO_POW_128) >= 0
              || offsetIsEnormousAndSizeIsNonZero1
              || size2.compareTo(TWO_POW_128) >= 0
              || offsetIsEnormousAndSizeIsNonZero2;
      default -> false;
    };
  }

  public static boolean isMxpx(
      Fork fork, OpCodeData opCode, EWord size1, EWord offset1, EWord size2, EWord offset2) {
    boolean mxpx;
    if (isPostCancun(fork)) {
      mxpx = isMxpx(opCode, size1, offset1, size2, offset2);
    } else {
      MxpType randomMxpType = opCode.billing().type();
      mxpx = isMxpxLondon(randomMxpType, size1, offset1, size2, offset2);
    }
    return mxpx;
  }

  private static boolean isMxpxLondon(
      MxpType randomMxpType, EWord size1, EWord offset1, EWord size2, EWord offset2) {
    EWord maxOffset1 = EWord.ZERO;
    EWord maxOffset2 = EWord.ZERO;
    EWord maxOffset;

    switch (randomMxpType) {
      case TYPE_2 -> maxOffset1 = offset1.add(31);
      case TYPE_3 -> maxOffset1 = offset1;
      case TYPE_4 -> {
        if (!size1.isZero()) {
          maxOffset1 = offset1.add(size1).subtract(1);
        }
      }
      case TYPE_5 -> {
        if (!size1.isZero()) {
          maxOffset1 = offset1.add(size1).subtract(1);
        }
        if (!size2.isZero()) {
          maxOffset2 = offset2.add(size2).subtract(1);
        }
      }
    }

    maxOffset = maxOffset1.greaterThan(maxOffset2) ? maxOffset1 : maxOffset2;
    return maxOffset.compareTo(TWO_POW_32) >= 0;
  }

  public static boolean isMxpx(
      OpCodeData opCode, EWord size1, EWord offset1, EWord size2, EWord offset2) {
    EWord maxOffset1 = EWord.ZERO;
    EWord maxOffset2 = EWord.ZERO;
    EWord maxOffset;

    switch (opCode.mnemonic()) {
      case MLOAD, MSTORE -> maxOffset1 = offset1.add(31);
      case MSTORE8 -> maxOffset1 = offset1;
      case LOG0,
          LOG1,
          LOG2,
          LOG3,
          LOG4,
          CREATE,
          CREATE2,
          RETURN,
          REVERT,
          SHA3,
          CALLDATACOPY,
          CODECOPY,
          EXTCODECOPY,
          RETURNDATACOPY -> {
        if (!size1.isZero()) {
          maxOffset1 = offset1.add(size1).subtract(1);
        }
      }
      case CALL, CALLCODE, DELEGATECALL, STATICCALL -> {
        if (!size1.isZero()) {
          maxOffset1 = offset1.add(size1).subtract(1);
        }
        if (!size2.isZero()) {
          maxOffset2 = offset2.add(size2).subtract(1);
        }
      }
    }

    maxOffset = maxOffset1.greaterThan(maxOffset2) ? maxOffset1 : maxOffset2;
    return maxOffset.compareTo(TWO_POW_32) >= 0;
  }

  public static void appendOpCodeCall(List<Bytes> args, OpCode opCode, BytecodeCompiler program) {
    for (Bytes arg : args) {
      program.push(arg);
    }
    program.op(opCode);
  }

  public static void appendOpCodeCall(OpCode randomOpCode, BytecodeCompiler program) {
    appendOpCodeCall(Collections.emptyList(), randomOpCode, program);
  }

  // Generates a BigInteger that requires a random number of bytes to be represented in [minBytes,
  // maxBytes)
  public EWord getRandomBigIntegerByBytesSize(int minBytes, int maxBytes) {
    if (minBytes < 0 || maxBytes > 32 || minBytes > maxBytes) {
      throw new IllegalArgumentException("Invalid input values");
    }
    int minBits = 8 * minBytes;
    int maxBits = 8 * maxBytes;
    int numBits = RAND.nextInt(minBits, maxBits + 1);
    return EWord.of(new BigInteger(numBits == 0 ? 1 : numBits, RAND));
  }

  public List<EWord> getRandomUpTo32BytesBigIntegers(int n) {
    List<EWord> randomBigIntegers = new ArrayList<>();
    for (int i = 0; i < n; i++) {
      randomBigIntegers.add(EWord.of(getRandomBigIntegerByBytesSize(0, 32)));
    }
    return randomBigIntegers;
  }
}
