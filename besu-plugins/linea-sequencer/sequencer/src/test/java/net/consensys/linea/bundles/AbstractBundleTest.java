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

package net.consensys.linea.bundles;

import java.math.BigInteger;
import java.util.List;
import java.util.Optional;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jdk8.Jdk8Module;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECPPrivateKey;
import org.hyperledger.besu.crypto.SECPPublicKey;
import org.hyperledger.besu.crypto.SignatureAlgorithm;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.core.TransactionTestFixture;

abstract class AbstractBundleTest {
  protected static final ObjectMapper OBJECT_MAPPER =
      new ObjectMapper().registerModule(new Jdk8Module());
  protected static final KeyPair KEY_PAIR =
      new KeyPair(
          SECPPrivateKey.create(BigInteger.valueOf(Long.MAX_VALUE), SignatureAlgorithm.ALGORITHM),
          SECPPublicKey.create(BigInteger.valueOf(Long.MIN_VALUE), SignatureAlgorithm.ALGORITHM));

  protected static final Transaction TX1 =
      new TransactionTestFixture().nonce(0).gasLimit(21000).createTransaction(KEY_PAIR);
  protected static final Transaction TX2 =
      new TransactionTestFixture().nonce(1).gasLimit(21000).createTransaction(KEY_PAIR);
  protected static final Transaction TX3 =
      new TransactionTestFixture().nonce(2).gasLimit(21000).createTransaction(KEY_PAIR);

  protected TransactionBundle createBundle(
      Hash hash, long blockNumber, List<Transaction> maybeTxs) {
    return new TransactionBundle(
        hash,
        maybeTxs,
        blockNumber,
        Optional.empty(),
        Optional.empty(),
        Optional.empty(),
        Optional.empty());
  }
}
