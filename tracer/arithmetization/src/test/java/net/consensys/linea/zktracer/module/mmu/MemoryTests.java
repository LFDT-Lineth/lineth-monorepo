/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module.mmu;

import static net.consensys.linea.testing.BytecodeCompiler.newProgram;

import java.util.Random;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

class MemoryTests extends TracerTestBase {
  private final Random rnd = new Random(666);

  @Test
  void successionOverlappingMstore(TestInfo testInfo) {
    BytecodeRunner.of(
            newProgram(chainConfig)
                .push(Bytes.repeat((byte) 1, 32))
                .push(0)
                .op(OpCode.MSTORE)
                .push(Bytes.repeat((byte) 2, 32))
                .push(15)
                .op(OpCode.MSTORE)
                .push(Bytes.repeat((byte) 3, 32))
                .push(2)
                .op(OpCode.MSTORE)
                .push(6)
                .op(OpCode.MLOAD)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void fastMload(TestInfo testInfo) {
    BytecodeRunner.of(newProgram(chainConfig).push(34).push(0).op(OpCode.MLOAD).compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void alignedMstore8(TestInfo testInfo) {
    BytecodeRunner.of(newProgram(chainConfig).push(12).push(0).op(OpCode.MSTORE8).compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void nonAlignedMstore8(TestInfo testInfo) {
    BytecodeRunner.of(newProgram(chainConfig).push(66872).push(35).op(OpCode.MSTORE8).compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void mstoreAndReturn(TestInfo testInfo) {
    BytecodeCompiler program = newProgram(chainConfig);
    program
        .push("deadbeef11111111deadbeef22222222deadbeef00000000deadbeefcccccccc")
        .push(0x20)
        .op(OpCode.MSTORE)
        .push(0x10)
        .push(0x30)
        .op(OpCode.RETURN);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @Test
  void mstoreAndRevert(TestInfo testInfo) {
    BytecodeCompiler program = newProgram(chainConfig);
    program
        .push("deadbeef11111111deadbeef22222222deadbeef00000000deadbeefcccccccc")
        .push(0x20)
        .op(OpCode.MSTORE)
        .push(0x10)
        .push(0x28)
        .op(OpCode.REVERT);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @Test
  void returnAfterLog2(TestInfo testInfo) {
    BytecodeCompiler program = newProgram(chainConfig);
    program
        .push(0x01)
        .push(0x11)
        .op(OpCode.SHA3) // KECCAK("00")
        .push(0x00)
        .op(OpCode.MSTORE)
        .push(0x02)
        .push(0x31)
        .op(OpCode.SHA3) // KECCAK("0000")
        .push(0x20)
        .op(OpCode.MSTORE)
        //
        .push(0xbbbbbbbb) // topic 2
        .push(0xaaaaaaaa) // topic 1
        .push(0x20) // size
        .push(0x10) // offset
        .op(OpCode.LOG2)
        .push("deadbeef00000000deadbeef33333333deadbeefccccccccdeadbeef11111111")
        .push(0x40)
        .op(OpCode.MSTORE)
        .push(0x10)
        .push(0x30)
        .op(OpCode.RETURN);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @Test
  void revertAfterLog2(TestInfo testInfo) {
    BytecodeCompiler program = newProgram(chainConfig);
    program
        .push(0x01)
        .push(0x11)
        .op(OpCode.SHA3) // KECCAK("00")
        .push(0x00)
        .op(OpCode.MSTORE)
        .push(0x02)
        .push(0x31)
        .op(OpCode.SHA3) // KECCAK("0000")
        .push(0x20)
        .op(OpCode.MSTORE)
        //
        .push(0xbbbbbbbb) // topic 2
        .push(0xaaaaaaaa) // topic 1
        .push(0x20) // size
        .push(0x10) // offset
        .op(OpCode.LOG2)
        .push("deadbeef00000000deadbeef33333333deadbeefccccccccdeadbeef11111111")
        .push(0x40)
        .op(OpCode.MSTORE)
        .push(0x10)
        .push(0x30)
        .op(OpCode.REVERT);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  @Test
  void checkMSizeAfterMemoryExpansion(TestInfo testInfo) {
    BytecodeCompiler program = newProgram(chainConfig);
    program
        .push(0xFF)
        .push(0)
        .op(OpCode.MSTORE) // expand memory
        .op(OpCode.MSIZE); // call MSIZE on non-zero memory

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }
}
