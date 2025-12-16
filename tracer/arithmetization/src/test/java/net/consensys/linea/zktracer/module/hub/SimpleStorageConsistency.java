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
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class SimpleStorageConsistency extends TracerTestBase {

  private final Address receiverAddress =
      Address.fromHexString("0x00000bad0000000000000000000000000000b077");

  final KeyPair senderKeyPair1 = new SECP256K1().generateKeyPair();
  final Address senderAddress1 =
      Address.extract(Hash.hash(senderKeyPair1.getPublicKey().getEncodedBytes()));
  final ToyAccount senderAccount1 =
      ToyAccount.builder().balance(Wei.fromEth(123)).nonce(5).address(senderAddress1).build();

  final KeyPair senderKeyPair2 = new SECP256K1().generateKeyPair();
  final Address senderAddress2 =
      Address.extract(Hash.hash(senderKeyPair2.getPublicKey().getEncodedBytes()));
  final ToyAccount senderAccount2 =
      ToyAccount.builder().balance(Wei.fromEth(1231)).nonce(15).address(senderAddress2).build();

  final KeyPair senderKeyPair3 = new SECP256K1().generateKeyPair();
  final Address senderAddress3 =
      Address.extract(Hash.hash(senderKeyPair3.getPublicKey().getEncodedBytes()));
  final ToyAccount senderAccount3 =
      ToyAccount.builder().balance(Wei.fromEth(1231)).nonce(15).address(senderAddress3).build();

  private final String keyString =
      "0x00010203040060708090A0B0C0DE0F10101112131415161718191A1B1C1D1E1F";
  private final Bytes32 key = Bytes32.fromHexString(keyString);
  private final Bytes32 value1 = Bytes32.repeat((byte) 1);
  private final Bytes32 value2 = Bytes32.repeat((byte) 2);

  final List<String> simpleKey = List.of(keyString);
  final List<String> duplicateKey = List.of(keyString, keyString);
  final List<String> simpleKeyAndTrash =
      List.of(keyString, "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef");

  final List<AccessListEntry> warmOnlyReceiver =
      List.of(AccessListEntry.createAccessListEntry(receiverAddress, simpleKey));

  final List<AccessListEntry> stupidWarmer =
      List.of(
          AccessListEntry.createAccessListEntry(receiverAddress, duplicateKey),
          AccessListEntry.createAccessListEntry(receiverAddress, simpleKeyAndTrash),
          AccessListEntry.createAccessListEntry(senderAddress1, simpleKeyAndTrash),
          AccessListEntry.createAccessListEntry(senderAddress3, simpleKey),
          AccessListEntry.createAccessListEntry(receiverAddress, duplicateKey));

  @Test
  void test(TestInfo testInfo) {
    final ToyAccount receiverAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(receiverAddress)
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // SLOAD initial value
                    .push(key)
                    .op(OpCode.SLOAD)
                    .op(OpCode.POP)
                    // SSTORE value 1
                    .push(value1)
                    .push(key)
                    .op(OpCode.SSTORE)
                    // SSTORE value 2
                    .push(value2)
                    .push(key)
                    .op(OpCode.SSTORE)
                    .compile())
            .nonce(116)
            .build();

    final Transaction simpleWarm =
        ToyTransaction.builder()
            .sender(senderAccount1)
            .to(receiverAccount)
            .keyPair(senderKeyPair1)
            .gasLimit(1000000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(warmOnlyReceiver)
            .value(Wei.of(50000))
            .build();

    final Transaction stupidWarm =
        ToyTransaction.builder()
            .sender(senderAccount2)
            .to(receiverAccount)
            .keyPair(senderKeyPair2)
            .gasLimit(1000000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(stupidWarmer)
            .value(Wei.of(50000))
            .build();

    final Transaction noWarm =
        ToyTransaction.builder()
            .sender(senderAccount3)
            .to(receiverAccount)
            .keyPair(senderKeyPair3)
            .gasLimit(1000000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(List.of())
            .value(Wei.of(50000))
            .build();

    final List<Transaction> txs = List.of(simpleWarm, stupidWarm, noWarm);

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(receiverAccount, senderAccount1, senderAccount2, senderAccount3))
        .transactions(txs)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }
}
