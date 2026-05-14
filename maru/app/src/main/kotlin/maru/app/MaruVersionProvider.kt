/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.VersionProvider

class MaruVersionProvider : VersionProvider {
  // TODO: Get the version from a build file or environment variable
  override fun getVersion(): String = "maru/v0.1.0"
}
