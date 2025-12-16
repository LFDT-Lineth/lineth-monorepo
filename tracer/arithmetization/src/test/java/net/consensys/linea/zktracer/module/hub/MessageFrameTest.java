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

package net.consensys.linea.zktracer.module.hub;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class MessageFrameTest extends TracerTestBase {

  @Test
  void testCreate(TestInfo testInfo) {
    // The pc is not updated as expected
    // We do not execute the init code of the created smart contract
    // TODO: fix this!
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program
        .push("63deadbeef000000000000000000000000000000000000000000000000000000")
        .push(0)
        .op(OpCode.MSTORE)
        .push(0x05)
        .push(0)
        .push(0)
        .op(OpCode.CREATE)
        .op(OpCode.DUP1);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
  }

  @Test
  void testCall(TestInfo testInfo) {
    // Interestingly for CALL the pc is updated as expected
    // We execute the bytecode of the called smart contract
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program.push(0).push(0).push(0).push(0).push(0x01).push("ca11ee").push(0xffff).op(OpCode.CALL);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());

    final ToyAccount smartContractAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(7)
            .address(Address.fromHexString("0xca11ee"))
            .code(Bytes.fromHexString("0x63deadbeef00"))
            .build();

    bytecodeRunner.run(List.of(smartContractAccount), chainConfig, testInfo);
  }
}
