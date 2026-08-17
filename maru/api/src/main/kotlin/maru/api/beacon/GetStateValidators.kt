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
import linea.kotlin.encodeHex
import maru.api.ChainDataProvider

data class GetStateValidatorsResponse(
  @JsonProperty("execution_optimistic")val executionOptimistic: Boolean,
  @JsonProperty("finalized") val finalized: Boolean,
  @JsonProperty("data") val data: List<ValidatorResponse>,
)

class GetStateValidators(
  val chainDataProvider: ChainDataProvider,
) : Handler {
  override fun handle(ctx: Context) {
    val beaconState = getBeaconState(ctx.pathParam(GetStateValidators.STATE_ID), chainDataProvider)
    val validators =
      beaconState.validators.mapIndexed { index, validator ->
        ValidatorResponse(
          index = index.toString(),
          balance = "",
          status = "active_ongoing",
          validator = Validator(
            pubkey = validator.address.encodeHex(),
            withdrawalCredentials = "0x",
            effectiveBalance = "",
            slashed = false,
            activationEligibilityEpoch = "",
            activationEpoch = "",
            exitEpoch = "",
            withdrawableEpoch = "",
          ),
        )
      }
    val response =
      GetStateValidatorsResponse(
        executionOptimistic = false,
        finalized = false,
        data = validators,
      )
    ctx.status(200).json(response)
  }

  companion object {
    const val ROUTE: String = "/eth/v1/beacon/states/{state_id}/validators"
    const val STATE_ID: String = "state_id"
  }
}
