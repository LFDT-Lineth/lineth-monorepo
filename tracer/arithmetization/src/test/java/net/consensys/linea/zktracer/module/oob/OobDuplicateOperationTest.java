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

package net.consensys.linea.zktracer.module.oob;

import static org.junit.jupiter.api.Assertions.assertEquals;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class OobDuplicateOperationTest extends TracerTestBase {
  @Test
  void testCallDataLoadDuplicate(TestInfo testInfo) {
    final BytecodeRunner code =
        BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(1)
                .op(OpCode.CALLDATALOAD)
                .op(OpCode.POP)
                .push(1)
                .op(OpCode.CALLDATALOAD)
                .op(OpCode.POP)
                .op(OpCode.STOP)
                .compile());
    code.run(chainConfig, testInfo);

    assertEquals(1, code.getHub().oob().operations().getAll().size());
  }
}
