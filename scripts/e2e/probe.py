#!/usr/bin/env python3
"""Protocol probe for the mesh scenarios (QA-607, #607). Standard library only.

    probe.py x224 HOST:PORT   exit 0 on an RDP X.224 Connection Confirm
    probe.py ssh HOST:PORT    exit 0 on an SSH-2.0 banner

Exit 1 is any other outcome (refused, closed, silent past the deadline, wrong bytes), so a case
fails fast instead of hanging; exit 2 is a usage error.
"""
import socket
import sys

DEADLINE = 2.0
# TPKT (version 3, length 11) carrying an X.224 Connection Request with no cookie.
X224_REQUEST = bytes.fromhex("0300000b06e00000000000")


def exchange(addr, request):
    host, _, port = addr.rpartition(":")
    with socket.create_connection((host, int(port)), timeout=DEADLINE) as s:
        s.settimeout(DEADLINE)
        if request:
            s.sendall(request)
        return s.recv(64)


def main(argv):
    if len(argv) != 3 or argv[1] not in ("x224", "ssh"):
        print(__doc__, file=sys.stderr)
        return 2
    try:
        reply = exchange(argv[2], X224_REQUEST if argv[1] == "x224" else b"")
    except (OSError, ValueError) as err:
        print(f"probe {argv[1]} {argv[2]}: {err}", file=sys.stderr)
        return 1
    # Byte 0 is the TPKT version, byte 5 the X.224 type: 0xD0 is a Connection Confirm.
    ok = reply[:1] == b"\x03" and reply[5:6] == b"\xd0" if argv[1] == "x224" else reply.startswith(b"SSH-2.0-")
    print(f"probe {argv[1]} {argv[2]}: {'ok' if ok else 'unexpected reply ' + reply[:16].hex()}")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
