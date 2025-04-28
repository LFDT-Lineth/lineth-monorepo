package linea.web3j

import linea.domain.Constants
import org.web3j.crypto.Blob

fun padBlobForEip4844Submission(blob: ByteArray): ByteArray {
  return ByteArray(Constants.Eip4844BlobSize).apply { blob.copyInto(this) }
}
fun ByteArray.toWeb3jTxBlob(): Blob = Blob(padBlobForEip4844Submission(this))
fun List<ByteArray>.toWeb3jTxBlob(): List<Blob> = map { it.toWeb3jTxBlob() }
