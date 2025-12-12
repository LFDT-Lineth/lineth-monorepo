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

package net.consensys.linea.zktracer.module.add;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class AddTest extends TracerTestBase {
  @Test
  void testSmallZeroAdd(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes.of(0xF1))
                .push(Bytes.EMPTY)
                .op(OpCode.ADD)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testSmallZeroSub(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes.of(0xF1))
                .push(Bytes.EMPTY)
                .op(OpCode.SUB)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testOverflowLoAdd(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes.fromHexString("0xF0F1F2F3F4F5F6F7F8F9FAFBFCFDFEFF"))
                .push(Bytes.fromHexString("0xE0E1E2E3E4E5E6E7E8E9EAEBECEDEEEF"))
                .op(OpCode.ADD)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testHugeSmallAdd(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes.repeat((byte) 0xFF, 32))
                .push(Bytes.of(2))
                .op(OpCode.ADD)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testOverFlowHiAdd(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(
                    Bytes.concatenate(Bytes.repeat((byte) 0xFF, 16), Bytes.repeat((byte) 0x01, 16)))
                .push(
                    Bytes.concatenate(Bytes.repeat((byte) 0x02, 16), Bytes.repeat((byte) 0x01, 16)))
                .op(OpCode.ADD)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testSmallHugeSub(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes.of(2))
                .push(Bytes.repeat((byte) 0xFF, 32))
                .op(OpCode.SUB)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testHugeSmallSub(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(Bytes.repeat((byte) 0xFF, 32))
                .push(Bytes.of(2))
                .op(OpCode.SUB)
                .compile())
        .run(chainConfig, testInfo);
  }
}
