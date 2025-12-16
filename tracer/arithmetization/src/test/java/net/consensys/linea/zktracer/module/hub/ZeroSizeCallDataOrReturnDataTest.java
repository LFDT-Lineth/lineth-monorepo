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

package net.consensys.linea.zktracer.module.hub;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class ZeroSizeCallDataOrReturnDataTest extends TracerTestBase {

  @Test
  void zeroSizeHugeReturnAtOffsetTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push("ff".repeat(32)) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push("ca11ee") // address
        .push(1000) // gas
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    calleeProgram.op(OpCode.CALLDATASIZE);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner.of(program.compile())
        .run(Wei.fromEth(1), 30000L, List.of(calleeAccount), chainConfig, testInfo);
  }

  @Test
  void zeroSizeHugeCallDataOffsetTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push("ff".repeat(32)) // call data offset
        .push("ca11ee") // address
        .push(1000) // gas
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    calleeProgram.op(OpCode.CALLDATASIZE);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner.of(program.compile())
        .run(Wei.fromEth(1), 30000L, List.of(calleeAccount), chainConfig, testInfo);
  }

  @Test
  void zeroSizeHugeReturnDataOffsetTest(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(0) // return at capacity
        .push(0) // return at offset
        .push(0) // call data size
        .push(0) // call data offset
        .push("ca11ee") // address
        .push(1000) // gas
        .op(OpCode.STATICCALL);

    BytecodeCompiler calleeProgram = BytecodeCompiler.newProgram(chainConfig);
    calleeProgram.push(0).push("ff".repeat(32)).op(OpCode.RETURN);

    final ToyAccount calleeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(10)
            .address(Address.fromHexString("ca11ee"))
            .code(calleeProgram.compile())
            .build();

    BytecodeRunner.of(program.compile())
        .run(Wei.fromEth(1), 30000L, List.of(calleeAccount), chainConfig, testInfo);
  }
}
