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

package net.consensys.linea.zktracer.statemanager;

import static net.consensys.linea.testing.BytecodeCompiler.newProgram;
import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.DEFAULT_COINBASE_ADDRESS;

import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.*;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;

/** Those tests are intended to produce LT trace to test the HUB <-> Shomei lookups. */
@Tag("weekly")
public class HubShomeiTests extends TracerTestBase {

  private static final Bytes32 key1 = Bytes32.repeat((byte) 1);
  private static final Bytes32 key2 = Bytes32.repeat((byte) 2);
  private static final Bytes32 value = Bytes32.leftPad(Bytes.fromHexString("0x7a12e0"));

  private static final Address DEFAULT =
      Address.fromHexString("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef");

  private final Bytes SSLOAD1(ChainConfig chainConfig) {
    return newProgram(chainConfig).push(key1).op(OpCode.SLOAD).op(OpCode.POP).compile();
  }

  private Bytes SSTORE1(ChainConfig chainConfig) {
    return newProgram(chainConfig).push(value).push(key1).op(OpCode.SSTORE).compile();
  }

  /**
   * In this test we have two transactions. In the first one we prewarm a storage key, we SSTORE or
   * SLOAD it. In the second transaction we prewarm the same storage key. The aim of this test is to
   * have the bit FIRST_IN_BLOCK and LAST_IN_BLOCK on prewarming rows, not in execution rows
   */
  @ParameterizedTest
  @MethodSource("opcodeProvider")
  void sandwichPrewarming(OpCode opcode, TestInfo testInfo) {

    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.of(100000000)).address(senderAddress).build();

    final Bytes code =
        switch (opcode) {
          case SSTORE -> Bytes.concatenate(SSTORE1(chainConfig));
          case SLOAD -> Bytes.concatenate(SSLOAD1(chainConfig));
          default -> throw new IllegalStateException("Unexpected value: " + opcode);
        };

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.of(10000))
            .address(DEFAULT)
            .code(Bytes.concatenate(code))
            .build();

    final AccessListEntry accessListEntry =
        new AccessListEntry(recipientAccount.getAddress(), List.of(key1));

    final Transaction tx1 =
        ToyTransaction.builder()
            .transactionType(TransactionType.ACCESS_LIST)
            .sender(senderAccount)
            .keyPair(keyPair)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .accessList(List.of(accessListEntry))
            .to(recipientAccount)
            .build();

    final Transaction tx2 =
        ToyTransaction.builder()
            .transactionType(TransactionType.ACCESS_LIST)
            .sender(senderAccount)
            .nonce(senderAccount.getNonce() + 1)
            .keyPair(keyPair)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .accessList(List.of(accessListEntry))
            .payload(newProgram(chainConfig).push(1).push(1).op(OpCode.ADD).compile())
            .build();

    final ToyExecutionEnvironmentV2 executionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, recipientAccount))
            .transactions(List.of(tx1, tx2))
            .build();

    executionEnvironmentV2.run();

    final ZkTracer tracer = executionEnvironmentV2.getZkTracer();
    final Set<Address> addressSeen = tracer.getAddressesSeenByHubForRelativeBlock(1);
    final Map<Address, Set<Bytes32>> storageSeen = tracer.getStoragesSeenByHubForRelativeBlock(1);

    assert (addressSeen.size() == 4 + Fork.numberOfAddressesSeenBySystemTransaction(fork));
    assert (addressSeen.contains(senderAddress));
    assert (addressSeen.contains(recipientAccount.getAddress()));
    assert (addressSeen.contains(DEFAULT_COINBASE_ADDRESS));
    // last address seen is the effective to of teh second tx

    assert (storageSeen.get(DEFAULT).size() == 1);
    assert (storageSeen.get(DEFAULT).contains(key1));
  }

  /** In this test we prewarm two storage keys, but only one will be used during execution */
  @ParameterizedTest
  @MethodSource("opcodeProvider")
  void uselessPrewarming(OpCode opcode, TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.of(100000000)).address(senderAddress).build();

    final Bytes code =
        switch (opcode) {
          case SSTORE -> Bytes.concatenate(SSTORE1(chainConfig));
          case SLOAD -> Bytes.concatenate(SSLOAD1(chainConfig));
          default -> throw new IllegalStateException("Unexpected value: " + opcode);
        };

    final ToyAccount recipientAccount =
        ToyAccount.builder().balance(Wei.of(10000)).address(DEFAULT).code(code).build();

    final AccessListEntry accessListEntry =
        new AccessListEntry(recipientAccount.getAddress(), List.of(key1, key2));

    final Transaction tx =
        ToyTransaction.builder()
            .transactionType(TransactionType.ACCESS_LIST)
            .sender(senderAccount)
            .keyPair(keyPair)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .accessList(List.of(accessListEntry))
            .to(recipientAccount)
            .build();

    final ToyExecutionEnvironmentV2 executionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, recipientAccount))
            .transaction(tx)
            .build();
    executionEnvironmentV2.run();

    final ZkTracer tracer = executionEnvironmentV2.getZkTracer();
    final Set<Address> addressSeen = tracer.getAddressesSeenByHubForRelativeBlock(1);
    final Map<Address, Set<Bytes32>> storageSeen = tracer.getStoragesSeenByHubForRelativeBlock(1);

    assert (addressSeen.size() == 3 + Fork.numberOfAddressesSeenBySystemTransaction(fork));
    assert (addressSeen.contains(senderAddress));
    assert (addressSeen.contains(recipientAccount.getAddress()));
    assert (addressSeen.contains(DEFAULT_COINBASE_ADDRESS));

    assert (storageSeen.get(DEFAULT).size() == 2);
    assert (storageSeen.get(DEFAULT).contains(key1));
    assert (storageSeen.get(DEFAULT).contains(key2));
  }

  private static Stream<OpCode> opcodeProvider() {
    return Stream.of(OpCode.SSTORE, OpCode.SLOAD);
  }
}
