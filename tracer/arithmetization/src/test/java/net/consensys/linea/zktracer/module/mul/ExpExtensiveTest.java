package net.consensys.linea.zktracer.module.mul;

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

import static net.consensys.linea.zktracer.module.HexStringUtils.and;
import static net.consensys.linea.zktracer.module.HexStringUtils.rightShift;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.IntStream;
import java.util.stream.Stream;
import lombok.experimental.Accessors;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.HexStringUtils;
import net.consensys.linea.zktracer.opcode.OpCode;
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
public class ExpExtensiveTest extends TracerTestBase {
  // Test vectors
  static final String P_1 = "f076b857fa9947c1f9ec558262c72704099ca8cd325566f73fb99238102ed171";
  static final String P_2 = "c809c9170ca6faec82d43ee6754dbad01d198ecae0823bac23ca30c7f0c9657d";

  static final String[] EVEN_BASES =
      IntStream.rangeClosed(1, 256).mapToObj(HexStringUtils::even).toArray(String[]::new);

  static final String[] SIMPLE_ODD_BASES =
      IntStream.rangeClosed(0, 31)
          .map(i -> i * 8) // Generates 0, 8, 16, ..., 248
          .mapToObj(HexStringUtils::odd)
          .toArray(String[]::new);

  static final String[] OTHER_ODD_BASES =
      Stream.of(SIMPLE_ODD_BASES).map(mask -> and(P_1, mask)).toArray(String[]::new);

  static final String[] COMPLEX_EXPONENTS =
      IntStream.rangeClosed(0, 256).mapToObj(n -> rightShift(P_2, n)).toArray(String[]::new);

  static final String[] TINY_EXPONENTS =
      Stream.concat(
              IntStream.rangeClosed(0, 257) // Generates numbers 0 to 257
                  .mapToObj(BigInteger::valueOf),
              Stream.of(
                  BigInteger.valueOf(65535), BigInteger.valueOf(65536), BigInteger.valueOf(65537)))
          .map(bigInteger -> bigInteger.toString(16))
          .toArray(String[]::new);

  static final String[] THRESHOLD_EXPONENTS =
      Stream.of(
              BigInteger.ONE.shiftLeft(128).subtract(BigInteger.ONE), // (1 << 128) - 1
              BigInteger.ONE.shiftLeft(128), // (1 << 128)
              BigInteger.ONE.shiftLeft(128).add(BigInteger.ONE), // (1 << 128) + 1
              BigInteger.ONE.shiftLeft(256).subtract(BigInteger.ONE) // (1 << 256) - 1
              )
          .map(bigInteger -> bigInteger.toString(16))
          .toArray(String[]::new);

  static final String[] SPECIAL_EXPONENTS =
      Stream.of(TINY_EXPONENTS, THRESHOLD_EXPONENTS).flatMap(Stream::of).toArray(String[]::new);

  static final String[] BASES =
      Stream.of(EVEN_BASES, SIMPLE_ODD_BASES, OTHER_ODD_BASES)
          .flatMap(Stream::of)
          .toArray(String[]::new);

  static final String[] EXPONENTS =
      Stream.of(COMPLEX_EXPONENTS, SPECIAL_EXPONENTS).flatMap(Stream::of).toArray(String[]::new);

  static final List<Arguments> EXPONENTS_LIST = Stream.of(EXPONENTS).map(Arguments::of).toList();

  // Note that flatMap(Stream::of) converts stream of String[] to stream of String

  // This is not an actual test, but just a utility to generate test cases
  // @Disabled
  @Test
  public void generateTestCases(TestInfo testInfo) {
    for (int i = 0; i < BASES.length; i++) {
      System.out.println(
          "@Test\n"
              + "void expTestForBase_"
              + BASES[i]
              + "(TestInfo testInfo) {\n"
              + "    expProgramOf(BASES["
              + i
              + "]).run();\n"
              + "}\n");
    }
  }

