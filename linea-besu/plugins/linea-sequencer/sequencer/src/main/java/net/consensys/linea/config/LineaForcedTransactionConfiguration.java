/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config;

import lombok.Builder;
import net.consensys.linea.plugins.LineaOptionsConfiguration;
import net.consensys.linea.sequencer.forced.LineaForcedTransactionPool;

/** Configuration for the forced transaction pool. */
@Builder
public record LineaForcedTransactionConfiguration(int statusCacheSize)
    implements LineaOptionsConfiguration {

  public static final LineaForcedTransactionConfiguration DEFAULT =
      LineaForcedTransactionConfiguration.builder()
          .statusCacheSize(LineaForcedTransactionPool.DEFAULT_STATUS_CACHE_SIZE)
          .build();
}
