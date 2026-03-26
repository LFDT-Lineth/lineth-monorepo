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

package net.consensys.linea.testing;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.testing.AddressCollisions.senderCoinbaseCollision;
import static net.consensys.linea.zktracer.Trace.*;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.function.Consumer;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.module.hub.Hub;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;

/**
 * A BytecodeRunner takes bytecode, then run it in a single transaction in a single block, and
 * ensures that it executed correctly.
 */
@Accessors(fluent = true)
public final class BytecodeRunner {
  public static final long MAX_GAS_LIMIT =
      EIP_7825_TRANSACTION_GAS_LIMIT_CAP; // = 0x1000000 max tx gas limit since EIP-7825 (OSAKA)
  private final Bytes byteCode;
  @Getter ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2;

  /**
   * @param byteCode the byte code to test
   */
  public BytecodeRunner(Bytes byteCode) {
    this.byteCode = byteCode;
  }

  public static BytecodeRunner of(BytecodeCompiler program) {
    return new BytecodeRunner(program.compile());
  }

  public static BytecodeRunner of(Bytes byteCode) {
    return new BytecodeRunner(byteCode);
  }

  @Setter private Consumer<ZkTracer> zkTracerValidator = zkTracer -> {};

  // Default run method
  public void run(ChainConfig chainConfig, TestInfo testInfo) {
    this.run(
        Wei.fromEth(1), MAX_GAS_LIMIT, List.of(), Bytes.EMPTY, List.of(), chainConfig, testInfo);
  }

  // Ad-hoc senderBalance
  public void run(Wei senderBalance, ChainConfig chainConfig, TestInfo testInfo) {
    this.run(
        senderBalance, MAX_GAS_LIMIT, List.of(), Bytes.EMPTY, List.of(), chainConfig, testInfo);
  }

  // Ad-hoc gasLimit
  public void run(Long gasLimit, ChainConfig chainConfig, TestInfo testInfo) {
    this.run(Wei.fromEth(1), gasLimit, List.of(), Bytes.EMPTY, List.of(), chainConfig, testInfo);
  }

  // Ad-hoc senderBalance and gasLimit
  public void run(Wei senderBalance, Long gasLimit, ChainConfig chainConfig, TestInfo testInfo) {
    this.run(senderBalance, gasLimit, List.of(), Bytes.EMPTY, List.of(), chainConfig, testInfo);
  }

  // Ad-hoc accounts
  public void run(List<ToyAccount> additionalAccounts, ChainConfig chainConfig, TestInfo testInfo) {
    this.run(
        Wei.fromEth(1),
        MAX_GAS_LIMIT,
        additionalAccounts,
        Bytes.EMPTY,
        List.of(),
        chainConfig,
        testInfo);
  }

  // Ad-hoc gasLimit and accounts
  public void run(
      Long gasLimit,
      List<ToyAccount> additionalAccounts,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    this.run(
        Wei.fromEth(1),
        gasLimit,
        additionalAccounts,
        Bytes.EMPTY,
        List.of(),
        chainConfig,
        testInfo);
  }

  // Ad-hoc senderBalance, gasLimit and accounts
  public void run(
      Wei senderBalance,
      Long gasLimit,
      List<ToyAccount> additionalAccounts,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    this.run(
        senderBalance, gasLimit, additionalAccounts, Bytes.EMPTY, List.of(), chainConfig, testInfo);
  }

  public void run(
      Bytes payload, List<AccessListEntry> accessList, ChainConfig chainConfig, TestInfo testInfo) {
    this.run(Wei.fromEth(1), MAX_GAS_LIMIT, List.of(), payload, accessList, chainConfig, testInfo);
  }

  public void runWithAddressCollision(
      Bytes payload,
      List<AccessListEntry> accessList,
      AddressCollisions collision,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    this.runWithAddressCollision(
        Wei.fromEth(1),
        MAX_GAS_LIMIT,
        List.of(),
        payload,
        accessList,
        collision,
        chainConfig,
        testInfo);
  }

