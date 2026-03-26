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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.identity;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.EnumSource;

@ExtendWith(UnitTestWatcher.class)
public class Tests extends TracerTestBase {

  // 512 random bytes
  final Bytes randBytes =
      Bytes.fromHexString(
          "fc82e3eacb81bb23c620386ea0427556d93af36b9dec2bcf8642728901f578a9f29cfead7f44372286ac20032e3d83939c4f58a93efee5ddc4a48990dc2cb188694a240b2f6b00386a442ed5368cf030bd2e73e09a44848e9130522b2d7a6a78bbb4554c8923c5be03a49d62c430e7e28d92b18f6db900ea8be85fa0f82fa913e02f106bc1322919f59223465c6accff2d89360f5e38cd4a38872696751af5191ad19123702e68d910b2a0898700a08272e2326b43a6dca195c71db782ab560d31ed7a2d698c4bdcd7fceb252e042160ba5c2929144373ccc3ec089317908b31cfaed98a0a6f49afdf69f23d2c06c308478af4c671385393b0413fe4b93a67b8acac06aafb9d1a4c50732d3bb9bc45a9d799b767158474efa59a3ecba29b7f815589f8f5b6363341ee5c39a492ffc74b3ce61909367570100f8ede71e0ace31780d9eebf8a4fe33fcab4c6efb3e66910574fd8693f1960be4a107be95268493e318aeac36caaae7b0275e372425b765e4c55512aaa4237365d614b48bc0815e095637f14a9bd3cd7eac38130864286c08d25cb94cc953112af1f902c3a5f387ba2ce3a4fc6393ef4c360d22418e127f0d6f6a455b9386aa2c95984cfea90834dcd5de0934c81b4b555d7ae6e02d467fbb2335abdefa741430d8845f73dbb07c71b07cf2a536a14ea80160277153ea6ee552dbe6432fc415df89af463260b3881");

  final ToyAccount byteSource =
      ToyAccount.builder()
          .address(Address.fromHexString("acc0fc0de"))
          .nonce(59)
          .balance(Wei.of(1_000_000L))
          .code(randBytes)
          .build();

  final ToyAccount identityCaller =
      ToyAccount.builder()
          .address(Address.fromHexString("1dca11e7")) // identity caller
          .nonce(103)
          .balance(Wei.of(1_000_000L))
          .code(randBytes)
          .build();

  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  void nontrivialCallDataIdentityTest(OpCode callOpCode, TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    fullCodeCopyOf(program, byteSource);
    appendCall(
        program,
        callOpCode,
        1_000_000,
        Address.ID,
        1_000_000,
        0,
        byteSource.getCode().size(),
        7,
        51);
    program.op(RETURNDATASIZE); // should return 512

    identityCaller.setCode(program.compile());

    Transaction transaction =
        ToyTransaction.builder()
            .sender(userAccount)
            .keyPair(keyPair)
            .to(identityCaller)
            .value(Wei.of(7_000_000_000L))
            .build();
    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(byteSource, userAccount, identityCaller))
        .transaction(transaction)
        .build()
        .run();
  }

  /**
   * The following copies the entirety of the account code to RAM.
   *
   * @param program
   * @param account
   */
  public static void fullCodeCopyOf(BytecodeCompiler program, ToyAccount account) {
    final Address address = account.getAddress();
    program
        .push(address.getBytes())
        .op(EXTCODESIZE) // puts the code size on the stack
        .push(0)
        .push(0)
        .push(address.getBytes())
        .op(EXTCODECOPY); // copies the entire code to RAM
  }
}
