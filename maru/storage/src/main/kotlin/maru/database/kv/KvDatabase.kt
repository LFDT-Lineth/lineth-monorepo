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
) : BeaconChain,
  P2PState {
  override fun isInitialized(): Boolean = kvStoreAccessor.get(Schema.LatestBeaconState).getOrNull() != null

  companion object {
    object Schema {
      val BeaconStateByBlockRoot: KvStoreColumn<ByteArray, BeaconState> =
        KvStoreColumn.create(
          1,
          KvStoreSerializers.BytesSerializer,
          KvStoreSerializers.BeaconStateSerializer,
        )

      val SealedBeaconBlockByBlockRoot: KvStoreColumn<ByteArray, SealedBeaconBlock> =
        KvStoreColumn.create(
          2,
          KvStoreSerializers.BytesSerializer,
          KvStoreSerializers.SealedBeaconBlockSerializer,
        )

      val BeaconBlockRootByBlockNumber: KvStoreColumn<ULong, ByteArray> =
        KvStoreColumn.create(
          3,
          KvStoreSerializers.ULongSerializer,
          KvStoreSerializers.BytesSerializer,
        )

      val LatestBeaconState: KvStoreVariable<BeaconState> =
        KvStoreVariable.create(
          1,
          KvStoreSerializers.BeaconStateSerializer,
        )

      val DiscoverySequenceNumber: KvStoreVariable<ULong> =
        KvStoreVariable.create(
          2,
          KvStoreSerializers.ULongSerializer,
        )
    }
  }

  override fun getLatestBeaconState(): BeaconState = kvStoreAccessor.get(Schema.LatestBeaconState).get()

  override fun getBeaconState(beaconBlockRoot: ByteArray): BeaconState? =
    kvStoreAccessor.get(Schema.BeaconStateByBlockRoot, beaconBlockRoot).getOrNull()

  override fun getBeaconState(beaconBlockNumber: ULong): BeaconState? =
    kvStoreAccessor
      .get(Schema.BeaconBlockRootByBlockNumber, beaconBlockNumber)
      .flatMap { blockRoot -> kvStoreAccessor.get(Schema.BeaconStateByBlockRoot, blockRoot) }
      .getOrNull()

  override fun getSealedBeaconBlock(beaconBlockRoot: ByteArray): SealedBeaconBlock? =
    kvStoreAccessor.get(Schema.SealedBeaconBlockByBlockRoot, beaconBlockRoot).getOrNull()

  override fun getSealedBeaconBlock(beaconBlockNumber: ULong): SealedBeaconBlock? =
    kvStoreAccessor
      .get(Schema.BeaconBlockRootByBlockNumber, beaconBlockNumber)
      .flatMap { blockRoot -> kvStoreAccessor.get(Schema.SealedBeaconBlockByBlockRoot, blockRoot) }
      .getOrNull()

  override fun newBeaconChainUpdater(): BeaconChain.Updater = KvUpdater(this.kvStoreAccessor)

  override fun getLocalNodeRecordSequenceNumber(): ULong =
    kvStoreAccessor
      .get(Schema.DiscoverySequenceNumber)
      .getOrDefault(0uL)

  override fun newP2PStateUpdater(): P2PState.Updater = KvUpdater(this.kvStoreAccessor)

  override fun close() {
    kvStoreAccessor.close()
  }

  class KvUpdater(
    kvStoreAccessor: KvStoreAccessor,
  ) : BeaconChain.Updater,
    P2PState.Updater {
    private val transaction: KvStoreTransaction = kvStoreAccessor.startTransaction()

    override fun putBeaconState(beaconState: BeaconState): BeaconChain.Updater {
      transaction.put(Schema.BeaconStateByBlockRoot, beaconState.beaconBlockHeader.hash, beaconState)
      transaction.put(Schema.LatestBeaconState, beaconState)
      return this
    }

    override fun putSealedBeaconBlock(sealedBeaconBlock: SealedBeaconBlock): BeaconChain.Updater {
      transaction.put(
        Schema.SealedBeaconBlockByBlockRoot,
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash,
        sealedBeaconBlock,
      )
      transaction.put(
        Schema.BeaconBlockRootByBlockNumber,
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.number,
        sealedBeaconBlock.beaconBlock.beaconBlockHeader.hash,
      )

      return this
    }

    override fun putDiscoverySequenceNumber(newSequenceNumber: ULong): P2PState.Updater {
      transaction.put(Schema.DiscoverySequenceNumber, newSequenceNumber)
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
