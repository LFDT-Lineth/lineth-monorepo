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
package net.consensys.linea.zktracer.precompiles;

import static org.junit.jupiter.api.Assertions.assertEquals;

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
public class EcrecoverTests extends TracerTestBase {

  @Test
  void basicEcrecoverTest(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(0x01) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // Check that the line count is made
    assertEquals(0, bytecodeRunner.getHub().ecRecoverEffectiveCall().lineCount());
  }

  @Test
  void insufficientGasEcrecoverTest(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(0x01) // address
            .push(0x0bb7) // gas; note 0x0bb8 ⇔ 3000
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // Check that the line count is made
    assertEquals(0, bytecodeRunner.getHub().ecRecoverEffectiveCall().lineCount());
  }
}
