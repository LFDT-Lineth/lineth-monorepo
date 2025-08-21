import axios from "axios";
import { ethers, TransactionLike } from "ethers";
import { Agent } from "https";
import forge from "node-forge";
import { readFileSync } from "fs";
import path from "path";

export function convertToPem(p12base64: string | forge.util.ByteStringBuffer, password: string) {
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

export function getWeb3SignerHttpsAgent(
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

export async function getWeb3SignerSignature(
  web3SignerUrl: string,
  web3SignerPublicKey: string,
  transaction: TransactionLike,
  agent?: Agent,
): Promise<string> {
  try {
    const { data } = await axios.post(
      `${web3SignerUrl}/api/v1/eth1/sign/${web3SignerPublicKey}`,
      {
        data: ethers.Transaction.from(transaction).unsignedSerialized,
      },
      { httpsAgent: agent },
    );
    return data;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (error: any) {
    throw new Error(`Web3SignerError: ${JSON.stringify(error.message)}`);
  }
}

export async function estimateTransactionGas(
  provider: ethers.JsonRpcProvider,
  transaction: ethers.TransactionRequest,
): Promise<bigint> {
  try {
    return await provider.estimateGas(transaction);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (error: any) {
    throw new Error(`GasEstimationError: ${JSON.stringify(error.message)}`);
  }
}

export async function executeTransaction(
  provider: ethers.JsonRpcProvider,
  transaction: TransactionLike,
): Promise<ethers.TransactionReceipt | null> {
  try {
    const tx = await provider.broadcastTransaction(ethers.Transaction.from(transaction).serialized);
    return await tx.wait();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  } catch (error: any) {
    throw new Error(`TransactionError: ${JSON.stringify(error.message)}`);
  }
}
