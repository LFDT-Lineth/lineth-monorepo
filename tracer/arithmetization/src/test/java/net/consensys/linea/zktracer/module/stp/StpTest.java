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

package net.consensys.linea.zktracer.module.stp;

import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.longToBytes;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.Random;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
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
public class StpTest extends TracerTestBase {
  /**
   * NOTE: Do not make this static as it will introduce non-deterministic behaviour into the testing
   * process.
   */
  private final Random RAND = new Random(666L);

  private final long SENDER_BALANCE = 0xFFFFFFFFFFFFL;

  @Test
  void testCall(TestInfo testInfo) {
    final int NB_CALL = 200;

    final List<ToyAccount> accounts = new ArrayList<>();
    final List<Transaction> transactions = new ArrayList<>();

    for (int i = 0; i < NB_CALL; i++) {
      final OpCode opcode = randOpCodeCall();
      final boolean toExists = RAND.nextBoolean();
      final boolean toWarm = toExists && RAND.nextBoolean();
      final Wei balance = Wei.of(SENDER_BALANCE);
      final long gasCall = RAND.nextLong(0, 100000L);
      final BigInteger value = BigInteger.valueOf(RAND.nextLong(0, 100000L));
      final long gasLimit = RAND.nextLong(23400, 1000000L);

      transactions.add(
          txCall(opcode, toExists, toWarm, balance, value, gasCall, gasLimit, accounts));
    }

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transactions(transactions)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  @Test
  void testCreate(TestInfo testInfo) {
    final int NB_CREATE = 200;

    final List<ToyAccount> world = new ArrayList<>();
    final List<Transaction> txList = new ArrayList<>();

    for (int i = 0; i < NB_CREATE; i++) {

      final OpCode opcode = RAND.nextBoolean() ? OpCode.CREATE : OpCode.CREATE2;
      if (opcode == OpCode.CREATE) {
        txList.add(txCreate(world));
      } else {
        txList.add(txCreate2(world));
      }
    }

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(world)
        .transactions(txList)
        .transactionProcessingResultValidator(TransactionProcessingResultValidator.EMPTY_VALIDATOR)
        .build()
        .run();
  }

  OpCode randOpCodeCall() {
    final int rand = RAND.nextInt(0, 4);
    switch (rand) {
      case 0 -> {
        return OpCode.CALL;
      }
      case 1 -> {
        return OpCode.CALLCODE;
      }
      case 2 -> {
        return OpCode.DELEGATECALL;
      }
      case 3 -> {
        return OpCode.STATICCALL;
      }
      default -> throw new IllegalArgumentException("Arguments is between 0 and 4");
    }
  }

  final Transaction txCall(
      OpCode opcode,
      boolean toExist,
      boolean toWarm,
      Wei balance,
      BigInteger value,
      long gasCall,
      long gasLimit,
      List<ToyAccount> world) {

    // Create the sender account
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(balance).nonce(1).address(senderAddress).build();
    world.add(senderAccount);

    // Create the callee account and address
    final Address calleeAddress = Address.wrap(Bytes.random(20));
    final ToyAccount calleeAccount =
        ToyAccount.builder().nonce(1).balance(Wei.ONE).address(calleeAddress).build();
    if (toExist) {
      world.add(calleeAccount);
    }

    // Create the to account, which contains the bytecode to execute
    final Address toAddress = Address.wrap(Bytes.random(20));
    final ToyAccount toAccount =
        ToyAccount.builder()
            .address(toAddress)
            .nonce(1)
            .balance(Wei.ONE)
            .code(codeCall(opcode, calleeAddress, value, gasCall))
            .build();
    world.add(toAccount);

    AccessListEntry entry = AccessListEntry.createAccessListEntry(senderAddress, List.of());
    if (toWarm) {
      entry = AccessListEntry.createAccessListEntry(calleeAddress, List.of());
    }

    return ToyTransaction.builder()
        .sender(senderAccount)
        .to(toAccount)
        .keyPair(keyPair)
        .transactionType(TransactionType.ACCESS_LIST)
        .value(Wei.of(100000L))
        .gasLimit(gasLimit)
        .accessList(List.of(entry))
        .build();
  }

  private Bytes codeCall(OpCode opcode, Address calleeAddress, BigInteger value, long gasCall) {
    return switch (opcode) {
      case CALL, CALLCODE ->
          BytecodeCompiler.newProgram(chainConfig)
              .push(Bytes.minimalBytes(6)) // retLength
              .push(Bytes.minimalBytes(5)) // terOffset
              .push(Bytes.minimalBytes(4)) // argsLength
              .push(Bytes.minimalBytes(3)) // argsOffset
              .push(bigIntegerToBytes(value)) // value
              .push(calleeAddress) // address
              .push(longToBytes(gasCall)) // gas
              .op(opcode)
              .compile();
      case DELEGATECALL, STATICCALL ->
          BytecodeCompiler.newProgram(chainConfig)
              .push(Bytes.minimalBytes(5)) // retLength
              .push(Bytes.minimalBytes(4)) // terOffset
              .push(Bytes.minimalBytes(3)) // argsLength
              .push(Bytes.minimalBytes(2)) // argsOffset
              .push(calleeAddress) // address
              .push(longToBytes(gasCall)) // gas
              .op(opcode)
              .compile();

      default -> throw new IllegalStateException("Unexpected value: " + opcode);
    };
  }

  final Transaction txCreate(List<ToyAccount> world) {
    // create sender account
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final long value = RAND.nextLong();
    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.of(SENDER_BALANCE))
            .nonce(Math.abs(RAND.nextInt()))
            .address(senderAddress)
            .build();
    world.add(senderAccount);

    // create to account
    final Address to = Address.wrap(Bytes.random(20));
    final ToyAccount toAccount =
        ToyAccount.builder()
            .nonce(Math.abs(RAND.nextInt()))
            .balance(Wei.ONE)
            .address(to)
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .push(Bytes.fromHexString("0xff")) // length
                    .push(Bytes.fromHexString("0x80")) // offset
                    .push(Bytes.minimalBytes(value)) // value
                    .op(OpCode.CREATE)
                    .compile())
            .build();
    world.add(toAccount);

