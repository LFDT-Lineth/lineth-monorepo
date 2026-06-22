/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.bundles;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.core.JsonParseException;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.exc.UnrecognizedPropertyException;
import com.fasterxml.jackson.databind.module.SimpleModule;
import com.fasterxml.jackson.datatype.jdk8.Jdk8Module;
import java.util.List;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.junit.jupiter.api.Test;

class TransactionBundleTest extends AbstractBundleTest {
  private static final SimpleModule BUNDLE_MODULE = new SimpleModule();

  static {
    BUNDLE_MODULE.addSerializer(
        TransactionBundle.PendingBundleTx.class, new TransactionBundle.PendingBundleTxSerializer());
    BUNDLE_MODULE.addSerializer(Hash.class, new TransactionBundle.HashSerializer());
    BUNDLE_MODULE.addDeserializer(
        org.hyperledger.besu.ethereum.core.Transaction.class,
        new TransactionBundle.PendingBundleTxDeserializer());
    BUNDLE_MODULE.addDeserializer(Hash.class, new TransactionBundle.HashDeserializer());
  }

  private static final ObjectMapper OBJECT_MAPPER =
      new ObjectMapper().registerModule(new Jdk8Module()).registerModule(BUNDLE_MODULE);

  @Test
  void serializeToJson() throws JsonProcessingException {

    Hash hash1 = Hash.fromHexStringLenient("0x1234");
    TransactionBundle bundle1 = createBundle(hash1, 1, List.of(TX1, TX2));

    assertThat(OBJECT_MAPPER.writeValueAsString(bundle1))
        .isEqualTo(
            """
            {"blockNumber":1,"bundleIdentifier":"0x0000000000000000000000000000000000000000000000000000000000001234","pendingTransactions":["+E+AghOIglIIgASAggqWoHNvbkX5jC5D+Q0GW88l7bP45W+b8oubebJsfXgE+lRzoAVzHPSnS/zQmUxq3Hg9UHQ3p51KWM6dyYuqKVM7HYz7","+E8BghOIglIIgASAggqVoGgwjcqbkx9qWzUse4MmYxq5fGYo617lp3j9YAj74GDhoFrjtX1uTIbDgflVrS1EPJv2jmbGV2NbxukBL0sNVpBf"],"hasPriority":false}""");
  }

  @Test
  void deserializeFromJson() throws JsonProcessingException {
    TransactionBundle bundle =
        OBJECT_MAPPER.readValue(
            """
            {"blockNumber":1,"bundleIdentifier":"0x0000000000000000000000000000000000000000000000000000000000001234","pendingTransactions":["+E+AghOIglIIgASAggqWoHNvbkX5jC5D+Q0GW88l7bP45W+b8oubebJsfXgE+lRzoAVzHPSnS/zQmUxq3Hg9UHQ3p51KWM6dyYuqKVM7HYz7","+E8BghOIglIIgASAggqVoGgwjcqbkx9qWzUse4MmYxq5fGYo617lp3j9YAj74GDhoFrjtX1uTIbDgflVrS1EPJv2jmbGV2NbxukBL0sNVpBf"],"hasPriority":true}""",
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
            {"wrong":1,"bundleIdentifier":"0x0000000000000000000000000000000000000000000000000000000000001234","pendingTransactions":["+E+AghOIglIIgASAggqWoHNvbkX5jC5D+Q0GW88l7bP45W+b8oubebJsfXgE+lRzoAVzHPSnS/zQmUxq3Hg9UHQ3p51KWM6dyYuqKVM7HYz7","+E8BghOIglIIgASAggqVoGgwjcqbkx9qWzUse4MmYxq5fGYo617lp3j9YAj74GDhoFrjtX1uTIbDgflVrS1EPJv2jmbGV2NbxukBL0sNVpBf"],"hasPriority":true}""",
                    TransactionBundle.class))
        .isInstanceOf(UnrecognizedPropertyException.class)
        .hasMessageContaining("Unrecognized field \"wrong\"");
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

  @Test
  void prioritizedBundleHasPriorityTxs() {
    Hash hash = Hash.fromHexStringLenient("0x1234");
    TransactionBundle bundle = createBundle(hash, 1, List.of(TX1, TX2), true);

    assertThat(bundle.pendingTransactions().stream().allMatch(PendingTransaction::hasPriority))
        .isTrue();
  }
}
