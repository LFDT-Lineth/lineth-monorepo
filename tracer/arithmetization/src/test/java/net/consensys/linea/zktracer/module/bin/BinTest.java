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

package net.consensys.linea.zktracer.module.bin;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.units.bigints.UInt256;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class BinTest extends TracerTestBase {
  @Test
  public void edgeCase(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig).push(0xf0).push(0xf0).op(OpCode.AND).compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testSignedSignextend(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(UInt256.MAX_VALUE)
                .push(UInt256.MAX_VALUE)
                .op(OpCode.SIGNEXTEND)
                .op(OpCode.POP)
                .push(UInt256.valueOf(31))
                .push(UInt256.MAX_VALUE)
                .op(OpCode.SIGNEXTEND)
                .op(OpCode.POP)
                .push(UInt256.valueOf(32))
                .push(UInt256.MAX_VALUE)
                .op(OpCode.SIGNEXTEND)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testSignextendRef(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(0xFF)
                .push(0)
                .op(OpCode.SIGNEXTEND)
                .op(OpCode.POP)
                .push(0x7F)
                .push(0)
                .op(OpCode.SIGNEXTEND)
                .op(OpCode.POP)
                .compile())
        .run(chainConfig, testInfo);
  }
}
