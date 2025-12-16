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

package net.consensys.linea.zktracer.module.shf;

import static net.consensys.linea.zktracer.module.HexStringUtils.xor;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.IntStream;
import java.util.stream.Stream;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@Accessors(fluent = true)
@Tag("weekly")
@ExtendWith(UnitTestWatcher.class)
public class ShfExtensiveTest extends TracerTestBase {

  private static final List<Arguments> shfTestSourceList = new ArrayList<>();
  @Getter private static Stream<Arguments> shfWithMaskTestSource;

  @BeforeAll
  public static void init() {
    List<Arguments> shfWithMaskTestSourceList = new ArrayList<>();
    for (int k = 0; k < 32; k++) {
      for (int l = 1; l <= 8; l++) {
        String value = value(k, l);
        shfTestSourceList.add(Arguments.of(value, k, l));
        for (String XY : XYs) {
          String mask = XY + "00".repeat(31);
          String valueXorMask = xor(value, mask);
          shfWithMaskTestSourceList.add(Arguments.of(valueXorMask, k, l, XY));
        }
      }
    }
    shfWithMaskTestSource = shfWithMaskTestSourceList.stream();
    // shfWithMaskTestSource inputs are used only once, so it is fine to generate the corresponding
    // stream here.
    // Note that whenever a stream is used, it is also consumed,
    // that is why in the case of shfTestSourceList inputs,
    // we generate a new stream every time it is needed.
  }

  @ParameterizedTest
  @MethodSource("shfTestSource")
  void shlTest(String value, int k, int l, TestInfo testInfo) {
    shfProgramOf(value, OpCode.SHL).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @MethodSource("shfTestSource")
  void shrTest(String value, int k, int l, TestInfo testInfo) {
    shfProgramOf(value, OpCode.SHR).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @MethodSource("shfTestSource")
  void sarTest(String value, int k, int l, TestInfo testInfo) {
    shfProgramOf(value, OpCode.SAR).run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @MethodSource("shfWithMaskTestSource")
  void sarWithMaskTest(String value, int k, int l, String XY, TestInfo testInfo) {
    shfProgramOf(value, OpCode.SAR).run(chainConfig, testInfo);
  }

  private static Stream<Arguments> shfTestSource() {
    return shfTestSourceList.stream();
    // A new stream is generated whenever it is necessary, starting from the same list
  }

  // Inputs and support methods

  static final String[] SHIFTS =
      Stream.concat(
              IntStream.rangeClosed(0, 257) // Generates numbers 0 to 257
                  .mapToObj(BigInteger::valueOf),
              Stream.of(
                  BigInteger.valueOf(511),
                  BigInteger.valueOf(512),
                  BigInteger.valueOf(513),
                  BigInteger.valueOf(65535),
                  BigInteger.valueOf(65536),
                  BigInteger.valueOf(65537),
                  BigInteger.ONE.shiftLeft(128).subtract(BigInteger.ONE), // (1 << 128) - 1
                  BigInteger.ONE.shiftLeft(128), // (1 << 128)
                  BigInteger.ONE.shiftLeft(128).add(BigInteger.ONE), // (1 << 128) + 1
                  BigInteger.ONE.shiftLeft(256).subtract(BigInteger.ONE) // (1 << 256) - 1
                  ))
          .map(bigInteger -> bigInteger.toString(16))
          .toArray(String[]::new);

  static final String[] P = {
    "df", "d5", "a2", "e7", "6e", "9d", "3a", "20",
    "96", "2d", "17", "48", "19", "7f", "0d", "4c",
    "ff", "3d", "57", "a4", "a8", "87", "45", "b9",
    "c9", "34", "1a", "f3", "57", "84", "d3", "ee"
  }; // big-endian (from the least significant byte to the most significant byte)

  static final String[] XYs = new String[] {"80", "90", "a0", "b0", "c0", "d0", "e0", "f0"};

  //  Creates a program that concatenates shifts operations (with different relevant shift values)
  //  for a given value and opcode
  private BytecodeRunner shfProgramOf(String value, OpCode opCode) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    for (String shift : SHIFTS) {
      program.push(value).push(shift).op(opCode);
    }
    return BytecodeRunner.of(program.compile());
  }

  public static String value(int k, int l) {
    String[] v = new String[32];
    // 0 to k - 1
    if (k >= 0) System.arraycopy(P, 0, v, 0, k);
    // k
    v[k] = String.format("%02x", (1 << l) - 1);
    // k + 1 to 31
    for (int i = k + 1; i < 32; i++) {
      v[i] = "00";
    }
    return String.join("", java.util.Arrays.asList(v).reversed());
  }

  // Testing support methods
  @Test
  void testValue() {
    Assertions.assertEquals(
        "0000000000000000000000003f573dff4c0d7f1948172d96203a9d6ee7a2d5df", value(19, 6));
  }

  // ###################################################################################################################

  // This test should be executed only occasionally since very long. Run below batched tests instead
  @Disabled
  @ParameterizedTest
  @MethodSource("shfExtensiveTestSource")
  void shfExtensiveTest(String shift, String value, OpCode opCode, TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig).push(value).push(shift).op(opCode).compile())
        .run(chainConfig, testInfo);
  }

  private static Stream<Arguments> shfExtensiveTestSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (String shift : SHIFTS) {
      for (int k = 0; k < 32; k++) {
        for (int l = 1; l <= 8; l++) {
          String value = value(k, l);
          arguments.add(Arguments.of(shift, value, OpCode.SHL));
          arguments.add(Arguments.of(shift, value, OpCode.SHR));
          arguments.add(Arguments.of(shift, value, OpCode.SAR));
          // Adding additional cases for SAR
          for (String XY : XYs) {
            String mask = XY + "00".repeat(31);
            String valueXorMask = xor(value, mask);
            arguments.add(Arguments.of(shift, valueXorMask, OpCode.SAR));
          }
        }
      }
    }
    return arguments.stream();
  }
}
