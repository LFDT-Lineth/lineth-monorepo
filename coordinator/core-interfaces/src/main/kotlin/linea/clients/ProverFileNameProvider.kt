package linea.clients

import linea.domain.ProofIndex

interface ProverFileNameProvider<TProofIndex : ProofIndex> {
  fun getFileName(proofIndex: TProofIndex): String
}
