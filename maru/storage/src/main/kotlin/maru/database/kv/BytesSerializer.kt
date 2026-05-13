/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database.kv

import tech.pegasys.teku.storage.server.kvstore.serialization.KvStoreSerializer

class BytesSerializer : KvStoreSerializer<ByteArray> {
  override fun serialize(value: ByteArray): ByteArray = value

  override fun deserialize(bytes: ByteArray): ByteArray = bytes
}
