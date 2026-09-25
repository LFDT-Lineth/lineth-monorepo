package linea.clients

import linea.domain.ProofIndex

interface ProverFileNameProvider<TProofIndex : ProofIndex> : ProverFileNameStartBlockNumberProvider {
  fun getFileName(proofIndex: TProofIndex): String
}

interface ProverFileNameStartBlockNumberProvider {
  fun getFileNameStartBlockNumber(fileName: String): Long {
    return fileName.substringBefore('-').toULong().toLong()
  }
}
