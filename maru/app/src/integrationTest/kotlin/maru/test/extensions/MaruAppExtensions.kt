/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.extensions

import maru.app.MaruApp
import maru.core.ExecutionPayload

fun MaruApp.headBeaconBlockNumber(): ULong =
  this.beaconChain
    .getLatestBeaconState()
    .beaconBlockHeader.number

fun MaruApp.headElBlock(): ExecutionPayload =
  this.beaconChain
    .getLatestBeaconBlock()
    .beaconBlockBody.executionPayload

fun MaruApp.headElBlockNumber(): ULong = this.headElBlock().blockNumber
