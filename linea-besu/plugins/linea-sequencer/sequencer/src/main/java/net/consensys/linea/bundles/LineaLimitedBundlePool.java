/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.bundles;

import static java.util.Collections.emptyList;

import com.fasterxml.jackson.databind.MappingIterator;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SequenceWriter;
import com.fasterxml.jackson.databind.module.SimpleModule;
import com.fasterxml.jackson.datatype.jdk8.Jdk8Module;
import com.github.benmanes.caffeine.cache.Cache;
import com.github.benmanes.caffeine.cache.Caffeine;
import com.github.benmanes.caffeine.cache.RemovalCause;
import com.github.benmanes.caffeine.cache.Scheduler;
import com.google.auto.service.AutoService;
import com.google.common.annotations.VisibleForTesting;
import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import lombok.extern.slf4j.Slf4j;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.AddedBlockContext;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.BesuService;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.util.Subscribers;

/**
 * A pool for managing TransactionBundles with limited size and FIFO eviction. Provides access via
 * hash identifiers or block numbers.
 */
@AutoService(BesuService.class)
@Slf4j
public class LineaLimitedBundlePool implements BundlePoolService, BesuEvents.BlockAddedListener {
  public static final String BUNDLE_SAVE_FILENAME = "bundles.ndjson";
  private final BlockchainService blockchainService;
  private final Cache<Hash, TransactionBundle> cache;
  private final Map<Long, List<TransactionBundle>> blockIndex;
  private final Path saveFilePath;
  private final AtomicBoolean isFrozen = new AtomicBoolean(false);
  private final Subscribers<TransactionBundleAddedListener> transactionBundleAddedListeners =
      Subscribers.create();
  private final Subscribers<TransactionBundleRemovedListener> transactionBundleRemovedListeners =
      Subscribers.create();

  /**
   * Initializes the LineaLimitedBundlePool with a maximum size and expiration time, and registers
   * as a blockAddedEvent listener.
   *
   * @param maxSizeInBytes The maximum size in bytes of the pool objects.
   */
  @VisibleForTesting
  public LineaLimitedBundlePool(
      final Path dataDir,
      final long maxSizeInBytes,
      final BesuEvents eventService,
      final BlockchainService blockchainService) {
    this.saveFilePath = dataDir.resolve(BUNDLE_SAVE_FILENAME);
    this.blockchainService = blockchainService;
    this.cache =
        Caffeine.newBuilder()
            .maximumWeight(maxSizeInBytes) // Maximum size in bytes
            .scheduler(
                Scheduler
                    .systemScheduler()) // To ensure maintenance operation are not delayed too much
            .weigher(
                (Hash key, TransactionBundle value) -> {
                  // Calculate the size of a TransactionBundle in bytes
                  return calculateWeight(value);
                })
            .removalListener(
                (Hash key, TransactionBundle bundle, RemovalCause cause) -> {
                  if (bundle != null) {
                    if (cause.wasEvicted()) {
                      log.atTrace()
                          .setMessage("Dropping transaction bundle {}:{} due to {}")
                          .addArgument(bundle::blockNumber)
                          .addArgument(() -> bundle.bundleIdentifier().getBytes().toHexString())
                          .addArgument(cause::name)
                          .log();
                      removeFromBlockIndex(bundle);
                    }

                    transactionBundleRemovedListeners.forEach(
                        listener -> listener.onTransactionBundleRemoved(bundle));
                  }
                })
            .build();
    this.blockIndex = new ConcurrentHashMap<>();

    // register ourselves as a block added listener:
    eventService.addBlockAddedListener(this);
  }

  /**
   * Retrieves a list of TransactionBundles associated with a block number.
   *
   * @param blockNumber The block number to look up.
   * @return A list of TransactionBundles for the given block number, or an empty list if none are
   *     found. The returned list if safe for modification since it is not backed by the original
   *     one
   */
  public List<TransactionBundle> getBundlesByBlockNumber(long blockNumber) {
    return List.copyOf(blockIndex.getOrDefault(blockNumber, emptyList()));
  }

  /**
   * Retrieves a TransactionBundle by its unique hash identifier.
   *
   * @param hash The hash identifier of the TransactionBundle.
   * @return The TransactionBundle associated with the hash, or null if not found.
   */
  public TransactionBundle get(Hash hash) {
    return cache.getIfPresent(hash);
  }

  /**
   * Retrieves a TransactionBundle by its replacement UUID
   *
   * @param replacementUUID identifier of the TransactionBundle.
   * @return The TransactionBundle associated with the uuid, or null if not found.
   */
  public TransactionBundle get(UUID replacementUUID) {
    return cache.getIfPresent(UUIDToHash(replacementUUID));
  }

