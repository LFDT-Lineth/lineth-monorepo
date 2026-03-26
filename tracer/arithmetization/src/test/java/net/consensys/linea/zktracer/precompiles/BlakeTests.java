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

import static net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation.BLAKE2f_HASH_OUTPUT_SIZE;
import static org.junit.jupiter.api.Assertions.assertEquals;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class BlakeTests extends TracerTestBase {
  @Test
  void emptyBlakeTest(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(Address.BLAKE2B_F_COMPRESSION.getBytes()) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertEquals(0, bytecodeRunner.getHub().blakeEffectiveCall().lineCount());
    assertEquals(0, bytecodeRunner.getHub().blakeRounds().lineCount());
  }

  @Test
  void basicBlakeTest(TestInfo testInfo) {
    final int round = 10;

    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(Bytes.fromHexString("0x0badb077")) // value, some random data to hash
            .push(5) // offset
            .op(OpCode.MSTORE)
            .push(round) // value = r for Blake call
            .push(3) // offset
            .op(OpCode.MSTORE8)
            .push(BLAKE2f_HASH_OUTPUT_SIZE) // return size
            .push(0) // return offset
            .push(213) // size
            .push(0) // offset
            .push(0) // value
            .push(Address.BLAKE2B_F_COMPRESSION.getBytes()) // address
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
    assertEquals(1, bytecodeRunner.getHub().blakeEffectiveCall().lineCount());
    assertEquals(round, bytecodeRunner.getHub().blakeRounds().lineCount());
  }

  @Test
  void wrongFInputTest(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(Bytes.fromHexString("0x0badb077")) // value, some random data to hash
            .push(5) // offset
            .op(OpCode.MSTORE)
            .push(2) // value = f for Blake call
            .push(212) // offset
            .op(OpCode.MSTORE8)
            .push(BLAKE2f_HASH_OUTPUT_SIZE) // return size
            .push(0) // return offset
            .push(213) // size
            .push(0) // offset
            .push(0) // value
            .push(Address.BLAKE2B_F_COMPRESSION.getBytes()) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertEquals(0, bytecodeRunner.getHub().blakeEffectiveCall().lineCount());
    assertEquals(0, bytecodeRunner.getHub().blakeRounds().lineCount());
  }

  @Test
  void notEnoughGasBlakeTest(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(Bytes.fromHexString("0x0badb077")) // value, some random data to hash
            .push(5) // offset
            .op(OpCode.MSTORE)
            .push(10) // value = r * 256 ** 3  for Blake call
            .push(0) // offset
            .op(OpCode.MSTORE8)
            .push(BLAKE2f_HASH_OUTPUT_SIZE) // return size
            .push(0) // return offset
            .push(213) // size
            .push(0) // offset
            .push(0) // value
            .push(Address.BLAKE2B_F_COMPRESSION.getBytes()) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertEquals(0, bytecodeRunner.getHub().blakeEffectiveCall().lineCount());
    assertEquals(0, bytecodeRunner.getHub().blakeRounds().lineCount());
  }
}
