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

package net.consensys.linea.zktracer.instructionprocessing;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class ReturnDataCopyTest extends TracerTestBase {
  @Test
  void testReturnDataCopyFromSha256(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(
                    Bytes.fromHexString(
                        "0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"))
                .push(1)
                .op(OpCode.MSTORE)
                .push(22) // r@c, shorter than the return data
                .push(5) // r@o, deliberately overlaps with call data
                .push(18) // cds
                .push(2) // cdo
                .push(Address.SHA256) // address
                .op(OpCode.GAS) // gas
                .op(OpCode.STATICCALL)
                .push(5)
                .push(5)
                .push(5)
                .op(OpCode.RETURNDATACOPY)
                .push(0)
                .push(0)
                .op(OpCode.MLOAD)
                .op(OpCode.STOP)
                .compile())
        .run(chainConfig, testInfo);
  }
}
