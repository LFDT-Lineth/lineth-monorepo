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

package net.consensys.linea.tracewriter;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class TraceWriterVirtualBlockTest {

  @TempDir Path tempDir;

  // ---- file name format ----

  @Test
  void fileNameFollowsNonCanonicalConflatedFormat() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path = writer.virtualBlockTraceFilePath(100L, "v2.0.0", "abc123");
    assertThat(path.getFileName().toString())
        .isEqualTo("100-abc123-noncanonical.conflated.v2.0.0.lt");
  }

  @Test
  void fileNameUsesGzExtensionWhenCompressionEnabled() {
    TraceWriter writer = new TraceWriter(tempDir, true);
    Path path = writer.virtualBlockTraceFilePath(200L, "v1.0.0", "deadbeef");
    assertThat(path.getFileName().toString())
        .isEqualTo("200-deadbeef-noncanonical.conflated.v1.0.0.lt.gz");
  }

  @Test
  void fileNameContainsBlockNumber() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path = writer.virtualBlockTraceFilePath(9876L, "v1", "hash");
    assertThat(path.getFileName().toString()).startsWith("9876-");
  }

  @Test
  void fileNameContainsTxsHash() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path = writer.virtualBlockTraceFilePath(1L, "v1", "myhash42");
    assertThat(path.getFileName().toString()).contains("myhash42");
  }

  @Test
  void fileNameContainsTracesEngineVersion() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path = writer.virtualBlockTraceFilePath(1L, "v3.1.4", "hash");
    assertThat(path.getFileName().toString()).contains("v3.1.4");
  }

  // ---- path location ----

  @Test
  void pathIsUnderOutputDirectory() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path = writer.virtualBlockTraceFilePath(1L, "v0", "hash");
    assertThat(path.toString()).startsWith(tempDir.toString());
  }

  // ---- cache key isolation ----

  @Test
  void differentTxsHashesProduceDifferentPaths() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path1 = writer.virtualBlockTraceFilePath(100L, "v1", "hash1");
    Path path2 = writer.virtualBlockTraceFilePath(100L, "v1", "hash2");
    assertThat(path1).isNotEqualTo(path2);
  }

  @Test
  void differentBlockNumbersProduceDifferentPaths() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path1 = writer.virtualBlockTraceFilePath(99L, "v1", "samehash");
    Path path2 = writer.virtualBlockTraceFilePath(100L, "v1", "samehash");
    assertThat(path1).isNotEqualTo(path2);
  }

  @Test
  void differentVersionsProduceDifferentPaths() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path1 = writer.virtualBlockTraceFilePath(100L, "v1.0", "samehash");
    Path path2 = writer.virtualBlockTraceFilePath(100L, "v2.0", "samehash");
    assertThat(path1).isNotEqualTo(path2);
  }

  @Test
  void sameInputsProduceSamePath() {
    TraceWriter writer = new TraceWriter(tempDir, false);
    Path path1 = writer.virtualBlockTraceFilePath(100L, "v1", "abc");
    Path path2 = writer.virtualBlockTraceFilePath(100L, "v1", "abc");
    assertThat(path1).isEqualTo(path2);
  }
}
