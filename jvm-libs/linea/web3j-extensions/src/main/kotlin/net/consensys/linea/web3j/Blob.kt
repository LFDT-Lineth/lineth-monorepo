package net.consensys.linea.web3j

import linea.domain.Constants

fun padBlobForEip4844Submission(blob: ByteArray): ByteArray {
  return ByteArray(Constants.Eip4844BlobSize).apply { blob.copyInto(this) }
}
