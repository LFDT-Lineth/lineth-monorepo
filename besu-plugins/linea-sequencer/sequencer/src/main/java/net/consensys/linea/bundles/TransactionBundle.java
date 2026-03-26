/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.bundles;

import static com.fasterxml.jackson.annotation.JsonInclude.Include.NON_ABSENT;

import com.fasterxml.jackson.annotation.JsonAutoDetect;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.DeserializationContext;
import com.fasterxml.jackson.databind.SerializerProvider;
import com.fasterxml.jackson.databind.deser.std.StdDeserializer;
import com.fasterxml.jackson.databind.ser.std.StdSerializer;
import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicLong;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.ToString;
import lombok.experimental.Accessors;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.parameters.UnsignedLongParameter;
import org.hyperledger.besu.ethereum.core.Transaction;

/** TransactionBundle class representing a collection of pending transactions with metadata. */
@Accessors(fluent = true)
@Getter
@EqualsAndHashCode
@ToString
@JsonInclude(NON_ABSENT)
@JsonPropertyOrder({"blockNumber", "minTimestamp", "maxTimestamp"})
@JsonAutoDetect(fieldVisibility = JsonAutoDetect.Visibility.ANY)
public class TransactionBundle {
  private static final AtomicLong BUNDLE_COUNT = new AtomicLong(0L);
  private final transient long sequence = BUNDLE_COUNT.incrementAndGet();
  private final Hash bundleIdentifier;
  private final List<? extends PendingBundleTx> pendingTransactions;
  private final Long blockNumber;
  private final Optional<Long> minTimestamp;
  private final Optional<Long> maxTimestamp;
  private final Optional<List<Hash>> revertingTxHashes;
  private final Optional<UUID> replacementUUID;
  private final boolean hasPriority;

  @JsonCreator
  public TransactionBundle(
      @JsonProperty("bundleIdentifier") final Hash bundleIdentifier,
      @JsonProperty("pendingTransactions") final List<Transaction> transactions,
      @JsonProperty("blockNumber") final Long blockNumber,
      @JsonProperty("minTimestamp") final Optional<Long> minTimestamp,
      @JsonProperty("maxTimestamp") final Optional<Long> maxTimestamp,
      @JsonProperty("revertingTxHashes") final Optional<List<Hash>> revertingTxHashes,
      @JsonProperty("replacementUUID") final Optional<UUID> replacementUUID,
      @JsonProperty("hasPriority") final boolean hasPriority) {
    this.bundleIdentifier = bundleIdentifier;
    this.pendingTransactions =
        transactions.stream()
            .map(hasPriority ? PriorityPendingBundleTx::new : NormalPendingBundleTx::new)
            .toList();
    this.blockNumber = blockNumber;
    this.minTimestamp = minTimestamp;
    this.maxTimestamp = maxTimestamp;
    this.revertingTxHashes = revertingTxHashes;
    this.replacementUUID = replacementUUID;
    this.hasPriority = hasPriority;
  }

  public BundleParameter toBundleParameter() {
    return new BundleParameter(
        pendingTransactions.stream()
            .map(ptx -> ptx.getTransaction().encoded().toHexString())
            .toList(),
        new UnsignedLongParameter(blockNumber),
        minTimestamp,
        maxTimestamp,
        revertingTxHashes,
        replacementUUID.map(UUID::toString),
        Optional.empty());
  }

  /** A pending transaction contained in a bundle. */
  public interface PendingBundleTx extends PendingTransaction {
    TransactionBundle getBundle();

    default String toBase64String() {
      return getTransaction().encoded().toBase64String();
    }

    default boolean isBundleStart() {
      return getBundle().pendingTransactions().getFirst().equals(this);
    }
  }

  /** A pending transaction contained in a bundle without priority. */
  private class NormalPendingBundleTx
      extends org.hyperledger.besu.ethereum.eth.transactions.PendingTransaction.Local
      implements PendingBundleTx {

    public NormalPendingBundleTx(final Transaction transaction) {
      super(transaction);
    }

    @Override
    public TransactionBundle getBundle() {
      return TransactionBundle.this;
    }

    @Override
    public String toTraceLog() {
      return "Bundle tx: " + super.toTraceLog();
    }
  }

  /** A pending transaction contained in a bundle with priority. */
  private class PriorityPendingBundleTx
      extends org.hyperledger.besu.ethereum.eth.transactions.PendingTransaction.Local.Priority
      implements PendingBundleTx {

    public PriorityPendingBundleTx(final Transaction transaction) {
      super(transaction);
    }

    @Override
    public TransactionBundle getBundle() {
      return TransactionBundle.this;
    }

    @Override
    public String toTraceLog() {
      return "Priority bundle tx: " + super.toTraceLog();
    }
  }

  public static class HashSerializer extends StdSerializer<Hash> {
    public HashSerializer() {
      this(null);
    }

    public HashSerializer(final Class<Hash> t) {
      super(t);
    }

    @Override
    public void serialize(
        final Hash value, final JsonGenerator gen, final SerializerProvider provider)
        throws IOException {
      gen.writeString(value.getBytes().toHexString());
    }
  }

  public static class HashDeserializer extends StdDeserializer<Hash> {
    public HashDeserializer() {
      this(null);
    }

    @Override
    public Hash deserialize(final JsonParser p, final DeserializationContext ctxt)
        throws IOException {
      return Hash.fromHexString(p.getValueAsString());
    }

    public HashDeserializer(final Class<Hash> t) {
      super(t);
    }
  }

  public static class PendingBundleTxSerializer extends StdSerializer<PendingBundleTx> {
    public PendingBundleTxSerializer() {
      this(null);
    }

    public PendingBundleTxSerializer(final Class<PendingBundleTx> t) {
      super(t);
    }

    @Override
    public void serialize(
        final PendingBundleTx pendingBundleTx,
        final JsonGenerator gen,
        final SerializerProvider serializerProvider)
        throws IOException {
      gen.writeString(pendingBundleTx.toBase64String());
    }
  }

  public static class PendingBundleTxDeserializer extends StdDeserializer<Transaction> {
    public PendingBundleTxDeserializer() {
      this(null);
    }

    public PendingBundleTxDeserializer(final Class<?> vc) {
      super(vc);
    }

    @Override
    public Transaction deserialize(
        final JsonParser jsonParser, final DeserializationContext deserializationContext)
        throws IOException {
      return Transaction.readFrom(Bytes.fromBase64String(jsonParser.getValueAsString()));
    }
  }

  public static class TransactionBundleDeserializerV1 extends StdDeserializer<TransactionBundle> {
    private static final TypeReference<Map.Entry<Hash, BundleParameter>> TYPE_REFERENCE =
        new TypeReference<>() {};

    public TransactionBundleDeserializerV1() {
      this(null);
    }

    public TransactionBundleDeserializerV1(final Class<?> vc) {
      super(vc);
    }

    @Override
    public TransactionBundle deserialize(final JsonParser p, final DeserializationContext ctxt)
        throws IOException {
      @SuppressWarnings("unchecked")
      final var entry = (Map.Entry<Hash, BundleParameter>) p.readValueAs(TYPE_REFERENCE);
      final var hash = entry.getKey();
      final var parameters = entry.getValue();
      return new TransactionBundle(
          hash,
          parameters.txs().stream()
              .map(Bytes::fromBase64String)
              .map(Transaction::readFrom)
              .toList(),
          parameters.blockNumber(),
          parameters.minTimestamp(),
          parameters.maxTimestamp(),
          parameters.revertingTxHashes(),
          parameters.replacementUUID().map(UUID::fromString),
          false);
    }
  }
}
