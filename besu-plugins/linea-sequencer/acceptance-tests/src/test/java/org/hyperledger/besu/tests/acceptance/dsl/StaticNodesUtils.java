/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package org.hyperledger.besu.tests.acceptance.dsl;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.stream.Collectors;

public class StaticNodesUtils {

  public static Path createStaticNodesFile(final Path directory, final List<String> staticNodes) {
    try {
      final Path tempFile = Files.createTempFile(directory, "", "");
      tempFile.toFile().deleteOnExit();

      final Path staticNodesFile = tempFile.getParent().resolve("static-nodes.json");
      Files.move(tempFile, staticNodesFile);
      staticNodesFile.toFile().deleteOnExit();

      final String json =
          staticNodes.stream()
              .map(s -> String.format("\"%s\"", s))
              .collect(Collectors.joining(",", "[", "]"));

      Files.writeString(staticNodesFile, json);

      return staticNodesFile;
    } catch (final IOException e) {
      throw new IllegalStateException(e);
    }
  }
}