  public void run(
      Wei senderBalance,
      Long gasLimit,
      List<ToyAccount> additionalAccounts,
      Bytes payload,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    this.run(
        senderBalance, gasLimit, additionalAccounts, payload, List.of(), chainConfig, testInfo);
  }

  // Ad-hoc senderBalance, gasLimit, accounts and payload
  public void run(
      Wei senderBalance,
      Long gasLimit,
      List<ToyAccount> additionalAccounts,
      Bytes payload,
      List<AccessListEntry> accessList,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    buildToyExecutionEnvironmentV2(
        senderBalance,
        gasLimit,
        additionalAccounts,
        payload,
        accessList,
        AddressCollisions.NO_COLLISION,
        chainConfig,
        testInfo);
    toyExecutionEnvironmentV2.run();
  }

  public void runWithAddressCollision(
      Wei senderBalance,
      Long gasLimit,
      List<ToyAccount> additionalAccounts,
      Bytes payload,
      List<AccessListEntry> accessList,
      AddressCollisions collision,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    buildToyExecutionEnvironmentV2(
        senderBalance,
        gasLimit,
        additionalAccounts,
        payload,
        accessList,
        collision,
        chainConfig,
        testInfo);
    toyExecutionEnvironmentV2.run();
  }

  private void buildToyExecutionEnvironmentV2(
      Wei senderBalance,
      Long gasLimit,
      List<ToyAccount> additionalAccounts,
      Bytes payload,
      List<AccessListEntry> accessList,
      AddressCollisions collision,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    checkArgument(byteCode != null, "byteCode cannot be empty");
    final long transactionValue = 272; // 256 + 16, easier for debugging
    final long gasPrice = 8;

    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(keyPair.getPublicKey());
    final Address recipientAddress =
        Address.fromHexString("0x1111111111111111111111111111111111111111");

    final int senderNonce = 5;

    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(senderBalance)
            .nonce(senderNonce)
            .address(senderAddress)
            .build();

    final Long selectedGasLimit = Optional.of(gasLimit).orElse(MAX_GAS_LIMIT);

    final ToyAccount receiverAccount =
        switch (collision) {
          case SENDER_IS_RECIPIENT, TRIPLE_COLLISION ->
              ToyAccount.builder()
                  // Accounts update are already made in the TX_SKIP section
                  // .balance(senderBalance.subtract(transactionValue + gasPrice *
                  // selectedGasLimit))
                  // .nonce(senderNonce + 1)
                  .address(senderAddress)
                  .build();
          default ->
              ToyAccount.builder()
                  .balance(Wei.fromEth(1))
                  .nonce(23)
                  .address(recipientAddress)
                  .code(byteCode)
                  .build();
        };

    final ToyTransaction.ToyTransactionBuilder txBuilder =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .value(Wei.of(transactionValue)) // 256 + 16, easier for debugging
            .keyPair(keyPair)
            .gasLimit(selectedGasLimit)
            .gasPrice(Wei.of(gasPrice));
    if (!payload.isEmpty()) {
      txBuilder.payload(payload);
    }
    if (!accessList.isEmpty()) {
      txBuilder.accessList(accessList).transactionType(TransactionType.ACCESS_LIST);
    }

    final Transaction tx = txBuilder.build();

    final List<ToyAccount> accounts = new ArrayList<>();
    accounts.add(senderAccount);
    if (collision == AddressCollisions.NO_COLLISION
        || collision == AddressCollisions.RECIPIENT_IS_COINBASE) {
      accounts.add(receiverAccount);
    }
    accounts.addAll(additionalAccounts);

    ToyExecutionEnvironmentV2.ToyExecutionEnvironmentV2Builder toyExecutionEnvironmentV2Builder =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .transactionProcessingResultValidator(
                TransactionProcessingResultValidator.EMPTY_VALIDATOR)
            .accounts(accounts)
            .zkTracerValidator(zkTracerValidator)
            .transaction(tx);

    if (senderCoinbaseCollision(collision)) {
      toyExecutionEnvironmentV2Builder.coinbase(senderAddress);
    } else if (collision == AddressCollisions.RECIPIENT_IS_COINBASE) {
      toyExecutionEnvironmentV2Builder.coinbase(recipientAddress);
    }

    toyExecutionEnvironmentV2 = toyExecutionEnvironmentV2Builder.build();
  }

