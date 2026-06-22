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
import java.math.BigInteger

class ULongSerializer : KvStoreSerializer<ULong> {
  override fun serialize(value: ULong): ByteArray = BigInteger.valueOf(value.toLong()).toByteArray()

  override fun deserialize(bytes: ByteArray): ULong = BigInteger(bytes).toLong().toULong()
}
