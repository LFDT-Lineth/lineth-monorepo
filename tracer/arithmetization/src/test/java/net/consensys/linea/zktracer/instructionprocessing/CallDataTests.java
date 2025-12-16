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
 * The tests below are designed to test our handling of call data. For deployment transactions, and
 * more generally deployment context's, call data is empty and the tests are "trivial" in some
 * sense. They aren't for message calls.
 */
@ExtendWith(UnitTestWatcher.class)
public class CallDataTests extends TracerTestBase {
  // @Test
  // void transactionCallDataForMessageCallTest(TestInfo testInfo) {
  // }

  // @Test
  // void transactionCallDataForDeploymentTest(TestInfo testInfo) {
  // }

  @Test
  void nonAlignedCallDataInCallTest(TestInfo testInfo) {

    Transaction transaction = transactionCallingCallDataCodeAccount();
    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(transaction)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  // @Test
  // void callDataInCreateTest(TestInfo testInfo) {
  // }

  private final Bytes callData32 =
      Bytes.fromHexString("abcdef01234567890000deadbeef0000aa0f517e002024aa9876543210fedcba");

  Bytes callDataByteCode =
      BytecodeCompiler.newProgram(chainConfig)
          .push(13) // size
          .push(29) // sourceOffset
          .push(17) // targetOffset
          .op(OpCode.CALLDATACOPY)
          .push(0)
          .op(OpCode.MLOAD)
          .push(0)
          .op(OpCode.CALLDATALOAD)
          .push(28)
          .op(OpCode.MSTORE)
          .op(OpCode.MSIZE) // size
          .push(11) // offset
          .op(OpCode.RETURN) // the final instruction will expand memory
          .compile();

  final Bytes callerCode =
      BytecodeCompiler.newProgram(chainConfig)
          .push(callData32)
          .push(2)
          .op(OpCode.MSTORE)
          .push(44) // r@c, shorter than the return data
          .push(19) // r@o, deliberately overlaps with call data
          .push(35) // cds
          .push(1) // cdo
          .push("ca11da7ac0de") // address
          .op(OpCode.GAS) // gas
          .op(OpCode.STATICCALL)
          .compile();

  KeyPair keyPair = new SECP256K1().generateKeyPair();
  Address userAddress = Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
  ToyAccount userAccount =
      ToyAccount.builder().balance(Wei.fromEth(10)).nonce(99).address(userAddress).build();
  ToyAccount targetOfTransaction =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(13)
          .address(Address.fromHexString("ca11ee"))
          .code(callerCode)
          .build();
  ToyAccount callDataCodeAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(13)
          .address(Address.fromHexString("ca11da7ac0de"))
          .code(callDataByteCode)
          .build();

  List<ToyAccount> accounts = List.of(userAccount, callDataCodeAccount, targetOfTransaction);

  Transaction transactionCallingCallDataCodeAccount() {
    return ToyTransaction.builder()
        .sender(userAccount)
        .to(targetOfTransaction)
        .payload(callData32)
        .transactionType(TransactionType.FRONTIER)
        .value(Wei.ONE)
        .keyPair(keyPair)
        .gasLimit(100_000L)
        .gasPrice(Wei.of(8))
        .build();
  }
}
