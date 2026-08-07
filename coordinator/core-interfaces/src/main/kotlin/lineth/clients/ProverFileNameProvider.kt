package lineth.clients

import lineth.domain.ProofIndex

interface ProverFileNameProvider<TProofIndex : ProofIndex> {
  fun getFileName(proofIndex: TProofIndex): String
}
