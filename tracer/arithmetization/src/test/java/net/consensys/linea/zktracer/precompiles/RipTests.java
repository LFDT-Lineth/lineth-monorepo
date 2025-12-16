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

package net.consensys.linea.zktracer.precompiles;

import static net.consensys.linea.zktracer.module.limits.precompiles.RipemdBlocks.RIPEMD160_BLOCKSIZE;
import static net.consensys.linea.zktracer.module.limits.precompiles.RipemdBlocks.numberOfRipemd160Blocks;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class RipTests extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("sizes")
  void basicRipTest(int size, TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(Bytes.fromHexString("0x0badb077")) // value, some random data to hash
            .push(5) // offset
            .op(OpCode.MSTORE)
            .push(14) // return size
            .push(0) // return offset
            .push(size) // size
            .push(1) // offset
            .push(0) // value
            .push(Address.RIPEMD160) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            // To check memory consistency
            .push(0)
            .op(OpCode.MLOAD)
            .op(OpCode.STOP)
            .compile();

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    // if size is 0, no RIP call is made
    // if size is huge, we get an OOGX, so no RIP call made
    final boolean noRipCAll = size == 0 || size == HUGE_SIZE;
    assertEquals(
        noRipCAll ? 0 : numberOfRipemd160Blocks(size),
        bytecodeRunner.getHub().ripemdBlocks().lineCount(),
        "Fail at size: " + size);
  }

  private static final int HUGE_SIZE = Integer.MAX_VALUE;

  private static Stream<Arguments> sizes() {
    final List<Arguments> arguments = new ArrayList<>();
    final List<Integer> sizes =
        List.of(
            0,
            1,
            15,
            72,
            RIPEMD160_BLOCKSIZE - 1,
            RIPEMD160_BLOCKSIZE,
            RIPEMD160_BLOCKSIZE + 1,
            HUGE_SIZE);

    for (int size : sizes) {
      arguments.add(Arguments.of(size));
    }
    return arguments.stream();
  }
}
