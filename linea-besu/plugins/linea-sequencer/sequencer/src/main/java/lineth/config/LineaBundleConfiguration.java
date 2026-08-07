/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package lineth.config;

import java.net.URL;
import java.util.Set;
import lineth.plugins.LineaOptionsConfiguration;
import lombok.Builder;
import lombok.Getter;
import lombok.ToString;
import lombok.experimental.Accessors;

/** The Linea Bundle configuration. */
@Builder(toBuilder = true)
@Accessors(fluent = true)
@Getter
@ToString
public class LineaBundleConfiguration implements LineaOptionsConfiguration {
  private Set<URL> forwardUrls;
  private int retryDelayMillis;
  private int timeoutMillis;
}
