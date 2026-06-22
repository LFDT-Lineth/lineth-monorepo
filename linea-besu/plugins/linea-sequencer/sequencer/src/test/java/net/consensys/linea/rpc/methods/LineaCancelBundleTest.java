/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.rpc.methods;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.util.UUID;
import net.consensys.linea.bundles.LineaLimitedBundlePool;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class LineaCancelBundleTest {

  private LineaCancelBundle lineaCancelBundle;
  private LineaLimitedBundlePool bundlePool;
  private PluginRpcRequest request;

  @BeforeEach
  void setup() {
    bundlePool = mock(LineaLimitedBundlePool.class);
    request = mock(PluginRpcRequest.class);
    lineaCancelBundle = new LineaCancelBundle().init(bundlePool);
  }

  @Test
  void testExecute_ValidUUID_RemovesBundle() {
    // Mock UUID input
    UUID replacementUUID = UUID.randomUUID();
    when(request.getParams()).thenReturn(new Object[] {replacementUUID});
    when(bundlePool.remove(replacementUUID)).thenReturn(true); // Simulate successful removal

    // Execute method
    boolean result = lineaCancelBundle.execute(request);

    // Verify behavior
    assertTrue(result, "Bundle should be successfully removed");
    verify(bundlePool).remove(replacementUUID); // Ensure remove() was called
  }

  @Test
  void testExecute_InvalidParams_ThrowsException() {
    // Mock invalid parameters (not a UUID)
    when(request.getParams()).thenReturn(new Object[] {"invalid_uuid"});

    Exception exception =
        assertThrows(
            PluginRpcEndpointException.class,
            () -> {
              lineaCancelBundle.execute(request);
            });

    assertTrue(exception.getMessage().contains("malformed linea_cancelBundle json param"));
  }

  @Test
  void testExecute_BundleNotFound_ReturnsFalse() {
    // Mock a valid UUID but simulate that the bundle doesn't exist
    UUID replacementUUID = UUID.randomUUID();
    when(request.getParams()).thenReturn(new Object[] {replacementUUID});
    when(bundlePool.remove(replacementUUID)).thenReturn(false); // Simulate bundle not found

    boolean result = lineaCancelBundle.execute(request);

    assertFalse(result, "Bundle should not be found in the pool");
  }
}
