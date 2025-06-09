/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.utils;

import linea.blob.BlobCompressor;
import linea.blob.BlobCompressorVersion;
import linea.blob.GoBackedBlobCompressor;
import org.apache.logging.log4j.LogManager;

public class Compressor {
  public static BlobCompressor instance;

  static {
    try {
      instance =
          GoBackedBlobCompressor.getInstance(
              BlobCompressorVersion.V1_2,
              // 100KB to match coordinator config.
              // However, is not relevant for the sequencer because it does not create blobs.
              102400);
    } catch (Throwable t) {
      LogManager.getLogger(Compressor.class)
          .error("Failed to load GoBackedBlobCompressor. errorMessage={}", t.getMessage(), t);
      throw new RuntimeException("Failed to load GoBackedBlobCompressor", t);
    }
  }
}
