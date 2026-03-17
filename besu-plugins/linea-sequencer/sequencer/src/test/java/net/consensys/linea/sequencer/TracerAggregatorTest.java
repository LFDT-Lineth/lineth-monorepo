/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer;

import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;

import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.tracing.OperationTracer;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.services.tracer.BlockAwareOperationTracer;
import org.junit.jupiter.api.Test;

class TracerAggregatorTest {

  private static final Address MINING_BENEFICIARY = Address.ZERO;

  @Test
  void traceStartBlockDelegatesToBlockAwareTracers() {
    final BlockAwareOperationTracer blockAwareTracer = mock(BlockAwareOperationTracer.class);
    final WorldView worldView = mock(WorldView.class);
    final BlockHeader blockHeader = mock(BlockHeader.class);
    final BlockBody blockBody = mock(BlockBody.class);

    final TracerAggregator aggregator = TracerAggregator.create(blockAwareTracer);

    aggregator.traceStartBlock(worldView, blockHeader, blockBody, MINING_BENEFICIARY);

    verify(blockAwareTracer).traceStartBlock(worldView, blockHeader, blockBody, MINING_BENEFICIARY);
  }

  @Test
  void traceEndBlockDelegatesToBlockAwareTracers() {
    final BlockAwareOperationTracer blockAwareTracer = mock(BlockAwareOperationTracer.class);
    final BlockHeader blockHeader = mock(BlockHeader.class);
    final BlockBody blockBody = mock(BlockBody.class);

    final TracerAggregator aggregator = TracerAggregator.create(blockAwareTracer);

    aggregator.traceEndBlock(blockHeader, blockBody);

    verify(blockAwareTracer).traceEndBlock(blockHeader, blockBody);
  }

  @Test
  void traceStartBlockWithProcessableHeaderDelegatesToBlockAwareTracers() {
    final BlockAwareOperationTracer blockAwareTracer = mock(BlockAwareOperationTracer.class);
    final WorldView worldView = mock(WorldView.class);
    final ProcessableBlockHeader processableBlockHeader = mock(ProcessableBlockHeader.class);

    final TracerAggregator aggregator = TracerAggregator.create(blockAwareTracer);

    aggregator.traceStartBlock(worldView, processableBlockHeader, MINING_BENEFICIARY);

    verify(blockAwareTracer).traceStartBlock(worldView, processableBlockHeader, MINING_BENEFICIARY);
  }

  @Test
  void traceStartBlockSkipsPlainOperationTracers() {
    final OperationTracer plainTracer = mock(OperationTracer.class);
    final BlockAwareOperationTracer blockAwareTracer = mock(BlockAwareOperationTracer.class);
    final WorldView worldView = mock(WorldView.class);
    final BlockHeader blockHeader = mock(BlockHeader.class);
    final BlockBody blockBody = mock(BlockBody.class);

    final TracerAggregator aggregator = TracerAggregator.create(plainTracer, blockAwareTracer);

    aggregator.traceStartBlock(worldView, blockHeader, blockBody, MINING_BENEFICIARY);

    verifyNoInteractions(plainTracer);
    verify(blockAwareTracer).traceStartBlock(worldView, blockHeader, blockBody, MINING_BENEFICIARY);
  }

  @Test
  void traceEndBlockSkipsPlainOperationTracers() {
    final OperationTracer plainTracer = mock(OperationTracer.class);
    final BlockAwareOperationTracer blockAwareTracer = mock(BlockAwareOperationTracer.class);
    final BlockHeader blockHeader = mock(BlockHeader.class);
    final BlockBody blockBody = mock(BlockBody.class);

    final TracerAggregator aggregator = TracerAggregator.create(plainTracer, blockAwareTracer);

    aggregator.traceEndBlock(blockHeader, blockBody);

    verifyNoInteractions(plainTracer);
    verify(blockAwareTracer).traceEndBlock(blockHeader, blockBody);
  }
}