  /**
   * Puts or replaces an existing TransactionBundle in the cache and updates the block index.
   *
   * @param hash The hash identifier of the TransactionBundle.
   * @param bundle The new TransactionBundle to replace the existing one.
   */
  public void putOrReplace(Hash hash, TransactionBundle bundle) {
    failIfFrozen(
        () -> {
          TransactionBundle existing = cache.getIfPresent(hash);
          if (existing != null) {
            removeFromBlockIndex(existing);
          }
          cache.put(hash, bundle);
          addToBlockIndex(bundle);
          return null;
        });
  }

  /**
   * Puts or replaces an existing TransactionBundle by UUIDin the cache and updates the block index.
   *
   * @param replacementUUID identifier of the TransactionBundle.
   * @param bundle The new TransactionBundle to replace the existing one.
   */
  public void putOrReplace(UUID replacementUUID, TransactionBundle bundle) {
    putOrReplace(UUIDToHash(replacementUUID), bundle);
  }

  /**
   * removes an existing TransactionBundle in the cache and updates the block index.
   *
   * @param replacementUUID identifier of the TransactionBundle.
   * @return boolean indicating if bundle was found and removed
   */
  public boolean remove(UUID replacementUUID) {
    return remove(UUIDToHash(replacementUUID));
  }

  /**
   * removes an existing TransactionBundle in the cache and updates the block index.
   *
   * @param hash The hash identifier of the TransactionBundle.
   * @return boolean indicating if bundle was found and removed
   */
  public boolean remove(Hash hash) {
    return failIfFrozen(
        () -> {
          var existingBundle = cache.getIfPresent(hash);
          if (existingBundle != null) {
            cache.invalidate(hash);
            removeFromBlockIndex(existingBundle);
            return true;
          }
          return false;
        });
  }

  @Override
  public long size() {
    return cache.estimatedSize();
  }

  @Override
  public boolean isFrozen() {
    return isFrozen.get();
  }

  @Override
  public void saveToDisk() {
    synchronized (isFrozen) {
      isFrozen.set(true);
      log.info("Saving bundles to {}", saveFilePath);
      final var objectMapper = new ObjectMapper().registerModule(new Jdk8Module());
      configureObjectMapperV2(objectMapper);
      try (final BufferedWriter bw = Files.newBufferedWriter(saveFilePath, StandardCharsets.UTF_8);
          final SequenceWriter sequenceWriter =
              objectMapper
                  .writerFor(TransactionBundle.class)
                  .withRootValueSeparator(System.lineSeparator())
                  .writeValues(bw)) {

        // write the header
        bw.write(objectMapper.writeValueAsString(Map.of("version", 2)));
        bw.newLine();

        // write the bundles sorted by block number
        final var savedCount =
            blockIndex.values().stream()
                .flatMap(List::stream)
                .sorted(Comparator.comparing(TransactionBundle::blockNumber))
                .peek(
                    bundle -> {
                      try {
                        sequenceWriter.write(bundle);
                      } catch (IOException e) {
                        throw new RuntimeException(e);
                      }
                    })
                .count();
        log.info("Saved {} bundles to {}", savedCount, saveFilePath);
      } catch (final Throwable ioe) {
        log.error("Error while saving bundles to {}", saveFilePath, ioe);
      }
    }
  }

  @Override
  public void loadFromDisk() {
    failIfFrozen(
        () -> {
          if (saveFilePath.toFile().exists()) {
            log.info("Loading bundles from {}", saveFilePath);
            final var chainHeadBlockNumber = blockchainService.getChainHeadHeader().getNumber();
            final var loadedCount = new AtomicLong(0L);
            final var skippedCount = new AtomicLong(0L);

            try (final BufferedReader br =
                Files.newBufferedReader(saveFilePath, StandardCharsets.UTF_8)) {
              final var objectMapper = new ObjectMapper().registerModule(new Jdk8Module());
              // read the header and check the version
              final var headerNode = objectMapper.readTree(br.readLine());
              if (!headerNode.has("version") || !headerNode.get("version").isInt()) {
                throw new IllegalArgumentException(
                    "Unsupported bundle serialization header " + headerNode);
              }
              log.info("Loading bundles from {}, header {}", saveFilePath, headerNode);

              // register (de)serializer for saving/restoring from disk according to the version
              final int version = headerNode.get("version").asInt();
              switch (version) {
                case 1 -> configureObjectMapperV1(objectMapper);
                case 2 -> configureObjectMapperV2(objectMapper);
                default ->
                    throw new IllegalArgumentException(
                        "Unsupported bundle serialization version " + version);
              }

              try (final MappingIterator<TransactionBundle> iterator =
                  objectMapper.readerFor(TransactionBundle.class).readValues(br)) {
                iterator.forEachRemaining(
                    bundle -> {
                      if (bundle.blockNumber() > chainHeadBlockNumber) {
                        this.putOrReplace(bundle.bundleIdentifier(), bundle);
                        loadedCount.incrementAndGet();
                      } else {
                        log.debug(
                            "Skipping bundle {} at location {}, since its block number {} is not greater than chain head block number {}",
                            bundle.bundleIdentifier(),
                            iterator.getCurrentLocation(),
                            bundle.blockNumber(),
                            chainHeadBlockNumber);
                        skippedCount.incrementAndGet();
                      }
                    });
                log.info("Loaded {} bundles from {}", loadedCount.get(), saveFilePath);
              }
            } catch (final Throwable t) {
              log.error(
                  "Error while reading bundles from {}, partially loaded {} bundles",
                  saveFilePath,
                  loadedCount.get(),
                  t);
            }
            saveFilePath.toFile().delete();
          }
          return null;
        });
  }

