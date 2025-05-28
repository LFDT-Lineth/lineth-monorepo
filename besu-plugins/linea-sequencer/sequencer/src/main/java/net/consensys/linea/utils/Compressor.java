/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
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
