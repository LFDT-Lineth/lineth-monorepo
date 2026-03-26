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

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class TxSkipTests extends TracerTestBase {

  @Test
  void testUselessAccessListForTxSkip(TestInfo testInfo) {
    final ToyAccount receiverAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(116)
            .address(Address.fromHexString("0xdead000000000000000000000000000beef"))
            .build();

    final ToyAccount coinbaseAccount =
        ToyAccount.builder()
            .address(DEFAULT_COINBASE_ADDRESS)
            .balance(Wei.fromEth(2))
            .nonce(5)
            .build();

    final KeyPair senderKeyPair1 = new SECP256K1().generateKeyPair();
    final Address senderAddress1 = Address.extract(senderKeyPair1.getPublicKey());
    final ToyAccount senderAccount1 =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(5).address(senderAddress1).build();

    final KeyPair senderKeyPair2 = new SECP256K1().generateKeyPair();
    final Address senderAddress2 = Address.extract(senderKeyPair2.getPublicKey());
    final ToyAccount senderAccount2 =
        ToyAccount.builder().balance(Wei.fromEth(1231)).nonce(15).address(senderAddress2).build();

    final KeyPair senderKeyPair3 = new SECP256K1().generateKeyPair();
    final Address senderAddress3 = Address.extract(senderKeyPair3.getPublicKey());
    final ToyAccount senderAccount3 =
        ToyAccount.builder().balance(Wei.fromEth(1231)).nonce(15).address(senderAddress3).build();

    final KeyPair senderKeyPair4 = new SECP256K1().generateKeyPair();
    final Address senderAddress4 = Address.extract(senderKeyPair4.getPublicKey());
    final ToyAccount senderAccount4 =
        ToyAccount.builder().balance(Wei.fromEth(11)).nonce(115).address(senderAddress4).build();

    final KeyPair senderKeyPair5 = new SECP256K1().generateKeyPair();
    final Address senderAddress5 = Address.extract(senderKeyPair5.getPublicKey());
    final ToyAccount senderAccount5 =
        ToyAccount.builder().balance(Wei.fromEth(12)).nonce(0).address(senderAddress5).build();

    final KeyPair senderKeyPair6 = new SECP256K1().generateKeyPair();
    final Address senderAddress6 = Address.extract(senderKeyPair6.getPublicKey());
    final ToyAccount senderAccount6 =
        ToyAccount.builder().balance(Wei.fromEth(12)).nonce(6).address(senderAddress6).build();

    final KeyPair senderKeyPair7 = new SECP256K1().generateKeyPair();
    final Address senderAddress7 = Address.extract(senderKeyPair7.getPublicKey());
    final ToyAccount senderAccount7 =
        ToyAccount.builder().balance(Wei.fromEth(231)).nonce(21).address(senderAddress7).build();

    final Transaction pureTransfer =
        ToyTransaction.builder()
            .sender(senderAccount1)
            .to(receiverAccount)
            .keyPair(senderKeyPair1)
            .value(Wei.of(123))
            .build();

    final Transaction pureTransferWoValue =
        ToyTransaction.builder()
            .sender(senderAccount2)
            .to(receiverAccount)
            .keyPair(senderKeyPair2)
            .value(Wei.of(0))
            .build();

    final List<String> listOfKeys =
        List.of("0x0123", "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef");
    final List<AccessListEntry> accessList =
        List.of(
            AccessListEntry.createAccessListEntry(
                Address.fromHexString("0x1234567890"), listOfKeys));

    final Transaction pureTransferWithUselessAccessList =
        ToyTransaction.builder()
            .sender(senderAccount3)
            .to(receiverAccount)
            .keyPair(senderKeyPair3)
            .gasLimit(100000L)
            .transactionType(TransactionType.ACCESS_LIST)
            .accessList(accessList)
            .value(Wei.of(546))
            .build();

    final Transaction pureTransferWithUselessCalldata =
        ToyTransaction.builder()
            .sender(senderAccount4)
            .to(receiverAccount)
            .keyPair(senderKeyPair4)
            .gasLimit(1000001L)
            .value(Wei.of(546))
            .payload(Bytes.minimalBytes(0xdeadbeefL))
            .build();

    final Transaction pureTransferWithUselessCalldataAndAccessList =
        ToyTransaction.builder()
            .sender(senderAccount5)
            .to(receiverAccount)
            .gasLimit(1000020L)
            .transactionType(TransactionType.EIP1559)
            .keyPair(senderKeyPair5)
            .value(Wei.of(546))
            .accessList(accessList)
            .payload(Bytes.minimalBytes(0xdeadbeefL))
            .build();

    final Transaction deploymentWithEmptyInit =
        ToyTransaction.builder()
            .sender(senderAccount6)
            .keyPair(senderKeyPair6)
            .gasLimit(1234567L)
            .value(Wei.of(546))
            .build();

    final Transaction deploymentWithEmptyInitAndUselessAccessList =
        ToyTransaction.builder()
            .sender(senderAccount7)
            .gasLimit(1000021L)
            .keyPair(senderKeyPair7)
            .transactionType(TransactionType.ACCESS_LIST)
            .value(Wei.of(546))
            .accessList(accessList)
            .build();

    final List<Transaction> txs =
        List.of(
            pureTransfer,
            pureTransferWoValue,
            pureTransferWithUselessAccessList,
            pureTransferWithUselessCalldata,
            pureTransferWithUselessCalldataAndAccessList,
            deploymentWithEmptyInit,
            deploymentWithEmptyInitAndUselessAccessList);

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(
            List.of(
                coinbaseAccount,
                senderAccount1,
                senderAccount2,
                senderAccount3,
                senderAccount4,
                senderAccount5,
                senderAccount6,
                senderAccount7,
                receiverAccount))
        .transactions(txs)
        .zkTracerValidator(
            zkTracer -> {

              // Ensure we have the right nb of rows in the HUB
              // assertThat(zkTracer.getHub().lineCount()).isEqualTo(txs.size() * nbOfRowsTxSkip);
            })
        .build()
        .run();
  }

  @Test
  void receiverIsCoinbase(TestInfo testInfo) {

    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(5).address(senderAddress).build();

    final ToyAccount coinbaseAccount =
        ToyAccount.builder()
            .address(DEFAULT_COINBASE_ADDRESS)
            .balance(Wei.fromEth(2))
            .nonce(5)
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(coinbaseAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(100))
            .gasPrice(Wei.of(8))
            .gasLimit(100000L)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(coinbaseAccount, senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void receiverIsSender(TestInfo testInfo) {

    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(5).address(senderAddress).build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void senderIsCoinbase(TestInfo testInfo) {

    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(5).address(senderAddress).build();

    final ToyAccount receiverAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(116)
            .address(Address.fromHexString("0xdead000000000000000000000000000beef"))
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(senderAddress)
        .build()
        .run();
  }

  @Test
  void senderIsCoinbaseIsReceiver(TestInfo testInfo) {

    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(5).address(senderAddress).build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(senderAddress)
        .build()
        .run();
  }

  @Test
  void skipMessageCallCoinbaseIsPrecompile(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final Address receiverAddress =
        Address.fromHexString("0xffffffffffffffffffffffffffffffffffffff");

    final ToyAccount receiverAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(0xffffee))
            .nonce(18)
            .address(receiverAddress)
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(Address.BLAKE2B_F_COMPRESSION)
        .build()
        .run();
  }

  @Test
  void skippedDepSenderIsCoinbase(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(senderAddress)
        .build()
        .run();
  }

  @Test
  void skippedDepDeploymentAddressIsCoinbase(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final int nonce = 632;
    final ToyAccount senderAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(0xffff))
            .nonce(nonce)
            .address(senderAddress)
            .build();

    final Address depAddress = Address.extract(getCreateRawAddress(senderAddress, nonce));

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .nonce((long) nonce)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(depAddress)
        .build()
        .run();
  }

  @Test
  void skippedDepCoinbaseIsPrecompile(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(Address.RIPEMD160)
        .build()
        .run();
  }

  @Test
  void txSkipValueIsZero(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(127).address(senderAddress).build();

    final ToyAccount receiverAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(Address.fromHexString("0xdead000000000000000000000000000beef"))
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .value(Wei.ZERO)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }
}
