/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.config;

import com.google.common.base.MoreObjects;
import jakarta.validation.constraints.Positive;
import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collections;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Stream;
import net.consensys.linea.plugins.LineaCliOptions;
import net.consensys.linea.sequencer.txselection.selectors.TransactionEventFilter;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.log.LogTopic;
import picocli.CommandLine;

/** The Linea Transaction Selector CLI options. */
public class LineaTransactionSelectorCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "transaction-selector-config";

  public static final String MAX_BLOCK_CALLDATA_SIZE = "--plugin-linea-max-block-calldata-size";
  public static final int DEFAULT_MAX_BLOCK_CALLDATA_SIZE = 70_000;
  public static final String OVER_LINE_COUNT_LIMIT_CACHE_SIZE =
      "--plugin-linea-over-line-count-limit-cache-size";
  public static final int DEFAULT_OVER_LINE_COUNT_LIMIT_CACHE_SIZE = 10_000;

  public static final String MAX_GAS_PER_BLOCK = "--plugin-linea-max-block-gas";
  public static final long DEFAULT_MAX_GAS_PER_BLOCK = 30_000_000L;
  public static final String MAX_BUNDLE_POOL_SIZE_BYTES =
      "--plugin-linea-max-bundle-pool-size-bytes";
  public static final long DEFAULT_MAX_BUNDLE_POOL_SIZE_BYTES = 1024 * 1024 * 16L;
  public static final String MAX_BUNDLE_GAS_PER_BLOCK = "--plugin-linea-max-bundle-block-gas";
  public static final long DEFAULT_MAX_BUNDLE_GAS_PER_BLOCK = 15_000_000L;

  public static final String EVENTS_DENY_LIST_PATH = "--plugin-linea-events-deny-list-path";
  public static final String EVENTS_BUNDLE_DENY_LIST_PATH =
      "--plugin-linea-events-bundle-deny-list-path";

  @Positive
  @CommandLine.Option(
      names = {MAX_BLOCK_CALLDATA_SIZE},
      hidden = true,
      paramLabel = "<INTEGER>",
      description = "Maximum size for the calldata of a block (default: ${DEFAULT-VALUE})")
  private int maxBlockCallDataSize = DEFAULT_MAX_BLOCK_CALLDATA_SIZE;

  @Positive
  @CommandLine.Option(
      names = {OVER_LINE_COUNT_LIMIT_CACHE_SIZE},
      hidden = true,
      paramLabel = "<INTEGER>",
      description =
          "Max number of transactions that go over the line count limit we keep track of (default: ${DEFAULT-VALUE})")
  private int overLineCountLimitCacheSize = DEFAULT_OVER_LINE_COUNT_LIMIT_CACHE_SIZE;

  @Positive
  @CommandLine.Option(
      names = {MAX_GAS_PER_BLOCK},
      hidden = true,
      paramLabel = "<LONG>",
      description = "Sets max gas per block  (default: ${DEFAULT-VALUE})")
  private Long maxGasPerBlock = DEFAULT_MAX_GAS_PER_BLOCK;

  @Positive
  @CommandLine.Option(
      names = {MAX_BUNDLE_POOL_SIZE_BYTES},
      hidden = true,
      paramLabel = "<LONG>",
      description =
          "Sets max memory size, in bytes, that the bundle txpool can occupy (default: ${DEFAULT-VALUE})")
  public Long maxBundlePoolSizeBytes = DEFAULT_MAX_BUNDLE_POOL_SIZE_BYTES;

  @Positive
  @CommandLine.Option(
      names = {MAX_BUNDLE_GAS_PER_BLOCK},
      hidden = true,
      paramLabel = "<LONG>",
      description =
          "Sets max amount of block gas bundle transactions can use (default: ${DEFAULT-VALUE})")
  public Long maxBundleGasPerBlock = DEFAULT_MAX_BUNDLE_GAS_PER_BLOCK;

  @CommandLine.Option(
      names = {EVENTS_DENY_LIST_PATH},
      hidden = true,
      paramLabel = "<STRING>",
      description = "Path to the file containing the events deny list")
  private String eventsDenyListPath;

  @CommandLine.Option(
      names = {EVENTS_BUNDLE_DENY_LIST_PATH},
      hidden = true,
      paramLabel = "<STRING>",
      description = "Path to the file containing the events deny list for bundles")
  private String eventsBundleDenyListPath;

  private LineaTransactionSelectorCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static LineaTransactionSelectorCliOptions create() {
    return new LineaTransactionSelectorCliOptions();
  }

  /**
   * Linea cli options from config.
   *
   * @param config the config
   * @return the Linea cli options
   */
  public static LineaTransactionSelectorCliOptions fromConfig(
      final LineaTransactionSelectorConfiguration config) {
    final LineaTransactionSelectorCliOptions options = create();
    options.maxBlockCallDataSize = config.maxBlockCallDataSize();
    options.overLineCountLimitCacheSize = config.overLinesLimitCacheSize();
    options.maxGasPerBlock = config.maxGasPerBlock();
    options.eventsDenyListPath = config.eventsDenyListPath();
    options.eventsBundleDenyListPath = config.eventsBundleDenyListPath();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaTransactionSelectorConfiguration toDomainObject() {
    return LineaTransactionSelectorConfiguration.builder()
        .maxBlockCallDataSize(maxBlockCallDataSize)
        .overLinesLimitCacheSize(overLineCountLimitCacheSize)
        .maxGasPerBlock(maxGasPerBlock)
        .maxBundleGasPerBlock(maxBundleGasPerBlock)
        .maxBundlePoolSizeBytes(maxBundlePoolSizeBytes)
        .eventsDenyListPath(eventsDenyListPath)
        .eventsDenyList(parseTransactionEventDenyList(eventsDenyListPath))
        .eventsBundleDenyListPath(eventsBundleDenyListPath)
        .eventsBundleDenyList(parseTransactionEventDenyList(eventsBundleDenyListPath))
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(MAX_BLOCK_CALLDATA_SIZE, maxBlockCallDataSize)
        .add(OVER_LINE_COUNT_LIMIT_CACHE_SIZE, overLineCountLimitCacheSize)
        .add(MAX_GAS_PER_BLOCK, maxGasPerBlock)
        .add(MAX_BUNDLE_GAS_PER_BLOCK, maxBundleGasPerBlock)
        .add(MAX_BUNDLE_POOL_SIZE_BYTES, maxBundlePoolSizeBytes)
        .add(EVENTS_DENY_LIST_PATH, eventsDenyListPath)
        .add(EVENTS_BUNDLE_DENY_LIST_PATH, eventsBundleDenyListPath)
        .toString();
  }

  public Map<Address, Set<TransactionEventFilter>> parseTransactionEventDenyList(
      final String filename) {
    if (filename == null || filename.isEmpty()) {
      return Collections.emptyMap();
    }

    Map<Address, Set<TransactionEventFilter>> eventFilters = new ConcurrentHashMap<>();
    try (Stream<String> lines = Files.lines(Path.of(new File(filename).toURI()))) {
      for (String line : (Iterable<String>) lines::iterator) {
        if (line.isEmpty()) {
          continue;
        }
        String[] parts = line.split(",", -1);
        if (parts.length != 5) {
          throw new IllegalArgumentException(
              "Invalid transaction event filter line: "
                  + line
                  + ". Expected format: address,topic0,topic1,topic2,topic3");
        }
        var address = Address.fromHexString(parts[0]);
        var eventFilter =
            new TransactionEventFilter(
                address,
                parts[1].isEmpty() ? null : LogTopic.wrap(Bytes32.fromHexString(parts[1])),
                parts[2].isEmpty() ? null : LogTopic.wrap(Bytes32.fromHexString(parts[2])),
                parts[3].isEmpty() ? null : LogTopic.wrap(Bytes32.fromHexString(parts[3])),
                parts[4].isEmpty() ? null : LogTopic.wrap(Bytes32.fromHexString(parts[4])));
        eventFilters.computeIfAbsent(address, (a) -> new HashSet<>()).add(eventFilter);
      }
      return eventFilters;
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
  }
}
