/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.metrics

import net.consensys.linea.metrics.MetricsCategory

enum class MaruMetricsCategory : MetricsCategory {
  ENGINE_API,
  METADATA,
  P2P_NETWORK,
  STORAGE,
  SYNCHRONIZER,
  NETWORK,
  CONSENSUS,

  // TODO: Adding the following categories fixed a lot of exceptions thrown during testing. They are related to Teku
  LIBP2P,
  EXECUTOR,
  EXECUTORS,
}
