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

package net.consensys.linea.zktracer;

import static org.identityconnectors.common.ByteUtil.randomBytes;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Random;
import java.util.stream.IntStream;
import java.util.stream.Stream;
import lombok.experimental.Accessors;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@Accessors(fluent = true)
@Tag("weekly")
@ExtendWith(UnitTestWatcher.class)
public class SignedOperationsExtensiveTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("signedComparisonsModDivTestSource")
  void signedComparisonsModDivTest(OpCode opCode, String a, String b, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig).push(b).push(a).op(opCode);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  private static Stream<Arguments> signedComparisonsModDivTestSource() {
    Random rng = new Random(123);

    final String ZERO = "00".repeat(32);
    final String ONE = "00".repeat(31) + "01";

    // 10^3 < SMALL_1 < SMALL_2 < 10^9
    final String SMALL_1 = "66" + randomBytes(rng, 1);
    final String SMALL_2 = "66" + randomBytes(rng, 2);

    final String LARGE_1 = randomBytes(rng, 16);
    final String LARGE_2 = "01" + randomBytes(rng, 16);

    final String MIN_NEG = "80" + "00".repeat(31);
    final String MAX_POS = "7f" + "ff".repeat(31);

    final String NEG_ONE = "ff".repeat(32);

    final String[] RND_POS =
        IntStream.range(0, 10)
            .mapToObj(
                i ->
                    (new BigInteger(randomBytes(rng, 32), 16).and(new BigInteger(MAX_POS, 16)))
                        .toString(16))
            .toArray(String[]::new);
    // e.g., "7f" + randomBytes(31, 5); // < 0x80 ...

    final String[] RND_NEG =
        IntStream.range(0, 10)
            .mapToObj(
                i ->
                    (new BigInteger(randomBytes(rng, 32), 16).or(new BigInteger(MIN_NEG, 16)))
                        .toString(16))
            .toArray(String[]::new);
    // e.g., "81" + randomBytes(31, 6); // > 0x80 ...

    final String[] RND =
        Stream.concat(Arrays.stream(RND_POS), Arrays.stream(RND_NEG)).toArray(String[]::new);

    String[] VALUES =
        Stream.concat(
                Arrays.stream(
                    new String[] {
                      ZERO, ONE, SMALL_1, SMALL_2, LARGE_1, LARGE_2, MIN_NEG, MAX_POS, NEG_ONE
                    }),
                Arrays.stream(RND))
            .toArray(String[]::new);

    List<Arguments> arguments = new ArrayList<>();
    for (OpCode opCode : List.of(OpCode.SLT, OpCode.SGT, OpCode.SMOD, OpCode.SDIV)) {
      for (String a : VALUES) {
        for (String b : VALUES) {
          arguments.add(Arguments.of(opCode, a, b));
        }
      }
    }
    return arguments.stream();
  }

  @ParameterizedTest
  @MethodSource("signExtendTestSource")
  private void signExtendTest(String position, String value, TestInfo testInfo) {
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig).push(value).push(position).op(OpCode.SIGNEXTEND);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  private static Stream<Arguments> signExtendTestSource() {
    Random rng = new Random(123);

    final String[] positions =
        Stream.concat(
                Stream.concat(
                        IntStream.rangeClosed(0, 32).mapToObj(BigInteger::valueOf),
                        Arrays.stream(
                            new BigInteger[] {
                              BigInteger.valueOf(0xFF),
                              BigInteger.valueOf(256).pow(16).subtract(BigInteger.ONE),
                              BigInteger.valueOf(256).pow(16),
                              BigInteger.valueOf(256).pow(16).add(BigInteger.ONE),
                              BigInteger.valueOf(256).pow(32).subtract(BigInteger.ONE)
                            }))
                    .map(n -> n.toString(16)),
                Stream.of(randomBytes(rng, 32))) // random value
            .toArray(String[]::new);

    final String[] bytes = {"00", "56", "7f", "80", "c2", "ff"};

    List<Arguments> arguments = new ArrayList<>();
    for (int i = 0; i < 32; i++) {
      for (String b : bytes) {
        for (String position : positions) {
          String value1 = "00".repeat(i) + b + "00".repeat(31 - i);
          String value2 = "11".repeat(i) + b + "ff".repeat(31 - i);
          String value3 = "ff".repeat(i) + b + "00".repeat(31 - i);

          arguments.add(Arguments.of(position, value1));
          arguments.add(Arguments.of(position, value2));
          arguments.add(Arguments.of(position, value3));
        }
      }
    }
    return arguments.stream();
  }

  // Support method
  private static String randomBytes(Random rng, int n) {
    StringBuilder sb = new StringBuilder();
    for (int i = 0; i < n; i++) {
      sb.append(String.format("%02x", new BigInteger(8, rng).byteValue()));
    }
    return sb.toString();
  }
}
