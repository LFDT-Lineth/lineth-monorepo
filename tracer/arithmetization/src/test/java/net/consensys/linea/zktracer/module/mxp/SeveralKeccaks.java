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
package net.consensys.linea.zktracer.module.mxp;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class SeveralKeccaks extends TracerTestBase {

  @Test
  public void testMul(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig).push(32).push(7).op(OpCode.MUL).compile())
        .run(chainConfig, testInfo);
  }

  /** For readability we write __ instead of 00 */
  @Test
  void testIsTheBeefDeadYet(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push("deadbeef") // 4 bytes
                .push(28 * 8)
                .op(OpCode.SHL) //  stack = [ 0x DE AD BE EF __ ... __]
                .op(OpCode.DUP1) // stack = [ 0x DE AD BE EF __ ... __, 0x DE AD BE EF __ ... __]
                .push(1)
                .op(OpCode.MSTORE) // memory looks like so 0x __ DE AD BE EF __ __ __ ...
                .push(6)
                .op(OpCode.MSTORE) // memory looks like so 0x __ DE AD BE EF __ DE AD BE EF __ __ __
                // ...
                .push(4)
                .push(1)
                .op(OpCode.SHA3) // KEC(0xDEADBEEF)
                .push(5)
                .push(0)
                .op(OpCode.SHA3) // KEC(0x00DEADBEEF)
                .push(6)
                .push(0)
                .op(OpCode.SHA3) // KEC(0x00DEADBEEF00)
                .push(5)
                .push(1)
                .op(OpCode.SHA3) // KEC(0xDEADBEEF00)
                .push(4)
                .push(6)
                .op(OpCode.SHA3) // KEC(0xDEADBEEF)
                .push(5)
                .push(6)
                .op(OpCode.SHA3) // KEC(0xDEADBEEF00)
                .push(6)
                .push(5)
                .op(OpCode.SHA3) // KEC(0x00DEADBEEF00)
                .push(5)
                .push(5)
                .op(OpCode.SHA3) // KEC(0x00DEADBEEF)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testSeveralKeccaks(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(0)
                .push(0)
                .op(OpCode.SHA3) // empty hash, no memory expansion
                .op(OpCode.POP)
                .push(0)
                .push(31)
                .op(OpCode.SHA3) // empty hash, no memory expansion
                .op(OpCode.POP)
                .push(64)
                .push(13)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(0)
                .push(49)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(64)
                .push(13)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(37)
                .push(75)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(32)
                .push(32)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(11)
                .push(75)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .compile())
        .run(chainConfig, testInfo);
  }
}