  private void configureObjectMapperV1(final ObjectMapper objectMapper) {
    final var module = new SimpleModule();
    module.addDeserializer(
        TransactionBundle.class, new TransactionBundle.TransactionBundleDeserializerV1());
    objectMapper.registerModule(module);
  }

  private void configureObjectMapperV2(final ObjectMapper objectMapper) {
    final var module = new SimpleModule();
    module.addSerializer(
        TransactionBundle.PendingBundleTx.class, new TransactionBundle.PendingBundleTxSerializer());
    module.addSerializer(Hash.class, new TransactionBundle.HashSerializer());
    module.addDeserializer(Transaction.class, new TransactionBundle.PendingBundleTxDeserializer());
    module.addDeserializer(Hash.class, new TransactionBundle.HashDeserializer());
    objectMapper.registerModule(module);
  }

  @Override
  public long subscribeTransactionBundleAdded(final TransactionBundleAddedListener listener) {
    return transactionBundleAddedListeners.subscribe(listener);
  }

  @Override
  public long subscribeTransactionBundleRemoved(final TransactionBundleRemovedListener listener) {
    return transactionBundleRemovedListeners.subscribe(listener);
  }

  @Override
  public void unsubscribeTransactionBundleAdded(final long listenerId) {
    transactionBundleAddedListeners.unsubscribe(listenerId);
  }

  @Override
  public void unsubscribeTransactionBundleRemoved(final long listenerId) {
    transactionBundleRemovedListeners.unsubscribe(listenerId);
  }

  /**
   * Adds a TransactionBundle to the block index.
   *
   * @param bundle The TransactionBundle to add.
   */
  private void addToBlockIndex(TransactionBundle bundle) {
    long blockNumber = bundle.blockNumber();
    blockIndex.computeIfAbsent(blockNumber, k -> new ArrayList<>()).add(bundle);
    transactionBundleAddedListeners.forEach(listener -> listener.onTransactionBundleAdded(bundle));
  }

  /**
   * Removes a TransactionBundle from the block index.
   *
   * @param bundle The TransactionBundle to remove.
   */
  private void removeFromBlockIndex(TransactionBundle bundle) {
    long blockNumber = bundle.blockNumber();
    List<TransactionBundle> bundles = blockIndex.get(blockNumber);
    if (bundles != null) {
      bundles.remove(bundle);
      if (bundles.isEmpty()) {
        blockIndex.remove(blockNumber);
      }
    }
  }

  private int calculateWeight(TransactionBundle bundle) {
    return bundle.pendingTransactions().stream().mapToInt(PendingTransaction::memorySize).sum();
  }

  /**
   * convert a UUID into a hash used by the bundle pool.
   *
   * @param uuid the uuid to hash
   * @return Hash identifier for the uuid
   */
  public static Hash UUIDToHash(UUID uuid) {
    return Hash.hash(
        Bytes.concatenate(
            Bytes.ofUnsignedLong(uuid.getMostSignificantBits()),
            Bytes.ofUnsignedLong(uuid.getLeastSignificantBits())));
  }

  /**
   * Cull the bundle pool on the basis of blocks added.
   *
   * @param addedBlockContext
   */
  @Override
  public void onBlockAdded(final AddedBlockContext addedBlockContext) {
    synchronized (isFrozen) {
      if (!isFrozen.get()) { // do nothing if frozen
        final var lastSeen = addedBlockContext.getBlockHeader().getNumber();
        final var latest = Math.max(lastSeen, blockchainService.getChainHeadHeader().getNumber());
        // keep it simple regarding reorgs and, cull the pool for any block numbers lower than
        // latest
        blockIndex.keySet().stream()
            .filter(k -> k < latest)
            // collecting to a set in order to not mutate the collection we are streaming:
            .collect(Collectors.toSet())
            .forEach(
                k -> {
                  blockIndex.get(k).forEach(bundle -> cache.invalidate(bundle.bundleIdentifier()));
                  // dropping from the cache should inherently remove from blockIndex, but this
                  // is cheap insurance against blockIndex map leaking due to cache evictions
                  blockIndex.remove(k);
                });
      }
    }
  }

  private <R> R failIfFrozen(Supplier<R> modificationAction) {
    synchronized (isFrozen) {
      if (isFrozen.get()) {
        throw new IllegalStateException("Bundle pool is not accepting modifications");
      }
      return modificationAction.get();
    }
  }
}