  public void runInitCode(ChainConfig chainConfig, TestInfo testInfo) {
    checkArgument(byteCode != null, "init code cannot be empty");

    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(keyPair.getPublicKey());

    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(112)).nonce(18).address(senderAddress).build();

    final Transaction tx =
        ToyTransaction.builder()
            .payload(byteCode)
            .gasLimit(MAX_GAS_LIMIT)
            .sender(senderAccount)
            .value(Wei.of(272)) // 256 + 16, easier for debugging
            .keyPair(keyPair)
            .gasPrice(Wei.of(8))
            .build();

    toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .transactionProcessingResultValidator(
                TransactionProcessingResultValidator.EMPTY_VALIDATOR)
            .accounts(List.of(senderAccount))
            .zkTracerValidator(zkTracerValidator)
            .transaction(tx)
            .build();
    toyExecutionEnvironmentV2.run();
  }

  /*
  BytecodeRunner: runOnlyForGasCost section
   */

  // Ad-hoc accounts
  public long runOnlyForGasCost(
      List<ToyAccount> additionalAccounts, ChainConfig chainConfig, TestInfo testInfo) {
    return this.runOnlyForGasCost(
        Wei.fromEth(1),
        (long) LINEA_BLOCK_GAS_LIMIT,
        additionalAccounts,
        Bytes.EMPTY,
        chainConfig,
        testInfo);
  }

  // Ad-hoc payload
  public long runOnlyForGasCost(Bytes payload, ChainConfig chainConfig, TestInfo testInfo) {
    return this.runOnlyForGasCost(
        Wei.fromEth(1), (long) LINEA_BLOCK_GAS_LIMIT, List.of(), payload, chainConfig, testInfo);
  }

  // Ad-hoc payload
  public long runOnlyForGasCost(ChainConfig chainConfig, TestInfo testInfo) {
    return this.runOnlyForGasCost(
        Wei.fromEth(1),
        (long) LINEA_BLOCK_GAS_LIMIT,
        List.of(),
        Bytes.EMPTY,
        chainConfig,
        testInfo);
  }

  // Ad-hoc senderBalance, accounts and payload
  // Uses LondonGasCalculator - update for fork upgrades
  // Does not include : accessListGas, codeDelegationGas
  public long runOnlyForGasCost(
      Wei senderBalance,
      Long gasLimit,
      List<ToyAccount> additionalAccounts,
      Bytes payload,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    checkArgument(byteCode != null, "byteCode cannot be empty");

    KeyPair keyPair = new SECP256K1().generateKeyPair();
    Address senderAddress = Address.extract(keyPair.getPublicKey());

    final ToyAccount senderAccount =
        ToyAccount.builder().balance(senderBalance).nonce(5).address(senderAddress).build();

    final ToyAccount receiverAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(6)
            .address(Address.fromHexString("0x1111111111111111111111111111111111111111"))
            .code(byteCode)
            .build();

    final ToyTransaction.ToyTransactionBuilder txBuilder =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .value(Wei.of(272)) // 256 + 16, easier for debugging
            .keyPair(keyPair)
            .gasLimit(gasLimit)
            .gasPrice(Wei.of(8));

    if (!payload.isEmpty()) {
      txBuilder.payload(payload);
    }

    final Transaction tx = txBuilder.build();

    List<ToyAccount> accounts = new ArrayList<>();
    accounts.add(senderAccount);
    accounts.add(receiverAccount);
    accounts.addAll(additionalAccounts);

    toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(accounts)
            .transaction(tx)
            .build();

    return toyExecutionEnvironmentV2.runForGasCost();
  }

  public Hub getHub() {
    return toyExecutionEnvironmentV2.getHub();
  }
}