    final long gasLimit = RAND.nextLong(21000, 1000000L);
    return ToyTransaction.builder()
        .sender(senderAccount)
        .keyPair(keyPair)
        .transactionType(TransactionType.FRONTIER)
        .value(Wei.ONE)
        .to(toAccount)
        .gasLimit(gasLimit)
        .build();
  }

  final Transaction txCreate2(List<ToyAccount> world) {
    // create senderAccount
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final long value = RAND.nextLong();
    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.of(SENDER_BALANCE))
            .nonce(Math.abs(RAND.nextInt()))
            .address(senderAddress)
            .build();
    world.add(senderAccount);

    // create to account
    final Address to = Address.wrap(Bytes.random(20));
    final ToyAccount toAccount =
        ToyAccount.builder()
            .nonce(Math.abs(RAND.nextInt()))
            .balance(Wei.ONE)
            .address(to)
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .push(Bytes.random(32)) // salt
                    .push(Bytes.fromHexString("0xff")) // length
                    .push(Bytes.fromHexString("0x80")) // offset
                    .push(Bytes.minimalBytes(value)) // value
                    .op(OpCode.CREATE)
                    .compile())
            .build();
    world.add(toAccount);

    final long gasLimit = RAND.nextLong(21000, 1000000L);
    return ToyTransaction.builder()
        .sender(senderAccount)
        .keyPair(keyPair)
        .transactionType(TransactionType.FRONTIER)
        .to(toAccount)
        .gasLimit(gasLimit)
        .value(Wei.ONE)
        .gasLimit(gasLimit)
        .build();
  }
}
