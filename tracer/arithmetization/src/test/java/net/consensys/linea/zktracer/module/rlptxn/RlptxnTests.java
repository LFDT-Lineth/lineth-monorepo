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

package net.consensys.linea.zktracer.module.rlptxn;

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static org.hyperledger.besu.datatypes.TransactionType.*;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.*;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.TestInstance;
import org.junit.jupiter.api.parallel.Execution;
import org.junit.jupiter.api.parallel.ExecutionMode;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@TestInstance(TestInstance.Lifecycle.PER_CLASS)
@Execution(ExecutionMode.CONCURRENT)
public class RlptxnTests extends TracerTestBase {

  /**
   * This test is a parameterized test that tests the RLP encoding of a transaction, checking
   * different combinations of : - transaction types, - values, - payloads, - access lists, - chain
   * id (empty or not), - signatures.
   */
  private final Random SEED = new Random(666);

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("rlpInputs")
  void testRlpTxn(
      TransactionType type,
      boolean isDeployment,
      BigInteger value,
      Bytes payload,
      List<AccessListEntry> accessList,
      boolean chainLess,
      TestInfo testInfo) {

    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.wrap(Bytes.random(14, SEED)))
            .address(senderAddress)
            .build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .address(Address.wrap(Bytes.random(Address.SIZE, SEED)))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .op(CALLDATASIZE)
                    .push(0)
                    .push(0)
                    .op(CALLDATACOPY)
                    .push(0)
                    .op(MLOAD)
                    .push(17)
                    .op(MLOAD)
                    .op(ADD)
                    .compile())
            .balance(Wei.ONE)
            .build();

    var txBuilder =
        ToyTransaction.builder()
            .sender(senderAccount)
            .gasLimit(1000000L)
            .keyPair(senderKeyPair)
            .transactionType(type)
            .value(Wei.of(value))
            .to(isDeployment ? null : recipientAccount)
            .payload(payload)
            .accessList(type == FRONTIER ? null : accessList);

    final Transaction transaction = chainLess ? txBuilder.chainId(null).build() : txBuilder.build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount))
        .transaction(transaction)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  private Stream<Arguments> rlpInputs() {
    final List<Arguments> arguments = new ArrayList<>();

    final List<TransactionType> possibleTxType = List.of(FRONTIER, ACCESS_LIST, EIP1559);
    final List<BigInteger> values =
        List.of(
            BigInteger.ZERO,
            BigInteger.ONE,
            BigInteger.valueOf(127),
            BigInteger.valueOf(128),
            BigInteger.valueOf(129),
            Bytes.random(8, SEED).toUnsignedBigInteger() // random medium BigInt
            );

    final List<Bytes> payloads =
        List.of(
            Bytes.EMPTY,
            Bytes.of(0),
            Bytes.of(1),
            bigIntegerToBytes(BigInteger.valueOf(127)),
            bigIntegerToBytes(BigInteger.valueOf(128)),
            bigIntegerToBytes(BigInteger.valueOf(129)),
            bigIntegerToBytes(BigInteger.valueOf(255)),
            bigIntegerToBytes(BigInteger.valueOf(256)),
            bigIntegerToBytes(BigInteger.valueOf(257)),
            Bytes.random(55, SEED),
            Bytes.random(56, SEED),
            Bytes.random(57, SEED),
            Bytes.random(255, SEED),
            Bytes.random(256, SEED),
            Bytes.random(257, SEED));

    final Address address1 = Address.wrap(Bytes.random(Address.SIZE, SEED));
    final Address address2 = Address.SHA256;
    final Address address3 = Address.ZERO;
    final Bytes32 storage1 = Bytes32.wrap(Bytes.random(WORD_SIZE, SEED));
    final Bytes32 storage2 = Bytes32.wrap(Bytes.random(WORD_SIZE, SEED));
    final Bytes32 storage3 = Bytes32.ZERO;

    final AccessListEntry entry1 = new AccessListEntry(address1, List.of());
    final AccessListEntry entry2 = new AccessListEntry(address1, List.of(storage1));
    final AccessListEntry entry3 = new AccessListEntry(address2, List.of(storage1));
    final AccessListEntry entry4 =
        new AccessListEntry(
            address3,
            List.of(
                storage1, storage1, storage2, storage3, storage3, storage2, storage1, storage3));

    final List<List<AccessListEntry>> accessLists =
        List.of(
            List.of(), // empty AccessList
            List.of(entry1), // accessList byte size < 56
            List.of(entry2), // 1 address, 1 storage, byte size < 56
            List.of(
                entry1, entry2, entry3, entry4, entry1,
                entry4) // duplicates, warming precompile, stupid stuff, zero address, zero storage
            // ...
            );

    for (TransactionType txType : possibleTxType) {
      for (int isDeployment = 0; isDeployment <= 1; isDeployment++) {
        for (BigInteger value : values) {
          for (Bytes payload : payloads) {
            if (txType == FRONTIER) {
              arguments.add(
                  Arguments.of(txType, isDeployment == 1, value, payload, List.of(), false));
              arguments.add(
                  Arguments.of(txType, isDeployment == 1, value, payload, List.of(), true));
            } else {
              for (List<AccessListEntry> accessList : accessLists) {
                arguments.add(
                    Arguments.of(txType, isDeployment == 1, value, payload, accessList, false));
              }
            }
          }
        }
      }
    }
    return arguments.stream();
  }
}
