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

package net.consensys.linea.zktracer.module.exp;

import static java.lang.Math.max;
import static java.lang.Math.min;
import static org.assertj.core.api.Assertions.assertThat;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import org.junit.jupiter.params.provider.ValueSource;

@Slf4j
@ExtendWith(UnitTestWatcher.class)
public class ExpTest extends TracerTestBase {
  // Generates 128, 64, 2, 1 as LD (leading digit)
  // LD_INDICES | LD
  // ---------- | ---------------------
  // 1          | 128 = 1000 0000 = 0x80
  // 2          | 64  = 0100 0000 = 0x40
  // 7          | 2   = 0000 0010 = 0x02
  // 8          | 1   = 0000 0001 = 0x01
  private static final int[] LD_INDICES = new int[] {1, 2, 7, 8}; // LEADING_DIGIT_INDICES

  // Generates 1, 2, 10, 15, 16, 17, 20, 31, 32 as ebsCutoff and cdsCutoff
  // and k (index of a specific sequence of bytes based on the test case)
  private static final int[] C = new int[] {1, 2, 10, 15, 16, 17, 20, 31, 32}; // CUTOFFS

  // See https://github.com/Consensys/linea-specification-internal/issues/326 as additional
  // documentation

  @Test
  void testExpLogSingleCase(TestInfo testInfo) {
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig).push(2).push(10).op(OpCode.EXP);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @Test
  void testExpLogDuplicate(TestInfo testInfo) {
    BytecodeRunner bytecodeRunner =
        BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(256)
                .push(256)
                .op(OpCode.EXP)
                .op(OpCode.POP)
                .push(256)
                .push(256)
                .op(OpCode.EXP)
                .op(OpCode.POP));
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @Test
  void testModexpLogSingleCase(TestInfo testInfo) {
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig)
            .push(1) // bbs
            .push(0)
            .op(OpCode.MSTORE)
            .push(1) // ebs
            .push(0x20)
            .op(OpCode.MSTORE)
            .push(1) // mbs
            .push(0x40)
            .op(OpCode.MSTORE)
            .push(
                Bytes.fromHexStringLenient(
                    "0x08090A0000000000000000000000000000000000000000000000000000000000")) // b, e,
            // m
            .push(0x60)
            .op(OpCode.MSTORE)
            .push(1) // retSize
            .push(0x9f) // retOffset
            .push(0x63) // argSize (cds)
            .push(0) // argOffset (cdo)
            .push(5) // address
            .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
            .op(OpCode.STATICCALL);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @ValueSource(
      ints = {
        0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
        25, 26, 27, 28, 29, 30, 31, 32
      })
  void testExpLogFFBlockCase(int k, TestInfo testInfo) {
    Bytes exponent = Bytes.fromHexString(ffBlock(k));
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig).push(exponent).push(10).op(OpCode.EXP);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @ValueSource(
      ints = {
        1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25,
        26, 27, 28, 29, 30, 31, 32
      })
  void testExpLogFFAtCase(int k, TestInfo testInfo) {
    Bytes exponent = Bytes.fromHexString(ffAt(k));
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig).push(exponent).push(10).op(OpCode.EXP);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("testModexpLogSource")
  void testModexpLogFFBlockWithLDCase(
      int ebsCutoff, int cdsCutoff, int k, int LDIndex, TestInfo testInfo) {
    // 0x00000000000000000000000000000040ffffffffffffffffffffffffffffffff
    Bytes wordAfterBase = Bytes.fromHexStringLenient(ffBlockWithLd(k, LDIndex));
    BytecodeCompiler program = initProgramInvokingModexp(ebsCutoff, cdsCutoff, wordAfterBase);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("testModexpLogSource")
  void testModexpLogLDAtCase(int ebsCutoff, int cdsCutoff, int k, int ldIndex, TestInfo testInfo) {
    Bytes wordAfterBase = Bytes.fromHexStringLenient(ldAt(k, ldIndex));
    BytecodeCompiler program = initProgramInvokingModexp(ebsCutoff, cdsCutoff, wordAfterBase);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  private static Stream<Arguments> testModexpLogSource() {
    List<Arguments> moxexpLogCases = new ArrayList<>();
    for (int ebsCutoff : C) {
      for (int cdsCutoff : C) {
        for (int k : C) {
          for (int LDIndex : LD_INDICES) {
            moxexpLogCases.add(Arguments.of(ebsCutoff, cdsCutoff, k, LDIndex));
          }
        }
      }
    }
    return moxexpLogCases.stream();
  }

  @Test
  void testModexpLogFFBlockWithLDCaseSpecific(TestInfo testInfo) {
    final int ebsCutoff = 17;
    final int cdsCutoff = 17;
    final int k = 16;
    final int ldIndex = 1;

    Bytes wordAfterBase = Bytes.fromHexStringLenient(ffBlockWithLd(k, ldIndex));
    BytecodeCompiler program = initProgramInvokingModexp(ebsCutoff, cdsCutoff, wordAfterBase);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @Test
  void testModexpLogLDAtCaseSpecific(TestInfo testInfo) {
    final int ebsCutoff = 17;
    final int cdsCutoff = 17;
    final int k = 2;
    final int ldIndex = 1;

    Bytes wordAfterBase = Bytes.fromHexStringLenient(ldAt(k, ldIndex));
    BytecodeCompiler program = initProgramInvokingModexp(ebsCutoff, cdsCutoff, wordAfterBase);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @MethodSource("testModexpLogZerosCaseSource")
  void testModexpLogZerosCase(int ebsCutoff, int cdsCutoff, TestInfo testInfo) {
    Bytes wordAfterBase =
        Bytes.fromHexStringLenient(
            "0000000000000000000000000000000000000000000000000000000000000000");
    BytecodeCompiler program = initProgramInvokingModexp(ebsCutoff, cdsCutoff, wordAfterBase);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  private static Stream<Arguments> testModexpLogZerosCaseSource() {
    List<Arguments> moxexpLogCases = new ArrayList<>();
    for (int ebsCutoff : C) {
      for (int cdsCutoff : C) {
        moxexpLogCases.add(Arguments.of(ebsCutoff, cdsCutoff));
      }
    }
    return moxexpLogCases.stream();
  }

  private BytecodeCompiler initProgramInvokingModexp(
      int ebsCutoff, int cdsCutoff, Bytes wordAfterBase) {
    final int bbs = 0;
    final int minimalValidEbs = ebsCutoff;
    final int mbs = 0;
    final int minimalValidCds = cdsCutoff + 96 + bbs;

    return BytecodeCompiler.newProgram(chainConfig)
        .push(bbs) // bbs
        .push(0)
        .op(OpCode.MSTORE)
        .push(minimalValidEbs) // ebs
        .push(32)
        .op(OpCode.MSTORE)
        .push(mbs) // mbs
        .push(64)
        .op(OpCode.MSTORE)
        .push(wordAfterBase) // e
        .push(96 + bbs)
        .op(OpCode.MSTORE)
        .push(512) // retSize
        .push(minimalValidCds) // retOffset
        .push(minimalValidCds) // argSize (cds)
        .push(0) // argOffset (cdo)
        .push(5) // address
        .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
        .op(OpCode.STATICCALL);
  }

  @Test
  @Disabled
  void testExpUtils() {
    // ExpLog case
    log.debug("FFBlock");
    for (int k = 0; k <= 32; k++) {
      log.debug(ffBlock(k));
    }

    log.debug("FFAt");
    for (int k = 1; k <= 32; k++) {
      log.debug(ffAt(k));
    }

    // ModexpLog case
    log.debug("FFBlockWithLD");
    for (int k : C) {
      for (int ldIndex : LD_INDICES) {
        log.debug(ffBlockWithLd(k, ldIndex));
      }
    }

    log.debug("LDAt");
    for (int k : C) {
      for (int ldIndex : LD_INDICES) {
        log.debug(ldAt(k, ldIndex));
      }
    }

    for (int ebsCutoff : C) {
      for (int cdsCutoff : C) {
        final int bbs = 0;
        final int minimalValidEbs = ebsCutoff;
        final int minimalValidCds = cdsCutoff + 96 + bbs;

        log.debug("minimalValidEbs: " + minimalValidEbs + ", minimalValidCds: " + minimalValidCds);
        log.debug("ebsCutoff: " + ebsCutoff + ", cdsCutoff: " + cdsCutoff);
        log.debug("###");

        assertThat(ebsCutoff).isEqualTo(min(minimalValidEbs, 32));
        assertThat(cdsCutoff).isEqualTo(min(max(minimalValidCds - (96 + bbs), 0), 32));
      }
    }
  }

  public static String ffBlock(int k) {
    if (k < 0 || k > 32) {
      throw new IllegalArgumentException("k must be between 0 and 32");
    }
    return "00".repeat(32 - k) + "ff".repeat(k);
  }

  public static String ffAt(int k) {
    if (k < 1 || k > 32) {
      throw new IllegalArgumentException("k must be between 1 and 32");
    }
    return "00".repeat(k - 1) + "ff" + "00".repeat(32 - k);
  }

  public static String ffBlockWithLd(int k, int LDIndex) {
    if (k < 1 || k > 32) {
      throw new IllegalArgumentException("k must be between 1 and 32");
    }
    if (LDIndex < 1 || LDIndex > 8) {
      throw new IllegalArgumentException("LDIndex must be between 1 and 8");
    }
    String ld =
        new BigInteger("0".repeat(LDIndex - 1) + "1" + "0".repeat(8 - LDIndex), 2).toString(16);
    if (k < 32) {
      return "00".repeat(32 - k - 1) + (ld.length() == 1 ? "0" + ld : ld) + "ff".repeat(k);
    }

    return "ff".repeat(k);
  }

  public static String ldAt(int k, int ldIndex) {
    if (k < 1 || k > 32) {
      throw new IllegalArgumentException("k must be between 1 and 32");
    }
    if (ldIndex < 1 || ldIndex > 8) {
      throw new IllegalArgumentException("ldindex must be between 1 and 8");
    }
    String ld =
        new BigInteger("0".repeat(ldIndex - 1) + "1" + "0".repeat(8 - ldIndex), 2).toString(16);

    return "00".repeat(k - 1) + (ld.length() == 1 ? "0" + ld : ld) + "00".repeat(32 - k);
  }
}
