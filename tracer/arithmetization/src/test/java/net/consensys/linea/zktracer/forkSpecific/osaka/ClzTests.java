/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.forkSpecific.osaka;

import static net.consensys.linea.zktracer.Trace.EVM_INST_CLZ;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import org.apache.tuweni.bytes.Bytes32;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class ClzTests extends TracerTestBase {
  /** Tests from the EIP: <a href="https://eips.ethereum.org/EIPS/eip-7939#test-cases">...</a> */
  @Test
  void clzOfZeroTest(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes32.EMPTY)
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void clzOfOnlyOneLeadingBitTest(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(
                    Bytes32.fromHexString(
                        "0x8000000000000000000000000000000000000000000000000000000000000000"))
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void clzOfMaxUINT256Test(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes32.repeat((byte) 0xFF))
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void clzOfOx40Test(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(
                    Bytes32.fromHexString(
                        "0x4000000000000000000000000000000000000000000000000000000000000000"))
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void clzOfOx7FFFTest(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(
                    Bytes32.fromHexString(
                        "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"))
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void clzOfBytes32of1Test(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(
                    Bytes32.fromHexString(
                        "0x0000000000000000000000000000000000000000000000000000000000000001"))
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void clzOf1Test(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(1)
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void clzOf2Test(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(2)
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  @Tag("weekly")
  @ParameterizedTest
  @MethodSource("allBitTestForClzSource")
  void extensiveBitPossibilityForClz(Bytes32 i, TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(i)
                .immediate(EVM_INST_CLZ) // CLZ
                .compile())
        .run(chainConfig, testInfo);
  }

  private static Stream<Arguments> allBitTestForClzSource() {
    final Bytes32 maxBytes32 = Bytes32.repeat((byte) 0xff);
    final List<Arguments> allBitPosition = new ArrayList<>();
    for (int k = 0; k <= 256; k++) {
      allBitPosition.add(Arguments.of(maxBytes32.shiftRight(k)));
    }
    return allBitPosition.stream();
  }
}
