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
import maru.database.BeaconChain
import maru.p2p.MaruPeer
import maru.p2p.Message
import maru.p2p.MessageData
import maru.p2p.RequestMessageAdapter
import maru.p2p.RpcMessageType
import maru.p2p.Version
import maru.p2p.messages.BeaconBlocksByRangeHandler.Companion.MAX_BLOCKS_PER_REQUEST
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.argumentCaptor
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.networking.eth2.rpc.core.ResponseCallback
import tech.pegasys.teku.networking.eth2.rpc.core.RpcException
import tech.pegasys.teku.networking.eth2.rpc.core.RpcResponseStatus

class BeaconBlocksByRangeHandlerTest {
  private lateinit var beaconChain: BeaconChain
  private lateinit var handler: BeaconBlocksByRangeHandler
  private lateinit var peer: MaruPeer
  private lateinit var callback: ResponseCallback<Message<BeaconBlocksByRangeResponse, RpcMessageType>>

  @BeforeEach
  fun setup() {
    beaconChain = mock()
    handler = BeaconBlocksByRangeHandler(beaconChain)
    peer = mock()
    callback = mock()
  }

  @Test
  fun `handles request with no blocks available`() {
    val request = BeaconBlocksByRangeRequest(startBlockNumber = 100UL, count = 10UL)
    val message =
      RequestMessageAdapter(
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          version = Version.V1,
          payload = request,
        ),
      )

    whenever(beaconChain.getSealedBeaconBlocks(100UL, 10UL)).thenReturn(emptyList())

    handler.handleIncomingMessage(peer, message, callback)

    val responseCaptor = argumentCaptor<Message<BeaconBlocksByRangeResponse, RpcMessageType>>()
    verify(callback).respondAndCompleteSuccessfully(responseCaptor.capture())

