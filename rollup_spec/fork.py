"""
The single fork binding for the spec: this spec targets one fork, Amsterdam.

`fork.py` is the only module that names the fork — it re-exports the
execution-specs Amsterdam package and defines the fork identity carried on the
wire. Supporting another fork is a deliberate edit here.

The SSZ stateless input encodes the fork as `PROTOCOL_FORKS.index(fork)`
(execution-specs `stateless_ssz.py`). That value is *derived* from the
`ProtocolFork` order — pinned by the execution-specs version in requirements.txt
— not hardcoded; Amsterdam is 20 at the pinned commit.

`ProtocolFork` is copied verbatim below because the pinned branch doesn't package
`execution_engine` (so `ethereum.forks.amsterdam.stateless` can't be imported);
the canonical import is attempted first, this copy is the fallback. Re-sync when
bumping the pin. Source: ethereum/execution-specs@a456712e04
(`forks/amsterdam/stateless.py`, `stateless_ssz.py`), pruned by PR #2926.

Downstream gap: a consumer pinned to a pre-prune index reads Amsterdam as 24, not
20. For example, Zesu is still on tests-zkevm@v0.4.1 (pre-prune), so the spec and
that build disagree on the wire until it re-syncs. The spec follows
execution-specs (which such engines mirror), so 20 is intended.
"""

from enum import StrEnum

# Single place that names the active fork: other spec modules import these
# fork-specific types from here (re-exported via __all__) instead of importing
# `ethereum.forks.amsterdam` directly.
from ethereum.forks.amsterdam import vm
from ethereum.forks.amsterdam.blocks import Block, Header, Log, Withdrawal
from ethereum.forks.amsterdam.transactions import (
    AccessListTransaction,
    BlobTransaction,
    FeeMarketTransaction,
    LegacyTransaction,
    SetCodeTransaction,
    Transaction,
    decode_transaction,
    recover_sender,
)
from ethereum.forks.amsterdam.fork import (
    BlockChain,
    apply_body,
    get_last_256_block_hashes,
)
from ethereum.forks.amsterdam.bloom import logs_bloom
from ethereum.forks.amsterdam.block_access_lists import (
    BlockAccessListBuilder,
    hash_block_access_list,
)
from ethereum.forks.amsterdam.vm.gas import calculate_total_blob_gas


try:
    # Preferred: the canonical enum straight from execution-specs.
    from ethereum.forks.amsterdam.stateless import ProtocolFork  # type: ignore
except ImportError:
    # Fallback: verbatim copy (see "SOURCE / RE-SYNC" above). Active because the
    # pinned branch does not package `execution_engine`, which `stateless.py`
    # imports at module load.
    class ProtocolFork(StrEnum):
        """
        Semantic execution-layer fork names understood by stateless inputs.

        Order is significant: it defines the SSZ `active_fork` index via
        `PROTOCOL_FORKS.index(...)`.
        """

        Frontier = "Frontier"
        Homestead = "Homestead"
        DAOFork = "DAOFork"
        TangerineWhistle = "TangerineWhistle"
        SpuriousDragon = "SpuriousDragon"
        Byzantium = "Byzantium"
        StPetersburg = "StPetersburg"
        Istanbul = "Istanbul"
        MuirGlacier = "MuirGlacier"
        Berlin = "Berlin"
        London = "London"
        ArrowGlacier = "ArrowGlacier"
        GrayGlacier = "GrayGlacier"
        Paris = "Paris"
        Shanghai = "Shanghai"
        Cancun = "Cancun"
        Prague = "Prague"
        Osaka = "Osaka"
        BPO1 = "BPO1"
        BPO2 = "BPO2"
        Amsterdam = "Amsterdam"


# Mirrors execution-specs `stateless_ssz.py`: PROTOCOL_FORKS = tuple(ProtocolFork);
# the SSZ enum value is the index into this tuple.
PROTOCOL_FORKS = tuple(ProtocolFork)

# The one fork this spec supports.
ACTIVE_FORK = ProtocolFork.Amsterdam

# DERIVED from the pinned ProtocolFork order (not hardcoded). 20 at the pinned
# commit; re-syncing the enum updates this automatically.
ACTIVE_FORK_SSZ_INDEX = PROTOCOL_FORKS.index(ACTIVE_FORK)


# Public surface: the fork identity plus the active-fork types re-exported for
# other spec modules to import from here.
__all__ = [
    "ProtocolFork",
    "PROTOCOL_FORKS",
    "ACTIVE_FORK",
    "ACTIVE_FORK_SSZ_INDEX",
    "UnsupportedForkError",
    "require_active_fork",
    "vm",
    "Block",
    "Header",
    "Log",
    "Withdrawal",
    "AccessListTransaction",
    "BlobTransaction",
    "FeeMarketTransaction",
    "LegacyTransaction",
    "SetCodeTransaction",
    "Transaction",
    "decode_transaction",
    "recover_sender",
    "BlockChain",
    "apply_body",
    "get_last_256_block_hashes",
    "logs_bloom",
    "BlockAccessListBuilder",
    "hash_block_access_list",
    "calculate_total_blob_gas",
]


class UnsupportedForkError(ValueError):
    """A stateless input declared a fork other than the one this spec supports."""


def require_active_fork(index: int) -> ProtocolFork:
    """
    Validate a decoded SSZ `active_fork` index against the one fork this spec
    supports (Amsterdam), returning it on success.

    This spec is single-fork by design; supporting another fork is a deliberate
    change here, not a runtime branch.
    """
    if index != ACTIVE_FORK_SSZ_INDEX:
        seen = (
            PROTOCOL_FORKS[index].value
            if 0 <= index < len(PROTOCOL_FORKS)
            else f"<out-of-range:{index}>"
        )
        raise UnsupportedForkError(
            f"stateless input declares fork index {index} ({seen}); "
            f"this spec supports only {ACTIVE_FORK.value} (index {ACTIVE_FORK_SSZ_INDEX})"
        )
    return ACTIVE_FORK
