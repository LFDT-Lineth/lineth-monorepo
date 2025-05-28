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

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.List;

import com.fasterxml.jackson.core.JsonParseException;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.exc.ValueInstantiationException;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.eth.transactions.PendingTransaction;
import org.junit.jupiter.api.Test;

class TransactionBundleTest extends AbstractBundleTest {

  @Test
  void serializeToJson() throws JsonProcessingException {

    Hash hash1 = Hash.fromHexStringLenient("0x1234");
    TransactionBundle bundle1 = createBundle(hash1, 1, List.of(TX1, TX2));

    assertThat(OBJECT_MAPPER.writeValueAsString(bundle1))
        .isEqualTo(
            """
            {"0x0000000000000000000000000000000000000000000000000000000000001234":{"blockNumber":1,"txs":["+E+AghOIglIIgASAggqWoHNvbkX5jC5D+Q0GW88l7bP45W+b8oubebJsfXgE+lRzoAVzHPSnS/zQmUxq3Hg9UHQ3p51KWM6dyYuqKVM7HYz7","+E8BghOIglIIgASAggqVoGgwjcqbkx9qWzUse4MmYxq5fGYo617lp3j9YAj74GDhoFrjtX1uTIbDgflVrS1EPJv2jmbGV2NbxukBL0sNVpBf"]}}""");
  }

  @Test
  void deserializeFromJson() throws JsonProcessingException {
    TransactionBundle bundle =
        OBJECT_MAPPER.readValue(
            """
            {"0x0000000000000000000000000000000000000000000000000000000000001234":{"blockNumber":1,"txs":["+E+AghOIglIIgASAggqWoHNvbkX5jC5D+Q0GW88l7bP45W+b8oubebJsfXgE+lRzoAVzHPSnS/zQmUxq3Hg9UHQ3p51KWM6dyYuqKVM7HYz7","+E8BghOIglIIgASAggqVoGgwjcqbkx9qWzUse4MmYxq5fGYo617lp3j9YAj74GDhoFrjtX1uTIbDgflVrS1EPJv2jmbGV2NbxukBL0sNVpBf"]}}""",
            TransactionBundle.class);

    assertThat(bundle.blockNumber()).isEqualTo(1);
    assertThat(bundle.bundleIdentifier()).isEqualTo(Hash.fromHexStringLenient("0x1234"));
    assertThat(bundle.pendingTransactions())
        .map(PendingTransaction::getTransaction)
        .map(Transaction::getHash)
        .containsExactly(TX1.getHash(), TX2.getHash());
  }

  @Test
  void deserializedMalformed() {
    assertThatThrownBy(
            () ->
                OBJECT_MAPPER.readValue(
                    """
            {"0x0000000000000000000000000000000000000000000000000000000000001234":{"wrong":1,"txs":["+E+AghOIglIIgASAggqWoHNvbkX5jC5D+Q0GW88l7bP45W+b8oubebJsfXgE+lRzoAVzHPSnS/zQmUxq3Hg9UHQ3p51KWM6dyYuqKVM7HYz7","+E8BghOIglIIgASAggqVoGgwjcqbkx9qWzUse4MmYxq5fGYo617lp3j9YAj74GDhoFrjtX1uTIbDgflVrS1EPJv2jmbGV2NbxukBL0sNVpBf"]}}""",
                    TransactionBundle.class))
        .isInstanceOf(ValueInstantiationException.class)
        .hasMessageContaining("because \"blockNumber\" is null");
  }

  @Test
  void restoreFromSerializedParseError() {
    assertThatThrownBy(
            () ->
                OBJECT_MAPPER.readValue(
                    """
            {{wrong=json}}""",
                    TransactionBundle.class))
        .isInstanceOf(JsonParseException.class)
        .hasMessageStartingWith(
            "Unexpected character ('{' (code 123)): was expecting double-quote to start field name");
  }
}
