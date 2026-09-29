import hashlib

import pytest

from rollup_spec.rollup import (
    BLOB_BYTES_LENGTH,
    BLOB_PAYLOAD_CAPACITY,
    pack_blob_payload,
    unpack_blob_payload,
)


@pytest.mark.parametrize("payload", [b"", b"\x00\xff\x00", bytes(range(256)) * 507, bytes(BLOB_PAYLOAD_CAPACITY)])
def test_blob_roundtrip_and_physical_size(payload: bytes) -> None:
    blob = pack_blob_payload(payload)
    assert len(blob) == BLOB_BYTES_LENGTH
    assert unpack_blob_payload(blob) == payload
    assert all(blob[i] < 64 for i in range(0, len(blob), 32))


def test_blob_rejects_oversized_payload() -> None:
    with pytest.raises(ValueError, match="capacity"):
        pack_blob_payload(bytes(BLOB_PAYLOAD_CAPACITY + 1))


def test_blob_rejects_invalid_field_element() -> None:
    with pytest.raises(ValueError, match="254 bits"):
        unpack_blob_payload(b"\x40" + bytes(BLOB_BYTES_LENGTH - 1))


def test_blob_rejects_missing_terminal() -> None:
    with pytest.raises(ValueError, match="terminal"):
        unpack_blob_payload(bytes(BLOB_BYTES_LENGTH))


@pytest.mark.parametrize("length, expected_sha256", [
    (0, "700a187d18ec30a349376803b8bd345a7b1cdc38c8559073abe0bb0965e7529e"),
    (1, "dfa0b02e8ee96f7641ea6fce3f608695c72be3053d5a7fda65dc7e53af4068ee"),
    (31, "3cb7b769bf97f2610d6b05a643ca46fd2bb2bba89acc35a7522f90666cd2aa29"),
    (32, "c943b33404d442d84e6504143315c9f455e222fa51d6b63bd41a93d65654dc2e"),
    (64, "204f88ea463853c8a38263973e985c44c9a62b1ac849517619da62589bf8e665"),
    (130047, "01f55680f4e3d70d610abd6a03d6568f012a53df3cf8a6498df07d4b6c0609fb"),
])
def test_blob_matches_go_packalign_physical_blob_vectors(length: int, expected_sha256: str) -> None:
    payload = bytes(i % 256 for i in range(length))
    assert hashlib.sha256(pack_blob_payload(payload)).hexdigest() == expected_sha256
