/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package chaos

import java.nio.file.Files
import java.nio.file.Path
import kotlin.collections.ifEmpty
import kotlin.text.isNotBlank
import kotlin.text.trim

object SetupHelper {
  /**
   *  Parses a file with following format:
   *  label = url
   *
   *  besu-follower-0 = http://127.0.0.1:18545
   *  besu-follower-1 = http://127.0.0.1:28545
   *  besu-sequencer-0 = http://127.0.0.1:58545
   */
  fun getNodesUrlsFromFile(filePath: Path): List<NodeInfo<String>> {
    require(Files.exists(filePath)) { "file does not exist: $filePath" }
    return parseLines(
      Files
        .readAllLines(filePath),
    ).ifEmpty { throw IllegalStateException("No valid URLs found in file: $filePath") }
  }

  fun parseLines(lines: List<String>): List<NodeInfo<String>> =
    lines
      .filter { it.isNotBlank() }
      .map {
        it
          .trim()
          .split("=")
          .map { it.trim() }
          .filter { it.isNotBlank() }
      }.filter { lineTouples -> lineTouples.isNotEmpty() }
      .map { lineTouples ->
        if (lineTouples.size == 1) {
          NodeInfo(lineTouples.first(), lineTouples.first())
        } else {
          NodeInfo(lineTouples.first(), lineTouples[1])
        }
      }
}
