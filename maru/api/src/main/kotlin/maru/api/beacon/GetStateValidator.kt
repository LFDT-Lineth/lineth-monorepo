/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.api.beacon

import com.fasterxml.jackson.annotation.JsonProperty
import io.javalin.http.Context
import io.javalin.http.Handler
import maru.api.ChainDataProvider
import maru.extensions.encodeHex

data class GetStateValidatorResponse(
  @JsonProperty("execution_optimistic") val executionOptimistic: Boolean,
  @JsonProperty("finalized") val finalized: Boolean,
  @JsonProperty("data") val data: ValidatorResponse,
)

data class ValidatorResponse(
  @JsonProperty("index") val index: String,
  @JsonProperty("balance") val balance: String,
  @JsonProperty("status") val status: String,
  @JsonProperty("validator") val validator: Validator,
)

// https://github.com/ethereum/consensus-specs/blob/v1.3.0/specs/phase0/beacon-chain.md#validator
data class Validator(
  @JsonProperty("pubkey") val pubkey: String,
  @JsonProperty("withdrawal_credentials") val withdrawalCredentials: String,
  @JsonProperty("effective_balance") val effectiveBalance: String,
  @JsonProperty("slashed") val slashed: Boolean,
  @JsonProperty("activation_eligibility_epoch") val activationEligibilityEpoch: String,
  @JsonProperty("activation_epoch") val activationEpoch: String,
  @JsonProperty("exit_epoch") val exitEpoch: String,
  @JsonProperty("withdrawable_epoch") val withdrawableEpoch: String,
)

class GetStateValidator(
  val chainDataProvider: ChainDataProvider,
) : Handler {
  override fun handle(ctx: Context) {
    val beaconState = getBeaconState(ctx.pathParam(STATE_ID), chainDataProvider)
    val indexedValidator = getValidator(ctx.pathParam(VALIDATOR_ID), beaconState)
    val validator =
      ValidatorResponse(
        index = indexedValidator.index.toString(),
        balance = "",
        status = "active_ongoing",
        validator = Validator(
          pubkey = indexedValidator.value.address.encodeHex(),
          withdrawalCredentials = "0x",
          effectiveBalance = "",
          slashed = false,
          activationEligibilityEpoch = "",
          activationEpoch = "",
          exitEpoch = "",
          withdrawableEpoch = "",
        ),
      )

    val response =
      GetStateValidatorResponse(
        executionOptimistic = false,
        finalized = false,
        data = validator,
      )
    ctx.status(200).json(response)
  }

  companion object {
    const val STATE_ID: String = "state_id"
    const val VALIDATOR_ID: String = "validator_id"
    const val ROUTE: String = "/eth/v1/beacon/states/{$STATE_ID}/validators/{$VALIDATOR_ID}"
  }
}
