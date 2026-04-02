export type Web3SignerTlsConfig = {
  /** Path to PKCS12 client keystore (mTLS client certificate) */
  keyStorePath: string;
  keyStorePassword: string;
  /** Path to PKCS12 truststore (server CA certificate) */
  trustStorePath: string;
  trustStorePassword: string;
};

export type SignerConfig =
  | { type: "private-key"; privateKey: `0x${string}` }
  | {
      type: "web3signer";
      endpoint: string;
      publicKey: `0x${string}`;
      tls?: Web3SignerTlsConfig;
    };
