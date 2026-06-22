import { readFileSync } from "fs";
import { Agent } from "https";
import forge from "node-forge";
import path from "path";

function convertToPem(p12base64: string | forge.util.ByteStringBuffer, password: string) {
  const p12Asn1 = forge.asn1.fromDer(p12base64);
  const p12 = forge.pkcs12.pkcs12FromAsn1(p12Asn1, false, password);

  return getCertificateFromP12(p12);
}

function getCertificateFromP12(p12: forge.pkcs12.Pkcs12Pfx) {
  const certData = p12.getBags({ bagType: forge.pki.oids.certBag });
  const certificate = certData[forge.pki.oids.certBag]?.[0];

  if (!certificate?.cert) {
    throw new Error("Certificate not found in P12");
  }

  const pemCertificate = forge.pki.certificateToPem(certificate.cert);
  return { pemCertificate };
}

export function buildHttpsAgent(
  keystorePath: string,
  keystorePassphrase: string,
  trustedStorePath: string,
  trustedStorePassphrase: string,
): Agent {
  const trustedStoreFile = readFileSync(path.resolve(import.meta.dirname, trustedStorePath), { encoding: "binary" });

  const { pemCertificate } = convertToPem(trustedStoreFile, trustedStorePassphrase);

  return new Agent({
    pfx: readFileSync(path.resolve(import.meta.dirname, keystorePath)),
    passphrase: keystorePassphrase,
    ca: pemCertificate,
  });
}
