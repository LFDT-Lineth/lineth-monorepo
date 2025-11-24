import axios from "axios";
import { Agent } from "https";
import { Address, Hex, serializeTransaction, TransactionSerializable } from "viem";
import { IContractSignerClient } from "../core/client/IContractSignerClient";
import { publicKeyToAddress } from "viem/accounts";
import forge from "node-forge";
import { readFileSync } from "fs";
import path from "path";
import { ILogger } from "../logging/ILogger";
import { getModuleDir } from "../utils/file";

/**
 * Adapter for Web3Signer service that provides contract signing functionality via remote API.
 * Uses HTTPS client authentication with P12 keystore and trusted store certificates.
 */
export class Web3SignerClientAdapter implements IContractSignerClient {
  private readonly agent: Agent;
  /**
   * Creates a new Web3SignerClientAdapter instance.
   *
   * @param {ILogger} logger - The logger instance for logging signing operations.
   * @param {string} web3SignerUrl - The base URL of the Web3Signer service.
   * @param {Hex} web3SignerPublicKey - The public key in hex format for the signing key.
   * @param {string} web3SignerKeystorePath - Path to the P12 keystore file for client authentication.
   * @param {string} web3SignerKeystorePassphrase - Passphrase for the keystore file.
   * @param {string} web3SignerTrustedStorePath - Path to the P12 trusted store file for CA certificate.
   * @param {string} web3SignerTrustedStorePassphrase - Passphrase for the trusted store file.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly web3SignerUrl: string,
    private readonly web3SignerPublicKey: Hex,
    web3SignerKeystorePath: string,
    web3SignerKeystorePassphrase: string,
    web3SignerTrustedStorePath: string,
    web3SignerTrustedStorePassphrase: string,
  ) {
    this.logger.info("Initialising HTTPS agent");
    this.agent = this.getHttpsAgent(
      web3SignerKeystorePath,
      web3SignerKeystorePassphrase,
      web3SignerTrustedStorePath,
      web3SignerTrustedStorePassphrase,
    );
  }

  /**
   * Signs a transaction by sending it to the remote Web3Signer service.
   * The transaction is serialized and sent via HTTPS POST request with client certificate authentication.
   *
   * @param {TransactionSerializable} tx - The transaction to sign.
   * @returns {Promise<Hex>} The signature as a hex string returned from the Web3Signer service.
   */
  async sign(tx: TransactionSerializable): Promise<Hex> {
    this.logger.debug("Signing transaction via remote Web3Signer");
    const { data } = await axios.post(
      `${this.web3SignerUrl}/api/v1/eth1/sign/${this.web3SignerPublicKey}`,
      {
        data: serializeTransaction(tx),
      },
      { httpsAgent: this.agent },
    );
    this.logger.debug(`Signing successful signature=${data}`);
    return data;
  }

  /**
   * Gets the Ethereum address associated with the Web3Signer public key.
   *
   * @returns {Address} The Ethereum address derived from the public key.
   */
  getAddress(): Address {
    // Seems that Viem `publicKeyToAddress` expects the secp256k1 pubkey with the 0x04 format identifier byte - `0x04 || <32-byte X> || <32-byte Y>`
    // However Web3Signer does not accept the 0x04 format identifier byte, and instead expects either:
    // - `<32-byte X> || <32-byte Y>` (no prefix)
    // - `0x<32-byte X> || <32-byte Y>` (with hex prefix)
    const pubkeyWithoutPrefix = this.web3SignerPublicKey.startsWith("0x")
      ? this.web3SignerPublicKey.slice(2)
      : this.web3SignerPublicKey;
    const uncompressedPubkeyWithFormatByte = `0x04${pubkeyWithoutPrefix}` as const;
    return publicKeyToAddress(uncompressedPubkeyWithFormatByte);
  }

  /**
   * Converts a P12 certificate to PEM format.
   *
   * @param {string | forge.util.ByteStringBuffer} p12base64 - The P12 certificate data in base64 or ByteStringBuffer format.
   * @param {string} password - The password to decrypt the P12 certificate.
   * @returns {{ pemCertificate: string }} An object containing the PEM-formatted certificate.
   */
  private convertToPem(p12base64: string | forge.util.ByteStringBuffer, password: string) {
    const p12Asn1 = forge.asn1.fromDer(p12base64);
    const p12 = forge.pkcs12.pkcs12FromAsn1(p12Asn1, false, password);
    return this.getCertificateFromP12(p12);
  }

  /**
   * Extracts a PEM certificate from a PKCS12 object.
   *
   * @param {forge.pkcs12.Pkcs12Pfx} p12 - The PKCS12 object containing the certificate.
   * @returns {{ pemCertificate: string }} An object containing the PEM-formatted certificate.
   * @throws {Error} If the certificate is not found in the P12 object.
   */
  private getCertificateFromP12(p12: forge.pkcs12.Pkcs12Pfx) {
    const certData = p12.getBags({ bagType: forge.pki.oids.certBag });
    const certificate = certData[forge.pki.oids.certBag]?.[0];
    if (!certificate?.cert) {
      throw new Error("Certificate not found in P12");
    }

    const pemCertificate = forge.pki.certificateToPem(certificate.cert);
    return { pemCertificate };
  }

  /**
   * Creates an HTTPS agent configured with client certificate authentication.
   * Loads the keystore (client certificate) and trusted store (CA certificate) from P12 files.
   *
   * @param {string} keystorePath - Path to the P12 keystore file for client authentication.
   * @param {string} keystorePassphrase - Passphrase for the keystore file.
   * @param {string} trustedStorePath - Path to the P12 trusted store file for CA certificate.
   * @param {string} trustedStorePassphrase - Passphrase for the trusted store file.
   * @returns {Agent} An HTTPS agent configured with the client and CA certificates.
   */
  private getHttpsAgent(
    keystorePath: string,
    keystorePassphrase: string,
    trustedStorePath: string,
    trustedStorePassphrase: string,
  ): Agent {
    const moduleDir = getModuleDir();
    const trustedStoreFile = readFileSync(path.resolve(moduleDir, trustedStorePath), { encoding: "binary" });
    this.logger.debug("Loading trusted store certificate");

    const { pemCertificate } = this.convertToPem(trustedStoreFile, trustedStorePassphrase);

    return new Agent({
      pfx: readFileSync(path.resolve(moduleDir, keystorePath)),
      passphrase: keystorePassphrase,
      ca: pemCertificate,
    });
  }
}
