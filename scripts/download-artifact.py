"""Download one HTTPS artifact using the host's Python TLS/proxy configuration.

The caller verifies size and SHA-256 after this helper exits. No shell is used.
"""

import argparse
import os
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("url")
    parser.add_argument("destination")
    parser.add_argument("--resume", action="store_true")
    args = parser.parse_args()
    if not args.url.startswith("https://"):
        raise SystemExit("artifact URL must use HTTPS")
    existing = os.path.getsize(args.destination) if args.resume and os.path.exists(args.destination) else 0
    headers = {"User-Agent": "modelctl/0.1"}
    if existing:
        headers["Range"] = f"bytes={existing}-"
    request = urllib.request.Request(args.url, headers=headers)
    with urllib.request.urlopen(request, timeout=60) as response:
        if not response.url.startswith("https://"):
            raise SystemExit("artifact redirect must use HTTPS")
        if existing and response.status != 206:
            response.close()
            existing = 0
            request = urllib.request.Request(args.url, headers={"User-Agent": "modelctl/0.1"})
            response = urllib.request.urlopen(request, timeout=60)
            if not response.url.startswith("https://"):
                raise SystemExit("artifact redirect must use HTTPS")
        with open(args.destination, "ab" if existing else "wb") as output:
            while True:
                chunk = response.read(1024 * 1024)
                if not chunk:
                    break
                output.write(chunk)
                print(output.tell(), flush=True)


if __name__ == "__main__":
    main()
