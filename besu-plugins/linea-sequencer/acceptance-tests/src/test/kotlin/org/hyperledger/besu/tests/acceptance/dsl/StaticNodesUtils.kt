/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package org.hyperledger.besu.tests.acceptance.dsl

import java.nio.file.Files
import java.nio.file.Path
import kotlin.io.path.moveTo
import kotlin.io.path.writeText

object StaticNodesUtils {

  @JvmStatic
  fun createStaticNodesFile(directory: Path, staticNodes: List<String>): Path {
    try {
      val tempFile = Files.createTempFile(directory, "", "")
      tempFile.toFile().deleteOnExit()

      val staticNodesFile = tempFile.parent.resolve("static-nodes.json")
      tempFile.moveTo(staticNodesFile)
      staticNodesFile.toFile().deleteOnExit()

      val json = staticNodes.joinToString(separator = ",", prefix = "[", postfix = "]") {
        "\"$it\""
      }

      staticNodesFile.writeText(json)

      return staticNodesFile
    } catch (e: Exception) {
      throw IllegalStateException(e)
    }
  }
}
