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


@pytest.mark.parametrize("payload", [b"", b"\x01", bytes(range(32)), bytes(range(64)), bytes(BLOB_PAYLOAD_CAPACITY)])
def test_blob_matches_go_packalign_254_bit_order(payload: bytes) -> None:
    # PackAlign reads big-endian bits, writes each 254-bit word in a 32-byte
    # big-endian slot (two leading zero bits), then pads the physical blob.
    stream = payload + b"\xff"
    expected = bytearray(BLOB_BYTES_LENGTH)
    for i in range((len(stream) * 8 + 253) // 254):
        word = 0
        for bit in range(i * 254, min((i + 1) * 254, len(stream) * 8)):
            word = (word << 1) | ((stream[bit // 8] >> (7 - bit % 8)) & 1)
        word <<= max(0, (i + 1) * 254 - len(stream) * 8)
        expected[i * 32:(i + 1) * 32] = word.to_bytes(32, "big")
    assert pack_blob_payload(payload) == bytes(expected)
