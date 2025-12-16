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
package net.consensys.linea.zktracer.instructionprocessing;

import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class JumpTest extends TracerTestBase {

  // NOTE: the bytecode we propose will at time use the following offsets (unless pcNew is large)
  // bytecode:
  // - PUSHX pcNew // offsets: 0, 1
  // - JUMP // offset: 2
  // - INVALID // offset: 3
  // - JUMPDEST // offset: 4
  // - PUSH1 0x5b // offsets: 5, 6 <- 0x5b is the byte value of JUMPDEST
  @ParameterizedTest
  @MethodSource("provideJumpScenario")
  void jumpScenarioTest(String description, String pcNew, TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(pcNew)
            .op(OpCode.JUMP)
            .op(OpCode.INVALID)
            .op(OpCode.JUMPDEST)
            .push(OpCode.JUMPDEST.byteValue()) // false JUMPDEST
            .compile();
    System.out.println(bytecode.toHexString());
    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }

  private static Stream<Arguments> provideJumpScenario() {
    return Stream.of(
        Arguments.of("jumpOntoValidJumpDestination", "4"),
        Arguments.of("jumpOntoINVALID", "3"),
        Arguments.of("jumpOntoJumpDestByteOwnedBySomePush", "6"),
        Arguments.of("jumpOutOfBoundsSmall", "ff"),
        Arguments.of("jumpOutOfBoundsMaxUint128", "ffffffffffffffffffffffffffffffff"),
        Arguments.of("jumpOutOfBoundsTwoToThe128", "0100000000000000000000000000000000"),
        Arguments.of("jumpOutOfBoundsTwoToThe128Plus4", "0100000000000000000000000000000004"),
        Arguments.of(
            "jumpOutOfBoundsMaxUint256",
            "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"));
    /*
    In order to run a specific test case (for example, the first one) use the following:
    .filter(arguments -> "jumpOntoValidJumpDestination".equals(arguments.get()[0]));
      */
  }

  @Test
  void simplestJumpiTest(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(1) // pc = 0, 1
            .push(8) // pc = 2, 3
            .op(OpCode.JUMPI) // pc = 4
            .op(OpCode.INVALID) // pc = 5
            .op(OpCode.JUMPDEST) // pc = 6, 6 ≡ true JUMPDEST
            .push(OpCode.JUMPDEST.byteValue()) // pc = 7, 8,  8 ≡ false JUMPDEST
            .compile();
    System.out.println(bytecode.toHexString());
    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }
}
