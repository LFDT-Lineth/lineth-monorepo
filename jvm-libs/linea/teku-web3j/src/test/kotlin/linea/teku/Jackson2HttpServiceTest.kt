/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.teku

import okhttp3.OkHttpClient
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV3
import tech.pegasys.teku.infrastructure.bytes.Bytes20
import tech.pegasys.teku.infrastructure.unsigned.UInt64

/**
 * Reproduces the web3j 6.0.0 regression: its default ObjectMapper (`tools.jackson.databind`, Jackson 3) does not
 * honour Teku's classic Jackson 2 `@JsonSerialize` hex-string serializers, so engine-api requests built from
 * [ForkChoiceStateV1]/[PayloadAttributesV3] were silently mis-serialized as raw internal object fields instead
 * of hex strings (observed in CI as e.g. `"timestamp":{"maxValue":false,"thirtyTwoEth":false,"zero":false}`
 * instead of `"timestamp":"0x..."`), which the execution client then rejected as an invalid request.
 */
class Jackson2HttpServiceTest {
  private val jackson2HttpService = Jackson2HttpService(url = "http://localhost:0", httpClient = OkHttpClient())

  @Test
  fun `serializes engine_forkchoiceUpdatedV3 params as hex strings, not raw object fields`() {
    val forkChoiceState = ForkChoiceStateV1(
      /* headBlockHash = */
      Bytes32.fromHexString("0x" + "11".repeat(32)),
      /* safeBlockHash = */
      Bytes32.fromHexString("0x" + "22".repeat(32)),
      /* finalizedBlockHash = */
      Bytes32.fromHexString("0x" + "33".repeat(32)),
    )
    val payloadAttributes = PayloadAttributesV3(
      /* timestamp = */
      UInt64.valueOf(1783356552L),
      /* prevRandao = */
      Bytes32.fromHexString("0x" + "44".repeat(32)),
      /* suggestedFeeRecipient = */
      Bytes20.fromHexString("0x" + "55".repeat(20)),
      /* withdrawals = */
      emptyList(),
      /* parentBeaconBlockRoot = */
      Bytes32.fromHexString("0x" + "66".repeat(32)),
    )

    @Suppress("UNCHECKED_CAST")
    val responseType = Response::class.java as Class<Response<Any>>
    val request = Request(
      "engine_forkchoiceUpdatedV3",
      listOf(forkChoiceState, payloadAttributes),
      jackson2HttpService,
      responseType,
    )

    val serialized = jackson2HttpService.objectMapper.writeValueAsString(request)

    // none of the buggy internal field names web3j 6.0.0's Jackson 3 mapper leaked should appear
    assertThat(serialized).doesNotContain("maxValue", "thirtyTwoEth", "wrappedBytes")
    assertThat(serialized).contains(""""headBlockHash":"0x${"11".repeat(32)}"""")
    assertThat(serialized).contains(""""safeBlockHash":"0x${"22".repeat(32)}"""")
    assertThat(serialized).contains(""""finalizedBlockHash":"0x${"33".repeat(32)}"""")
    assertThat(serialized).contains(""""timestamp":"0x6a4bdc88"""")
    assertThat(serialized).contains(""""prevRandao":"0x${"44".repeat(32)}"""")
    assertThat(serialized).contains(""""suggestedFeeRecipient":"0x${"55".repeat(20)}"""")
    assertThat(serialized).contains(""""parentBeaconBlockRoot":"0x${"66".repeat(32)}"""")
  }
}
