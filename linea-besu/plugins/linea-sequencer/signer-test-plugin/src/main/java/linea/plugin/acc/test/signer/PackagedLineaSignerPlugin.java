/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.plugin.acc.test.signer;

import java.util.HexFormat;
import linea.crypto.Secp256k1Signature;
import linea.crypto.Signer;
import linea.signing.NamedSignerProviderService;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import tech.pegasys.teku.infrastructure.async.SafeFuture;

/** Acceptance-test provider loaded from a plugin JAR rather than the Besu application classpath. */
public class PackagedLineaSignerPlugin implements BesuPlugin {
  public static final String PLUGIN_NAME = "PackagedLineaSignerPlugin";
  public static final String SIGNER_NAME = "liveness-test";
  public static final byte[] PUBLIC_KEY =
      HexFormat.of()
          .parseHex(
              "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
                  + "483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8");

  @Override
  public void register(final ServiceManager serviceManager) {
    serviceManager.addService(
        NamedSignerProviderService.class, new TestNamedSignerProviderService());
  }

  @Override
  public void start() {}

  @Override
  public void stop() {}

  private static class TestNamedSignerProviderService implements NamedSignerProviderService {
    @Override
    public Signer<Secp256k1Signature> create(final String name) {
      if (!SIGNER_NAME.equals(name)) {
        throw new IllegalArgumentException("Unknown test signer: " + name);
      }
      return new TestSigner();
    }
  }

  private static class TestSigner implements Signer<Secp256k1Signature> {
    @Override
    public byte[] publicKey() {
      return PUBLIC_KEY.clone();
    }

    @Override
    public SafeFuture<Secp256k1Signature> sign(final byte[] bytes) {
      return SafeFuture.failedFuture(
          new UnsupportedOperationException("The service-boundary test does not sign payloads"));
    }
  }
}
