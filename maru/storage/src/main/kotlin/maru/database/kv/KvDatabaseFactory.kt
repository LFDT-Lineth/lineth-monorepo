/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database.kv

import maru.serialization.rlp.ForkAwareBlockHashing
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.plugin.services.MetricsSystem
import org.hyperledger.besu.plugin.services.metrics.MetricCategory
import tech.pegasys.teku.storage.server.kvstore.KvStoreConfiguration
import tech.pegasys.teku.storage.server.rocksdb.RocksDbInstanceFactory
import java.nio.file.Path

object KvDatabaseFactory {
  fun createRocksDbDatabase(
    databasePath: Path,
    metricsSystem: MetricsSystem,
    metricCategory: MetricCategory,
    blockHashing: ForkAwareBlockHashing,
  ): KvDatabase {
    val kvStoreSerializers =
      KvStoreSerializers(
        beaconStateSerializer = blockHashing.beaconStateSerializer,
        sealedBeaconBlockSerializer = blockHashing.sealedBeaconBlockSerializer,
      )
    val schema = KvDatabase.Schema(kvStoreSerializers)
    val rocksDbInstance =
      RocksDbInstanceFactory.create(
        metricsSystem,
        metricCategory,
        KvStoreConfiguration().withDatabaseDir(databasePath),
        listOf(
          schema.SealedBeaconBlockByBlockRoot,
          schema.BeaconBlockRootByBlockNumber,
          schema.BeaconStateByBlockRoot,
        ),
        emptyList<Bytes>(),
        emptyList(),
        emptyList<Bytes>(),
      )
    return KvDatabase(rocksDbInstance, schema)
  }
}
