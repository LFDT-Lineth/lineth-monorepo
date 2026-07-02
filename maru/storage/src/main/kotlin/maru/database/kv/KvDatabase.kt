/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.database.kv

import maru.core.BeaconState
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.database.P2PState
import tech.pegasys.teku.storage.server.kvstore.KvStoreAccessor
import tech.pegasys.teku.storage.server.kvstore.KvStoreAccessor.KvStoreTransaction
import tech.pegasys.teku.storage.server.kvstore.schema.KvStoreColumn
import tech.pegasys.teku.storage.server.kvstore.schema.KvStoreVariable
import kotlin.jvm.optionals.getOrDefault
import kotlin.jvm.optionals.getOrNull

class KvDatabase(
  private val kvStoreAccessor: KvStoreAccessor,
  private val schema: Schema,
) : BeaconChain,
  P2PState {
  override fun isInitialized(): Boolean = kvStoreAccessor.get(schema.LatestBeaconState).getOrNull() != null

  class Schema(
    kvStoreSerializers: KvStoreSerializers,
  ) {
    val BeaconStateByBlockRoot: KvStoreColumn<ByteArray, BeaconState> =
      KvStoreColumn.create(
        1,
        kvStoreSerializers.bytesSerializer,
        kvStoreSerializers.beaconStateSerializer,
      )

    val SealedBeaconBlockByBlockRoot: KvStoreColumn<ByteArray, SealedBeaconBlock> =
      KvStoreColumn.create(
        2,
        kvStoreSerializers.bytesSerializer,
        kvStoreSerializers.sealedBeaconBlockSerializer,
      )

    val BeaconBlockRootByBlockNumber: KvStoreColumn<ULong, ByteArray> =
      KvStoreColumn.create(
        3,
        kvStoreSerializers.uLongSerializer,
        kvStoreSerializers.bytesSerializer,
      )

    val LatestBeaconState: KvStoreVariable<BeaconState> =
      KvStoreVariable.create(
        1,
        kvStoreSerializers.beaconStateSerializer,
      )

    val DiscoverySequenceNumber: KvStoreVariable<ULong> =
      KvStoreVariable.create(
        2,
        kvStoreSerializers.uLongSerializer,
      )
  }

  override fun getLatestBeaconState(): BeaconState = kvStoreAccessor.get(schema.LatestBeaconState).get()

  override fun getBeaconState(beaconBlockRoot: ByteArray): BeaconState? =
    kvStoreAccessor.get(schema.BeaconStateByBlockRoot, beaconBlockRoot).getOrNull()

  override fun getBeaconState(beaconBlockNumber: ULong): BeaconState? =
    kvStoreAccessor
      .get(schema.BeaconBlockRootByBlockNumber, beaconBlockNumber)
      .flatMap { blockRoot -> kvStoreAccessor.get(schema.BeaconStateByBlockRoot, blockRoot) }
      .getOrNull()

  override fun getSealedBeaconBlock(beaconBlockRoot: ByteArray): SealedBeaconBlock? =
    kvStoreAccessor.get(schema.SealedBeaconBlockByBlockRoot, beaconBlockRoot).getOrNull()

  override fun getSealedBeaconBlock(beaconBlockNumber: ULong): SealedBeaconBlock? =
    kvStoreAccessor
      .get(schema.BeaconBlockRootByBlockNumber, beaconBlockNumber)
      .flatMap { blockRoot -> kvStoreAccessor.get(schema.SealedBeaconBlockByBlockRoot, blockRoot) }
      .getOrNull()

  override fun newBeaconChainUpdater(): BeaconChain.Updater = KvUpdater(this.kvStoreAccessor, this.schema)

  override fun getLocalNodeRecordSequenceNumber(): ULong =
    kvStoreAccessor
      .get(schema.DiscoverySequenceNumber)
      .getOrDefault(0uL)

  override fun newP2PStateUpdater(): P2PState.Updater = KvUpdater(this.kvStoreAccessor, this.schema)

  override fun close() {
    kvStoreAccessor.close()
  }

  class KvUpdater(
    kvStoreAccessor: KvStoreAccessor,
    private val schema: Schema,
  ) : BeaconChain.Updater,
    P2PState.Updater {
    private val transaction: KvStoreTransaction = kvStoreAccessor.startTransaction()

    override fun putBeaconState(beaconState: BeaconState): BeaconChain.Updater {
      transaction.put(schema.BeaconStateByBlockRoot, beaconState.beaconBlockHeader.hash, beaconState)
      transaction.put(schema.LatestBeaconState, beaconState)
      return this
    }

    override fun putSealedBeaconBlock(sealedBeaconBlock: SealedBeaconBlock): BeaconChain.Updater {
      transaction.put(
        schema.SealedBeaconBlockByBlockRoot,
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash,
        sealedBeaconBlock,
      )
      transaction.put(
        schema.BeaconBlockRootByBlockNumber,
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.number,
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash,
      )

      return this
    }

    override fun putDiscoverySequenceNumber(newSequenceNumber: ULong): P2PState.Updater {
      transaction.put(schema.DiscoverySequenceNumber, newSequenceNumber)
      return this
    }

    override fun commit() {
      transaction.commit()
    }

    override fun rollback() {
      transaction.rollback()
    }

    override fun close() {
      transaction.close()
    }
  }
}
