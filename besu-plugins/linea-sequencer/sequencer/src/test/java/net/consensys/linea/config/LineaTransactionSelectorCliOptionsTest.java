/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.config;

import static org.junit.jupiter.api.Assertions.*;

import java.util.Map;
import kotlin.time.Instant;
import linea.blob.BlobCompressorVersion;
import org.junit.jupiter.api.Test;

class LineaTransactionSelectorCliOptionsTest {

  @Test
  void parseBlobCompressorVersionTimestamps_validInput() {
    LineaTransactionSelectorCliOptions opts = new LineaTransactionSelectorCliOptions();
    String input = "V1_2=2025-01-01T00:00:00Z,V2=2026-01-01T00:00:00Z";
    Map<BlobCompressorVersion, Instant> result = opts.parseBlobCompressorVersionTimestamps(input);
    assertEquals(2, result.size());
    assertEquals(
        Instant.Companion.parse("2025-01-01T00:00:00Z"), result.get(BlobCompressorVersion.V1_2));
    assertEquals(
        Instant.Companion.parse("2026-01-01T00:00:00Z"), result.get(BlobCompressorVersion.V2));
  }

  @Test
  void parseBlobCompressorVersionTimestamps_invalidInput() {
    LineaTransactionSelectorCliOptions opts = new LineaTransactionSelectorCliOptions();
    String input = "V1_2=2025-01-01T00:00:00Z,V2";
    Exception ex =
        assertThrows(
            IllegalArgumentException.class, () -> opts.parseBlobCompressorVersionTimestamps(input));
    assertTrue(ex.getMessage().contains("Invalid BlobCompressorVersion=Instant pair"));
  }
}
