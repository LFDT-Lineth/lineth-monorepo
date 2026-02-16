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

package net.consensys.linea.zktracer.delegation;

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;

import java.math.BigInteger;
import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class SimpleDelegationTest extends TracerTestBase {

  @Disabled
  @Test
  void simpleDelegationTest(TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(1789)).nonce(0).address(senderAddress).build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(0)
            .address(Address.fromHexString("0x1122334455667788990011223344556677889900"))
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            // add some stupid delegation
            .addCodeDelegation(BigInteger.ONE, Address.ALTBN128_ADD, 18, keyPair)
            .addCodeDelegation(BigInteger.ZERO, Address.ZERO, 0, keyPair)
            .addCodeDelegation(chainConfig.id, senderAddress, 1, keyPair)
            .addCodeDelegation(
                Bytes.repeat((byte) 0xFF, WORD_SIZE).toUnsignedBigInteger(),
                recipientAccount.getAddress(),
                0L,
                keyPair)
            .addCodeDelegation(chainConfig.id, senderAddress, 1, keyPair)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }
}