  // Tests
  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe(
      TestInfo testInfo) {
    expProgramOf(BASES[0]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc(
      TestInfo testInfo) {
    expProgramOf(BASES[1]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8(
      TestInfo testInfo) {
    expProgramOf(BASES[2]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0(
      TestInfo testInfo) {
    expProgramOf(BASES[3]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0(
      TestInfo testInfo) {
    expProgramOf(BASES[4]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc0(
      TestInfo testInfo) {
    expProgramOf(BASES[5]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff80(
      TestInfo testInfo) {
    expProgramOf(BASES[6]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00(
      TestInfo testInfo) {
    expProgramOf(BASES[7]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe00(
      TestInfo testInfo) {
    expProgramOf(BASES[8]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc00(
      TestInfo testInfo) {
    expProgramOf(BASES[9]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff800(
      TestInfo testInfo) {
    expProgramOf(BASES[10]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff000(
      TestInfo testInfo) {
    expProgramOf(BASES[11]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe000(
      TestInfo testInfo) {
    expProgramOf(BASES[12]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc000(
      TestInfo testInfo) {
    expProgramOf(BASES[13]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8000(
      TestInfo testInfo) {
    expProgramOf(BASES[14]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0000(
      TestInfo testInfo) {
    expProgramOf(BASES[15]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0000(
      TestInfo testInfo) {
    expProgramOf(BASES[16]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc0000(
      TestInfo testInfo) {
    expProgramOf(BASES[17]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff80000(
      TestInfo testInfo) {
    expProgramOf(BASES[18]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00000(
      TestInfo testInfo) {
    expProgramOf(BASES[19]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe00000(
      TestInfo testInfo) {
    expProgramOf(BASES[20]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc00000(
      TestInfo testInfo) {
    expProgramOf(BASES[21]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffff800000(
      TestInfo testInfo) {
    expProgramOf(BASES[22]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffff000000(
      TestInfo testInfo) {
    expProgramOf(BASES[23]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffe000000(
      TestInfo testInfo) {
    expProgramOf(BASES[24]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffffc000000(
      TestInfo testInfo) {
    expProgramOf(BASES[25]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffff8000000(
      TestInfo testInfo) {
    expProgramOf(BASES[26]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffff0000000(
      TestInfo testInfo) {
    expProgramOf(BASES[27]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0000000(
      TestInfo testInfo) {
    expProgramOf(BASES[28]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffc0000000(
      TestInfo testInfo) {
    expProgramOf(BASES[29]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffff80000000(
      TestInfo testInfo) {
    expProgramOf(BASES[30]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffff00000000(
      TestInfo testInfo) {
    expProgramOf(BASES[31]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffe00000000(
      TestInfo testInfo) {
    expProgramOf(BASES[32]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffffc00000000(
      TestInfo testInfo) {
    expProgramOf(BASES[33]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffff800000000(
      TestInfo testInfo) {
    expProgramOf(BASES[34]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffff000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[35]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffe000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[36]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffc000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[37]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffff8000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[38]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffff0000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[39]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffe0000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[40]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffffc0000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[41]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffff80000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[42]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffff00000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[43]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffe00000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[44]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffc00000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[45]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffff800000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[46]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffff000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[47]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffe000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[48]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffffc000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[49]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffff8000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[50]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffff0000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[51]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffe0000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[52]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffc0000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[53]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffff80000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[54]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffff00000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[55]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffe00000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[56]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffffc00000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[57]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffff800000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[58]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffff000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[59]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffe000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[60]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffc000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[61]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffff8000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[62]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffff0000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[63]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffe0000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[64]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffffc0000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[65]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffff80000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[66]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffff00000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[67]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffe00000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[68]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffc00000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[69]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffff800000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[70]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffff000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[71]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffe000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[72]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffffc000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[73]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffff8000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[74]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffff0000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[75]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffe0000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[76]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffc0000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[77]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffff80000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[78]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffff00000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[79]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffe00000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[80]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffffc00000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[81]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffff800000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[82]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffff000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[83]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffe000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[84]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffc000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[85]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffff8000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[86]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffff0000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[87]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffe0000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[88]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffffc0000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[89]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffff80000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[90]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffff00000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[91]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffe00000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[92]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffc00000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[93]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffff800000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[94]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffff000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[95]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffe000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[96]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffffc000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[97]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffff8000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[98]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffff0000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[99]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffe0000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[100]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffc0000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[101]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffff80000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[102]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffff00000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[103]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffe00000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[104]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffffc00000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[105]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffff800000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[106]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffff000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[107]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffe000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[108]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffc000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[109]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffff8000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[110]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffff0000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[111]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffe0000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[112]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffffc0000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[113]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffff80000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[114]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffff00000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[115]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffe00000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[116]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffc00000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[117]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffff800000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[118]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffff000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[119]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffe000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[120]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffffc000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[121]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffff8000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[122]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffff0000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[123]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffe0000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[124]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffc0000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[125]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffff80000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[126]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffff00000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[127]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffe00000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[128]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffffc00000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[129]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffff800000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[130]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffff000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[131]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffe000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[132]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffc000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[133]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffff8000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[134]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffff0000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[135]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffe0000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[136]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffffc0000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[137]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffff80000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[138]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffff00000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[139]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffe00000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[140]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffc00000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[141]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffff800000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[142]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffff000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[143]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffe000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[144]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffffc000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[145]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffff8000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[146]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffff0000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[147]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffe0000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[148]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffc0000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[149]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffff80000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[150]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffff00000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[151]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffe00000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[152]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffffc00000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[153]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffff800000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[154]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffff000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[155]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffe000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[156]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffc000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[157]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffff8000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[158]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffff0000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[159]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffe0000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[160]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffffc0000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[161]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffff80000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[162]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffff00000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[163]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffe00000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[164]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffc00000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[165]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffff800000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[166]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffff000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[167]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffe000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[168]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffffc000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[169]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffff8000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[170]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffff0000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[171]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffe0000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[172]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffc0000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[173]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffff80000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[174]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffff00000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[175]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffe00000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[176]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffffc00000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[177]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffff800000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[178]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffff000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[179]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffe000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[180]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffc000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[181]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffff8000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[182]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffff0000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[183]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffe0000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[184]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffffc0000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[185]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffff80000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[186]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffff00000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[187]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffe00000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[188]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffc00000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[189]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffff800000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[190]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffff000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[191]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffe000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[192]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffffc000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[193]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffff8000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[194]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffff0000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[195]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffe0000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[196]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffc0000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[197]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffff80000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[198]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffff00000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[199]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffe00000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[200]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffffc00000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[201]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffff800000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[202]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffff000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[203]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffe000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[204]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffc000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[205]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffff8000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[206]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffff0000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[207]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffe0000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[208]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffffc0000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[209]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffff80000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[210]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffff00000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[211]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffe00000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[212]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffc00000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[213]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffff800000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[214]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffff000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[215]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffe000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[216]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffffc000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[217]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffff8000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[218]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffff0000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[219]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffe0000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[220]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffc0000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[221]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffff80000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[222]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffff00000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[223]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffe00000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[224]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffffc00000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[225]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffff800000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[226]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffff000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[227]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffe000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[228]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffc000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[229]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffff8000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[230]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffff0000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[231]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffe0000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[232]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffffc0000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[233]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffff80000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[234]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffff00000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[235]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffe00000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[236]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffc00000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[237]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffff800000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[238]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffff000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[239]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffe000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[240]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fffc000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[241]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fff8000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[242]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fff0000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[243]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffe0000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[244]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffc0000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[245]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ff80000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[246]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ff00000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[247]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fe00000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[248]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fc00000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[249]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_f800000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[250]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_f000000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[251]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_e000000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[252]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_c000000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[253]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_8000000000000000000000000000000000000000000000000000000000000000(
      TestInfo testInfo) {
    expProgramOf(BASES[254]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_0(TestInfo testInfo) {
    expProgramOf(BASES[255]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff(
      TestInfo testInfo) {
    expProgramOf(BASES[256]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff(
      TestInfo testInfo) {
    expProgramOf(BASES[257]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff(
      TestInfo testInfo) {
    expProgramOf(BASES[258]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffff(
      TestInfo testInfo) {
    expProgramOf(BASES[259]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[260]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[261]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[262]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[263]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[264]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[265]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[266]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[267]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[268]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[269]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[270]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[271]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[272]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[273]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[274]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[275]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[276]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[277]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[278]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[279]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[280]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[281]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[282]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffffff(TestInfo testInfo) {
    expProgramOf(BASES[283]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffffff(TestInfo testInfo) {
    expProgramOf(BASES[284]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffffff(TestInfo testInfo) {
    expProgramOf(BASES[285]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ffff(TestInfo testInfo) {
    expProgramOf(BASES[286]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ff(TestInfo testInfo) {
    expProgramOf(BASES[287]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_f076b857fa9947c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(
      TestInfo testInfo) {
    expProgramOf(BASES[288]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_76b857fa9947c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(
      TestInfo testInfo) {
    expProgramOf(BASES[289]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_b857fa9947c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(
      TestInfo testInfo) {
    expProgramOf(BASES[290]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_57fa9947c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(
      TestInfo testInfo) {
    expProgramOf(BASES[291]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_fa9947c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[292]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_9947c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[293]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_47c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[294]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_c1f9ec558262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[295]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_f9ec558262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[296]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_ec558262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[297]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_558262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[298]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_8262c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[299]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_62c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[300]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_c72704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[301]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_2704099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[302]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_4099ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[303]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_99ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[304]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_9ca8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[305]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_a8cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[306]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_cd325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[307]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_325566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[308]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_5566f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[309]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_66f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[310]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_f73fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[311]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_3fb99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[312]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_b99238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[313]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_9238102ed171(TestInfo testInfo) {
    expProgramOf(BASES[314]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_38102ed171(TestInfo testInfo) {
    expProgramOf(BASES[315]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_102ed171(TestInfo testInfo) {
    expProgramOf(BASES[316]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_2ed171(TestInfo testInfo) {
    expProgramOf(BASES[317]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_d171(TestInfo testInfo) {
    expProgramOf(BASES[318]).run(chainConfig, testInfo);
  }

  @Test
  void expTestForBase_71(TestInfo testInfo) {
    expProgramOf(BASES[319]).run(chainConfig, testInfo);
  }

  // Disabled tests due to length of time to run
  @Disabled
  @ParameterizedTest
  @MethodSource("expWithEvenBaseAndComplexExponentTestSource")
  public void expWithEvenBaseAndComplexExponentTest(
      String base, String exponent, TestInfo testInfo) {
    expProgramOf(base, exponent).run(chainConfig, testInfo);
  }

  static Stream<Arguments> expWithEvenBaseAndComplexExponentTestSource() {
    return generateSource(EVEN_BASES, COMPLEX_EXPONENTS);
  }

  @Disabled
  @ParameterizedTest
  @MethodSource("expWithEvenBaseAndSpecialExponentTestSource")
  public void expWithEvenBaseAndSpecialExponentTest(
      String base, String exponent, TestInfo testInfo) {
    expProgramOf(base, exponent).run(chainConfig, testInfo);
  }

  static Stream<Arguments> expWithEvenBaseAndSpecialExponentTestSource() {
    return generateSource(EVEN_BASES, SPECIAL_EXPONENTS);
  }

  @Disabled
  @ParameterizedTest
  @MethodSource("expWithSimpleOddBaseAndComplexExponentTestSource")
  public void expWithSimpleOddBaseAndComplexExponentTest(
      String base, String exponent, TestInfo testInfo) {
    expProgramOf(base, exponent).run(chainConfig, testInfo);
  }

  static Stream<Arguments> expWithSimpleOddBaseAndComplexExponentTestSource() {
    return generateSource(SIMPLE_ODD_BASES, COMPLEX_EXPONENTS);
  }

  @Disabled
  @ParameterizedTest
  @MethodSource("expWithSimpleOddBaseAndSpecialExponentTestSource")
  public void expWithSimpleOddBaseAndSpecialExponentTest(
      String base, String exponent, TestInfo testInfo) {
    expProgramOf(base, exponent).run(chainConfig, testInfo);
  }

  static Stream<Arguments> expWithSimpleOddBaseAndSpecialExponentTestSource() {
    return generateSource(SIMPLE_ODD_BASES, SPECIAL_EXPONENTS);
  }

  @Disabled
  @ParameterizedTest
  @MethodSource("expWithOtherOddBaseAndComplexExponentTestSource")
  public void expWithOtherOddBaseAndComplexExponentTest(
      String base, String exponent, TestInfo testInfo) {
    expProgramOf(base, exponent).run(chainConfig, testInfo);
  }

  static Stream<Arguments> expWithOtherOddBaseAndComplexExponentTestSource() {
    return generateSource(OTHER_ODD_BASES, COMPLEX_EXPONENTS);
  }

  @Disabled
  @ParameterizedTest
  @MethodSource("expWithOtherOddBaseAndSpecialExponentTestSource")
  public void expWithOtherOddBaseAndSpecialExponentTest(
      String base, String exponent, TestInfo testInfo) {
    expProgramOf(base, exponent).run(chainConfig, testInfo);
  }

  static Stream<Arguments> expWithOtherOddBaseAndSpecialExponentTestSource() {
    return generateSource(OTHER_ODD_BASES, SPECIAL_EXPONENTS);
  }

  // Support methods
  private BytecodeRunner expProgramOf(String base, String exponent) {
    return BytecodeRunner.of(
        BytecodeCompiler.newProgram(chainConfig)
            .push(exponent)
            .push(base)
            .op(OpCode.EXP)
            .compile());
  }

  private BytecodeRunner expProgramOf(String base) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    for (String exponent : EXPONENTS) {
      program.push(exponent).push(base).op(OpCode.EXP);
    }
    return BytecodeRunner.of(program.compile());
  }

  static Stream<Arguments> generateSource(String[] bases, String[] exponents) {
    List<Arguments> arguments = new ArrayList<>();
    for (String base : bases) {
      for (String exponent : exponents) {
        arguments.add(Arguments.of(base, exponent));
      }
    }
    return arguments.stream();
  }
}
