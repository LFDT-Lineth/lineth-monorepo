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

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class KeccakTests extends TracerTestBase {
  @Test
  void singleEmptyKeccak(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(0) // size
                .push(0) // offset
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .compile())
        .run(chainConfig, testInfo);
  }

  /** computing KEC("ee ee ... ee"), aligned on 1st byte */
  @Test
  void singleWordKeccakNonAligned(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
                .push(1)
                .op(OpCode.MSTORE)
                .push(32) // size
                .push(1) // offset
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .compile())
        .run(chainConfig, testInfo);
  }

  /** computing KEC("ee ee ... ee"), aligned on 0th byte */
  @Test
  void singleWordKeccakAligned(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
                .push(0)
                .op(OpCode.MSTORE)
                .push(32) // size
                .push(0) // offset
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .compile())
        .run(chainConfig, testInfo);
  }

  /** computing KEC("ee") */
  @Test
  void singleByteKeccak(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push("ee")
                .push(1)
                .op(OpCode.MSTORE8)
                .push(1) // size
                .push(1) // offset
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void testSeveralKeccaks(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(0)
                .push(0)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(64)
                .push(13)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(11)
                .push(75)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .push(32)
                .push(32)
                .op(OpCode.SHA3)
                .op(OpCode.POP)
                .compile())
        .run(chainConfig, testInfo);
  }
}
