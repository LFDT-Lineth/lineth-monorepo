/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import java.io.IOException;
import org.hyperledger.besu.ethereum.core.Transaction;

public interface LivenessTxBuilder {
  Transaction buildUptimeTransaction(boolean isUp, long timestamp, long nonce) throws IOException;
}
