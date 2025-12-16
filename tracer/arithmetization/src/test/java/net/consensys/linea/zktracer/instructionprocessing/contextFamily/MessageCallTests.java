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
package net.consensys.linea.zktracer.instructionprocessing.contextFamily;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendCall;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MultiOpCodeSmcs.allContextOpCodesSmc;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
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
public class MessageCallTests extends TracerTestBase {

  /**
   * This test serves to verify that context data is correctly initialized after a CALL-type
   * instruction.
   */
  @ParameterizedTest
  @EnumSource(
      value = OpCode.class,
      names = {"CALL", "CALLCODE", "DELEGATECALL", "STATICCALL"})
  @ExtendWith(UnitTestWatcher.class)
  public void testWithCall(OpCode opCode, TestInfo testInfo) {

    ToyAccount recipientAccount = buildRecipient(opCode);

    List<ToyAccount> accounts = new ArrayList<>();
    accounts.add(userAccount);
    accounts.add(allContextOpCodesSmc(chainConfig));
    accounts.add(recipientAccount);

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .transaction(buildTransaction(recipientAccount))
        .accounts(accounts)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  /**
   * The recipient will see only its return data change. We take the opportunity to test context
   * data, too.
   *
   * @return
   */
  private ToyAccount buildRecipient(OpCode callOpCode) {

    BytecodeCompiler recipientCode = BytecodeCompiler.newProgram(chainConfig);
    recipientCode.op(CALLDATASIZE);
    recipientCode.op(RETURNDATASIZE);
    recipientCode.op(CALLER);
    recipientCode.op(ADDRESS);
    recipientCode.op(CALLVALUE);
    appendCall(
        recipientCode,
        callOpCode,
        100_000,
        allContextOpCodesSmc(chainConfig).getAddress(),
        1664,
        13,
        91,
        51,
        77);
    recipientCode.op(CALLDATASIZE);
    recipientCode.op(RETURNDATASIZE);

    return ToyAccount.builder()
        .balance(Wei.of(500_000L))
        .code(recipientCode.compile())
        .nonce(891)
        .address(Address.fromHexString("c0dec0ffee31"))
        .build();
  }

  private Transaction buildTransaction(ToyAccount recipientAccount) {
    return ToyTransaction.builder()
        .sender(userAccount)
        .to(recipientAccount)
        .value(Wei.of(3_000_000L))
        .payload(Bytes.fromHexString("0xaabb3311ee88"))
        .gasLimit(1_000_000L)
        .keyPair(keyPair)
        .nonce(userAccount.getNonce())
        .gasPrice(Wei.of(8L))
        .build();
  }
}
