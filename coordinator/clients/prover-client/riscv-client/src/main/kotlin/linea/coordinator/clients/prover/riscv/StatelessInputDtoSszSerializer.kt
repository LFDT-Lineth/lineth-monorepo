package linea.coordinator.clients.prover.riscv

fun interface StatelessInputDtoSszSerializer {
  fun getStatelessInputDtoSsz(
    statelessInputDto: StatelessInputDto,
  ): String
}
