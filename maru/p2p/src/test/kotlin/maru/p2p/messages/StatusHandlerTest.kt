/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.core.ext.DataGenerators
import maru.p2p.MaruPeer
import maru.p2p.Message
import maru.p2p.MessageData
import maru.p2p.RequestMessageAdapter
import maru.p2p.RpcMessageType
import maru.p2p.Version
import org.junit.jupiter.api.Test
import org.mockito.Mockito.mock
import org.mockito.kotlin.any
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.networking.eth2.rpc.core.ResponseCallback
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcResponseStatus
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import kotlin.random.Random

class StatusHandlerTest {
  @Test
  fun `responds with current status`() {
    val statusManager = mock<StatusManager>()
    val localBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val forkIdHash = Random.nextBytes(32)
    val localStatusMessage =
      MessageData(
        RpcMessageType.STATUS,
        Version.V1,
        Status(
          forkIdHash,
          localBeaconState.beaconBlockHeader.hash,
          localBeaconState.beaconBlockHeader.number,
        ),
      )
    whenever(statusManager.createStatusMessage()).thenReturn(localStatusMessage)
    whenever(statusManager.isValidForPeering(any())).thenReturn(true)

    val peer = mock<MaruPeer>()
    val callback = mock<ResponseCallback<Message<Status, RpcMessageType>>>()
    val statusHandler = StatusHandler(statusManager)
    val remoteBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val remoteStatusMessage =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          Status(
            forkIdHash,
            remoteBeaconState.beaconBlockHeader.hash,
            remoteBeaconState.beaconBlockHeader.number,
          ),
        ),
      )
    statusHandler.handleIncomingMessage(peer, remoteStatusMessage, callback)
    verify(callback).respondAndCompleteSuccessfully(localStatusMessage)
  }

  @Test
  fun `updates peer status`() {
    val statusManager = mock<StatusManager>()
    val forkIdHash = Random.nextBytes(32)
    val blockHash = Random.nextBytes(32)
    val blockNumber = 0UL
    val statusMessage = mock<Message<Status, RpcMessageType>>()
    val status = Status(forkIdHash, blockHash, blockNumber)
    val remoteBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val payload =
      Status(
        forkIdHash = forkIdHash,
        latestStateRoot = remoteBeaconState.beaconBlockHeader.hash,
        latestBlockNumber = remoteBeaconState.beaconBlockHeader.number,
      )

    whenever(statusManager.isValidForPeering(any())).thenReturn(true)
    whenever(statusManager.createStatusMessage()).thenReturn(statusMessage)
    whenever(statusMessage.payload).thenReturn(status)

    val peer = mock<MaruPeer>()
    val callback = mock<ResponseCallback<Message<Status, RpcMessageType>>>()
    val statusHandler = StatusHandler(statusManager)
    val remoteStatusMessage =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          payload,
        ),
      )
    statusHandler.handleIncomingMessage(peer, remoteStatusMessage, callback)

    verify(peer).updateStatus(remoteStatusMessage.payload)
  }

  @Test
  fun `updates peer status when forkIdHasJustChangedTo returns true`() {
    val statusManager = mock<StatusManager>()
    val forkIdHash = ByteArray(32) { 0x00.toByte() }
    val otherForkIdHash = ByteArray(32) { 0xFF.toByte() }
    val blockHash = Random.nextBytes(32)
    val blockNumber = 0UL

    val statusMessage = mock<Message<Status, RpcMessageType>>()
    val status = Status(forkIdHash, blockHash, blockNumber)
    val remoteBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val payload =
      Status(
        forkIdHash = otherForkIdHash,
        latestStateRoot = remoteBeaconState.beaconBlockHeader.hash,
        latestBlockNumber = remoteBeaconState.beaconBlockHeader.number,
      )

    whenever(statusManager.createStatusMessage()).thenReturn(statusMessage)
    whenever(statusManager.isValidForPeering(any())).thenReturn(true)
    whenever(statusMessage.payload).thenReturn(status)

    val peer = mock<MaruPeer>()
    val callback = mock<ResponseCallback<Message<Status, RpcMessageType>>>()
    val statusHandler = StatusHandler(statusManager)
    val remoteStatusMessage =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          payload,
        ),
      )
    statusHandler.handleIncomingMessage(peer, remoteStatusMessage, callback)

    verify(peer).updateStatus(remoteStatusMessage.payload)
  }

  @Test
  fun `handles request with Rpc exception`() {
    val statusManager = mock<StatusManager>()
    whenever(statusManager.isValidForPeering(any())).thenReturn(true)
    whenever(statusManager.createStatusMessage()).thenAnswer {
      throw RpcException(RpcResponseStatus.RESOURCE_UNAVAILABLE, "createStatusMessage exception testing")
    }

    val peer = mock<MaruPeer>()
    val callback = mock<ResponseCallback<Message<Status, RpcMessageType>>>()
    val statusHandler = StatusHandler(statusManager)
    val remoteBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val remoteStatusMessage =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          Status(
            Random.nextBytes(32),
            remoteBeaconState.beaconBlockHeader.hash,
            remoteBeaconState.beaconBlockHeader.number,
          ),
        ),
      )

    statusHandler.handleIncomingMessage(peer, remoteStatusMessage, callback)

    verify(callback).completeWithErrorResponse(
      RpcException(
        RpcResponseStatus.RESOURCE_UNAVAILABLE,
        "createStatusMessage exception testing",
      ),
    )
  }

  @Test
  fun `disconnects peer when forkIdHash is wrong`() {
    val statusManager = mock<StatusManager>()
    val forkIdHash = ByteArray(32) { 0x00.toByte() }
    val wrongForkIdHash = ByteArray(32) { 0xFF.toByte() }
    val blockHash = Random.nextBytes(32)
    val blockNumber = 0UL

    val statusMessage = mock<Message<Status, RpcMessageType>>()
    val status = Status(forkIdHash, blockHash, blockNumber)
    val remoteBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val payload =
      Status(
        forkIdHash = wrongForkIdHash,
        latestStateRoot = remoteBeaconState.beaconBlockHeader.hash,
        latestBlockNumber = remoteBeaconState.beaconBlockHeader.number,
      )

    whenever(statusManager.createStatusMessage()).thenReturn(statusMessage)
    whenever(statusManager.isValidForPeering(any())).thenReturn(false)
    whenever(statusMessage.payload).thenReturn(status)

    val peer = mock<MaruPeer>()
    val callback = mock<ResponseCallback<Message<Status, RpcMessageType>>>()
    val statusHandler = StatusHandler(statusManager)
    val remoteStatusMessage =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          payload,
        ),
      )
    statusHandler.handleIncomingMessage(peer, remoteStatusMessage, callback)

    verify(peer).disconnectCleanly(DisconnectReason.IRRELEVANT_NETWORK)
  }

  @Test
  fun `handles request with exception`() {
    val statusManager = mock<StatusManager>()
    whenever(statusManager.isValidForPeering(any())).thenReturn(true)
    whenever(statusManager.createStatusMessage()).thenThrow(
      IllegalStateException("createStatusMessage exception testing"),
    )

    val peer = mock<MaruPeer>()
    val callback = mock<ResponseCallback<Message<Status, RpcMessageType>>>()
    val statusHandler = StatusHandler(statusManager)
    val remoteBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val remoteStatusMessage =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          Status(
            Random.nextBytes(32),
            remoteBeaconState.beaconBlockHeader.hash,
            remoteBeaconState.beaconBlockHeader.number,
          ),
        ),
      )

    statusHandler.handleIncomingMessage(peer, remoteStatusMessage, callback)

    verify(callback).completeWithUnexpectedError(
      RpcException(
        RpcResponseStatus.SERVER_ERROR_CODE,
        "Handling request failed with unexpected error: createStatusMessage exception testing",
      ),
    )
  }

  @Test
  fun `disconnects peer when status is invalid`() {
    val statusManager = mock<StatusManager>()
    val localBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val forkIdHash = Random.nextBytes(32)
    val localStatusMessage =
      MessageData(
        RpcMessageType.STATUS,
        Version.V1,
        Status(
          forkIdHash,
          localBeaconState.beaconBlockHeader.hash,
          localBeaconState.beaconBlockHeader.number,
        ),
      )
    whenever(statusManager.createStatusMessage()).thenReturn(localStatusMessage)
    whenever(statusManager.isValidForPeering(any())).thenReturn(false)

    val peer = mock<MaruPeer>()
    val callback = mock<ResponseCallback<Message<Status, RpcMessageType>>>()
    val statusHandler = StatusHandler(statusManager)
    val remoteBeaconState = DataGenerators.randomBeaconState(
      0U,
    )
    val remoteStatusMessage =
      RequestMessageAdapter(
        MessageData(
          RpcMessageType.STATUS,
          Version.V1,
          Status(
            forkIdHash,
            remoteBeaconState.beaconBlockHeader.hash,
            remoteBeaconState.beaconBlockHeader.number,
          ),
        ),
      )
    statusHandler.handleIncomingMessage(peer, remoteStatusMessage, callback)
    verify(peer).disconnectCleanly(DisconnectReason.IRRELEVANT_NETWORK)
  }
}
