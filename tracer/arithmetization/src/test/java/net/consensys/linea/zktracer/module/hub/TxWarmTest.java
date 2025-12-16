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

import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.DEFAULT_COINBASE_ADDRESS;
import static net.consensys.linea.zktracer.types.AddressUtils.getCreateRawAddress;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.*;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class TxWarmTest extends TracerTestBase {

  private static final List<Bytes32> NO_STORAGE_KEYS = List.of();
  private static final Address OxO_ADDRESS = Address.fromHexString("0x0");
  private static final Address FIXED_ADDRESS = Address.fromHexString("0xdeadbeef");
  private static final Bytes32 FIXED_STORAGE_KEYS =
      Bytes32.fromHexString("0x0011223344556677889900112233445566778899001122334455667788990011");

  // sender account
  private static final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
  private static final Address senderAddress =
      Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
  private static final ToyAccount senderAccount =
      ToyAccount.builder().balance(Wei.fromEth(123)).nonce(12).address(senderAddress).build();

  // receiver account: 0+1 SMC account
  private static final ToyAccount receiverAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .address(Address.fromHexString("0xdead000000000000000000000000000beef"))
          .code(Bytes.fromHexString("0x6001600001"))
          // eq: BytecodeCompiler.newProgram(new TestInfoWithChainConfig(chainConfig,
          // testInfo)).push(1).push(0).op(OpCode.ADD).compile()
          .build();

  @Test
  void inefficientAccessList(TestInfo testInfo) {
    // inefficient access list
    final List<AccessListEntry> accessList = new ArrayList<>(List.of());

    // warming a PRC wo storage keys
    accessList.add(new AccessListEntry(Address.BLAKE2B_F_COMPRESSION, NO_STORAGE_KEYS));

    // warming a PRC with storage keys
    accessList.add(
        new AccessListEntry(Address.MODEXP, List.of(Bytes32.random(), Bytes32.random())));

    // warming 0x00 Address
    accessList.add(new AccessListEntry(OxO_ADDRESS, NO_STORAGE_KEYS));
    accessList.add(new AccessListEntry(OxO_ADDRESS, List.of(Bytes32.ZERO, FIXED_STORAGE_KEYS)));

    // warming dead beef address
    accessList.add(new AccessListEntry(FIXED_ADDRESS, NO_STORAGE_KEYS));

    // warming dead beef address a second time
    accessList.add(new AccessListEntry(FIXED_ADDRESS, NO_STORAGE_KEYS));

    // warming dead beef address a third time, with random storage keys
    accessList.add(new AccessListEntry(FIXED_ADDRESS, List.of(Bytes32.random(), Bytes32.random())));

    // warming dead beef address a fourth time, with defined storage keys
    accessList.add(new AccessListEntry(FIXED_ADDRESS, List.of(FIXED_STORAGE_KEYS)));

    // warming dead beef address a fifth time, with defined storage keys
    accessList.add(
        new AccessListEntry(
            FIXED_ADDRESS,
            List.of(
                FIXED_STORAGE_KEYS,
                Bytes32.random(),
                Bytes32.ZERO,
                Bytes32.random(),
                FIXED_STORAGE_KEYS)));

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(accessList)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void warmSender(TestInfo testInfo) {
    final List<AccessListEntry> accessList = new ArrayList<>(List.of());
    accessList.add(new AccessListEntry(senderAddress, List.of(FIXED_STORAGE_KEYS)));

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(accessList)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void warmReceiver(TestInfo testInfo) {
    final List<AccessListEntry> accessList = new ArrayList<>(List.of());
    accessList.add(
        new AccessListEntry(
            receiverAccount.getAddress(), List.of(Bytes32.ZERO, FIXED_STORAGE_KEYS)));

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(accessList)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void warmCoinbase(TestInfo testInfo) {
    final List<AccessListEntry> accessList = new ArrayList<>(List.of());
    accessList.add(
        new AccessListEntry(
            DEFAULT_COINBASE_ADDRESS, List.of(Bytes32.ZERO, FIXED_STORAGE_KEYS, Bytes32.random())));

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(accessList)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void warmDeploymentAddress(TestInfo testInfo) {
    final Bytes initCode =
        BytecodeCompiler.newProgram(chainConfig).push(1).push(0).op(OpCode.SLT).compile();

    final Address depAddress =
        Address.extract(getCreateRawAddress(senderAddress, senderAccount.getNonce()));

    final List<AccessListEntry> accessList = new ArrayList<>(List.of());
    accessList.add(
        new AccessListEntry(
            depAddress, List.of(Bytes32.ZERO, FIXED_STORAGE_KEYS, Bytes32.random())));

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .payload(initCode)
            .gasLimit(300000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(accessList)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }
}
