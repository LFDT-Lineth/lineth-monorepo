/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config;

import java.nio.file.Path;
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
    SignerType signerType,
    String signerName,
    String signerUrl,
    String signerKeyId,
    String signerAddress,
    boolean tlsEnabled,
    Path tlsKeyStorePath,
    String tlsKeyStorePassword,
    Path tlsTrustStorePath,
    String tlsTrustStorePassword,
    long gasLimit,
    long gasPrice)
    implements LineaOptionsConfiguration {
  public LineaLivenessServiceConfiguration {
    signerType = signerType == null ? SignerType.WEB3SIGNER : signerType;
  }

  public LineaLivenessServiceConfiguration(
      final boolean enabled,
      final Duration maxBlockAgeSeconds,
      final Duration bundleMaxTimestampSurplusSecond,
      final String contractAddress,
      final String signerUrl,
      final String signerKeyId,
      final String signerAddress,
      final boolean tlsEnabled,
      final Path tlsKeyStorePath,
      final String tlsKeyStorePassword,
      final Path tlsTrustStorePath,
      final String tlsTrustStorePassword,
      final long gasLimit,
      final long gasPrice) {
    this(
        enabled,
        maxBlockAgeSeconds,
        bundleMaxTimestampSurplusSecond,
        contractAddress,
        SignerType.WEB3SIGNER,
        null,
        signerUrl,
        signerKeyId,
        signerAddress,
        tlsEnabled,
        tlsKeyStorePath,
        tlsKeyStorePassword,
        tlsTrustStorePath,
        tlsTrustStorePassword,
        gasLimit,
        gasPrice);
  }

  public enum SignerType {
    WEB3SIGNER,
    CUSTOM
  }
}