    val response = responseCaptor.firstValue
    assertThat(response.type).isEqualTo(RpcMessageType.BEACON_BLOCKS_BY_RANGE)
    assertThat(response.payload.blocks).isEmpty()
  }

  @Test
  fun `handles request with blocks available`() {
    val request = BeaconBlocksByRangeRequest(startBlockNumber = 100UL, count = 3UL)
    val message =
      RequestMessageAdapter(
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          version = Version.V1,
          payload = request,
        ),
      )

    val blocks =
      listOf(
        DataGenerators.randomSealedBeaconBlock(
          number = 100UL,
        ),
        DataGenerators.randomSealedBeaconBlock(
          number = 101UL,
        ),
        DataGenerators.randomSealedBeaconBlock(
          number = 102UL,
        ),
      )

    whenever(beaconChain.getSealedBeaconBlocks(100UL, 3UL)).thenReturn(blocks)

    handler.handleIncomingMessage(peer, message, callback)

    val responseCaptor = argumentCaptor<Message<BeaconBlocksByRangeResponse, RpcMessageType>>()
    verify(callback).respondAndCompleteSuccessfully(responseCaptor.capture())

    val response = responseCaptor.firstValue
    assertThat(response.type).isEqualTo(RpcMessageType.BEACON_BLOCKS_BY_RANGE)
    assertThat(response.payload.blocks).hasSize(3)
    assertThat(response.payload.blocks).isEqualTo(blocks)
  }

  @Test
  fun `handles large count request`() {
    val request = BeaconBlocksByRangeRequest(startBlockNumber = 0UL, count = 1000UL)
    val message =
      RequestMessageAdapter(
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          version = Version.V1,
          payload = request,
        ),
      )

    // Handler should limit to MAX_BLOCKS_PER_REQUEST
    val limitedBlocks =
      (0UL until MAX_BLOCKS_PER_REQUEST).map { i ->
        DataGenerators.randomSealedBeaconBlock(
          number = i,
        )
      }

    // handler should limit to MAX_BLOCKS_PER_REQUEST
    whenever(beaconChain.getSealedBeaconBlocks(0UL, MAX_BLOCKS_PER_REQUEST)).thenReturn(limitedBlocks)

    handler.handleIncomingMessage(peer, message, callback)

    val responseCaptor = argumentCaptor<Message<BeaconBlocksByRangeResponse, RpcMessageType>>()
    verify(callback).respondAndCompleteSuccessfully(responseCaptor.capture())

    val response = responseCaptor.firstValue
    assertThat(response.payload.blocks).hasSize(256)

    // Verify that the handler limited the request to 64 blocks
    verify(beaconChain).getSealedBeaconBlocks(0UL, MAX_BLOCKS_PER_REQUEST)
  }

  @Test
  fun `handles and returns subset of requested blocks for request with blocks that would exceed the size limit`() {
    handler = BeaconBlocksByRangeHandler(
      beaconChain = beaconChain,
      blockRetrievalStrategy = SizeLimitBlockRetrievalStrategy(sizeLimit = 9000),
    )

    val request = BeaconBlocksByRangeRequest(startBlockNumber = 0UL, count = 10UL)
    val message =
      RequestMessageAdapter(
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          version = Version.V1,
          payload = request,
        ),
      )

    val limitedBlocks =
      (0UL until 10UL).map { i ->
        DataGenerators.randomSealedBeaconBlock(
          number = i,
        )
      }

    var startBlockNumber = 0UL
    whenever(beaconChain.getSealedBeaconBlock(any<ULong>()))
      .thenAnswer {
        limitedBlocks[(startBlockNumber++).toInt()]
      }

    handler.handleIncomingMessage(peer, message, callback)

    val responseCaptor = argumentCaptor<Message<BeaconBlocksByRangeResponse, RpcMessageType>>()
    verify(callback).respondAndCompleteSuccessfully(responseCaptor.capture())

    // The response should return 4 blocks out of 10, given that the compressed serialized size of
    // each block is around 2000 bytes, and we'd set the size limit as 9000 bytes
    val response = responseCaptor.firstValue
    assertThat(response.payload.blocks).hasSize(4)

    // Verify that getSealedBeaconBlock of block number 0 to 5 had been called
    // block number 5 was called but the block was not returned due to over-sized
    (0UL until 5UL).map { i ->
      verify(beaconChain).getSealedBeaconBlock(i)
    }

    // Verify that getSealedBeaconBlock of block number 6 to 10 had never been called
    (6UL until 10UL).map { i ->
      verify(beaconChain, never()).getSealedBeaconBlock(i)
    }
  }

  @Test
  fun `handles request with Rpc exception`() {
    val request = BeaconBlocksByRangeRequest(startBlockNumber = 0UL, count = 1000UL)
    val message =
      RequestMessageAdapter(
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          version = Version.V1,
          payload = request,
        ),
      )

    whenever(beaconChain.getSealedBeaconBlocks(0UL, MAX_BLOCKS_PER_REQUEST))
      .thenAnswer {
        throw RpcException(RpcResponseStatus.RESOURCE_UNAVAILABLE, "getSealedBeaconBlocks exception testing")
      }

    handler.handleIncomingMessage(peer, message, callback)

    verify(callback).completeWithErrorResponse(
      RpcException(
        RpcResponseStatus.RESOURCE_UNAVAILABLE,
        "getSealedBeaconBlocks exception testing",
      ),
    )
  }

  @Test
  fun `handles request with, Missing sealed beacon block, exception`() {
    val request = BeaconBlocksByRangeRequest(startBlockNumber = 0UL, count = 1000UL)
    val message =
      RequestMessageAdapter(
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          version = Version.V1,
          payload = request,
        ),
      )

    whenever(beaconChain.getSealedBeaconBlocks(0UL, MAX_BLOCKS_PER_REQUEST))
      .thenThrow(
        IllegalStateException("Missing sealed beacon block 10"),
      )

    handler.handleIncomingMessage(peer, message, callback)

    verify(callback).completeWithErrorResponse(
      RpcException(
        RpcResponseStatus.RESOURCE_UNAVAILABLE,
        "Handling request failed with IllegalStateException: Missing sealed beacon block 10",
      ),
    )
  }

  @Test
  fun `handles request with exception`() {
    val request = BeaconBlocksByRangeRequest(startBlockNumber = 0UL, count = 1000UL)
    val message =
      RequestMessageAdapter(
        MessageData(
          type = RpcMessageType.BEACON_BLOCKS_BY_RANGE,
          version = Version.V1,
          payload = request,
        ),
      )

    whenever(beaconChain.getSealedBeaconBlocks(0UL, MAX_BLOCKS_PER_REQUEST))
      .thenThrow(
        IllegalStateException("getSealedBeaconBlocks exception testing"),
      )

    handler.handleIncomingMessage(peer, message, callback)

    verify(callback).completeWithUnexpectedError(
      RpcException(
        RpcResponseStatus.SERVER_ERROR_CODE,
        "Handling request failed with IllegalStateException: getSealedBeaconBlocks exception testing",
      ),
    )
  }
}
