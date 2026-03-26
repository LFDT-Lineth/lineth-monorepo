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
package net.consensys.linea.zktracer;

import com.google.common.base.Preconditions;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyTransaction;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;

public class MultiBlockUtils extends TracerTestBase {

  public static void multiBlocksTest(List<Bytes> programs, TestInfo testInfo) {
    multiBlocksTest(programs, List.of(), testInfo);
  }

  public static void multiBlocksTest(
      List<Bytes> programs, List<Long> gasLimits, TestInfo testInfo) {
    Preconditions.checkArgument(gasLimits.isEmpty() || programs.size() == gasLimits.size());

    List<KeyPair> keyPairs = new ArrayList<>();
    List<Address> senderAddresses = new ArrayList<>();
    List<ToyAccount> senderAccounts = new ArrayList<>();
    List<ToyAccount> receiverAccounts = new ArrayList<>();
    List<Transaction> transactions = new ArrayList<>();

    for (int i = 0; i < programs.size(); i++) {
      Bytes program = programs.get(i);
      keyPairs.add(new SECP256K1().generateKeyPair());
      senderAddresses.add(Address.extract(keyPairs.getLast().getPublicKey()));
      senderAccounts.add(
          ToyAccount.builder()
              .balance(Wei.fromEth(1 + i))
              .nonce(3 + i)
              .address(senderAddresses.getLast())
              .build());
      receiverAccounts.add(
          ToyAccount.builder()
              .balance(Wei.ONE)
              .nonce(5 + i)
              .address(Address.fromHexString("0x" + (20 + i)))
              .code(program)
              .build());
      transactions.add(
          ToyTransaction.builder()
              .sender(senderAccounts.getLast())
              .to(receiverAccounts.getLast())
              .keyPair(keyPairs.getLast())
              .build());
    }

    MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
        MultiBlockExecutionEnvironment.builder(chainConfig, testInfo)
            .accounts(
                Stream.concat(senderAccounts.stream(), receiverAccounts.stream())
                    .collect(Collectors.toList()));

    for (int i = 0; i < transactions.size(); i++) {
      if (gasLimits.isEmpty()) {
        builder.addBlock(List.of(transactions.get(i)));
      } else {
        builder.addBlock(List.of(transactions.get(i)), gasLimits.get(i));
      }
    }

    builder.build().run();
  }
}
