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
import java.net.URL;
import java.util.Set;
import java.util.stream.Collectors;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

/** The Linea Bundle CLI options. */
public class LineaBundleCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "bundle-sequencer";

  private static final String BUNDLES_FORWARD_URLS = "--plugin-linea-bundles-forward-urls";
  private static final Set<URL> DEFAULT_BUNDLES_FORWARD_URLS = Set.of();

  private static final String BUNDLES_FORWARD_RETRY_DELAY =
      "--plugin-linea-bundles-forward-retry-delay";
  private static final int DEFAULT_BUNDLES_FORWARD_RETRY_DELAY_MILLIS = 1000;

  private static final String BUNDLES_FORWARD_TIMEOUT = "--plugin-linea-bundles-forward-timeout";
  private static final int DEFAULT_BUNDLES_FORWARD_TIMEOUT_MILLIS = 5000;

  @CommandLine.Option(
      names = {BUNDLES_FORWARD_URLS},
      paramLabel = "<SET<URL>>",
      description =
          "A comma separated list of endpoint to which incoming bundles will be forwarded (default: ${DEFAULT-VALUE})")
  private Set<URL> forwardUrls = DEFAULT_BUNDLES_FORWARD_URLS;

  @CommandLine.Option(
      names = {BUNDLES_FORWARD_RETRY_DELAY},
      paramLabel = "<INTEGER>",
      description =
          "Number of milliseconds to wait before retrying a failed forward (default: ${DEFAULT-VALUE})")
  private int retryDelayMillis = DEFAULT_BUNDLES_FORWARD_RETRY_DELAY_MILLIS;

  @CommandLine.Option(
      names = {BUNDLES_FORWARD_TIMEOUT},
      paramLabel = "<INTEGER>",
      description =
          "Number of milliseconds to wait before a forward times out (default: ${DEFAULT-VALUE})")
  private int timeoutMillis = DEFAULT_BUNDLES_FORWARD_TIMEOUT_MILLIS;

  private LineaBundleCliOptions() {}

  /**
   * Create Linea Bundle CLI options.
   *
   * @return the Linea RPC Bundle options
   */
  public static LineaBundleCliOptions create() {
    return new LineaBundleCliOptions();
  }

  /**
   * Linea Bundle CLI options from config.
   *
   * @param config the config
   * @return the Linea Bundle CLI options
   */
  public static LineaBundleCliOptions fromConfig(final LineaBundleConfiguration config) {
    final LineaBundleCliOptions options = create();
    options.forwardUrls = config.forwardUrls();
    options.retryDelayMillis = config.retryDelayMillis();
    options.timeoutMillis = config.timeoutMillis();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaBundleConfiguration toDomainObject() {
    return LineaBundleConfiguration.builder()
        .forwardUrls(forwardUrls)
        .retryDelayMillis(retryDelayMillis)
        .timeoutMillis(timeoutMillis)
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(
            BUNDLES_FORWARD_URLS,
            forwardUrls.stream().map(URL::toString).collect(Collectors.joining(",")))
        .add(BUNDLES_FORWARD_RETRY_DELAY, retryDelayMillis)
        .add(BUNDLES_FORWARD_TIMEOUT, timeoutMillis)
        .toString();
  }
}
