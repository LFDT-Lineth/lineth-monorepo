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

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/**
 * the purpose of these tests is to track balance updates for the sender, the coinbase and, in case
 * of a reverted transaction, the recipient.
 */
@ExtendWith(UnitTestWatcher.class)
public class BalanceTests extends TracerTestBase {

  @Test
  void unrevertedValueTransfer(TestInfo testInfo) {

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(stopTransaction)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  @Test
  void revertedValueTransferTest(TestInfo testInfo) {

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(revertTransaction)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  KeyPair keyPair = new SECP256K1().generateKeyPair();
  Address senderAddress = Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
  ToyAccount senderAccount =
      ToyAccount.builder().balance(Wei.of(2_000_001L)).nonce(23).address(senderAddress).build();

  ToyAccount stopAccount =
      ToyAccount.builder()
          .balance(Wei.of(500_000L))
          .nonce(23)
          .code(Bytes.of(0)) // bytecode = STOP
          .address(Address.fromHexString("0xadd7e55"))
          .build();

  Bytes revertByteCode =
      BytecodeCompiler.newProgram(chainConfig).push(0).push(0).op(OpCode.REVERT).compile();

  ToyAccount revertAccount =
      ToyAccount.builder()
          .balance(Wei.of(500_000L))
          .nonce(37)
          .code(revertByteCode) // bytecode = two push 0's and a REVERT
          .address(Address.fromHexString("badadd7e55bad"))
          .build();

  List<ToyAccount> accounts = List.of(senderAccount, stopAccount, revertAccount);

  Transaction stopTransaction =
      ToyTransaction.builder()
          .sender(senderAccount)
          .to(stopAccount)
          .transactionType(TransactionType.FRONTIER)
          .value(Wei.of(1_000_000L))
          .keyPair(keyPair)
          .gasLimit(125_000L)
          .gasPrice(Wei.of(8)) // total 1 million wei in gas
          .build();

  Transaction revertTransaction =
      ToyTransaction.builder()
          .sender(senderAccount)
          .to(revertAccount)
          .transactionType(TransactionType.FRONTIER)
          .value(Wei.of(1_000_000L))
          .keyPair(keyPair)
          .gasLimit(125_000L)
          .gasPrice(Wei.of(8)) // total 1 million wei in gas
          .build();
}
