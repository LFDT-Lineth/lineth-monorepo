/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

fun MaruPeer.toPeerInfo(): PeerInfo =
  PeerInfo(
    nodeId = id.toBase58(),
    enr = null,
    address = address.toExternalForm(),
    status = if (isConnected) PeerInfo.PeerStatus.CONNECTED else PeerInfo.PeerStatus.DISCONNECTED,
    direction = if (connectionInitiatedLocally()) {
      PeerInfo.PeerDirection.OUTBOUND
    } else {
      PeerInfo.PeerDirection.INBOUND
    },
  )
