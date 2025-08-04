/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config;

import java.time.Duration;
import lombok.Builder;
import net.consensys.linea.plugins.LineaOptionsConfiguration;

/** The Linea liveness service validation configuration. */
@Builder(toBuilder = true)
public record LineaLivenessServiceConfiguration(
    boolean enabled,
    Duration maxBlockAgeSeconds,
    Duration bundleMaxTimestampSurplusSecond,
    String contractAddress,
    String signerUrl,
    String signerKeyId,
    String signerAddress,
    long gasLimit,
    long gasPrice)
    implements LineaOptionsConfiguration {}
