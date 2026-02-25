import json

d = {"byte_slice_u512": {"word":"0x11223344","n":[61],"m":[61],"res":"0x22"}}
# Expecting {"byte_slice_u512": {"word":[287454020],"n":[61],"m":[61],"res":[34]}}

# Convert hex string fields to lists of integers
def convert_hex_fields(d):
    inner = next(iter(d.values()))
    for k, v in inner.items():
        if isinstance(v, str) and v.startswith("0x"):
            inner[k] = [int(v, 16)]
    return d

print(json.dumps(convert_hex_fields(d)))





