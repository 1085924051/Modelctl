"""Download one HTTPS artifact with proxy support and resumable retries.

The destination is a partial file while the download is in progress. Network
failures leave that file in place so a later invocation can continue with a
Range request. The caller verifies the final size and SHA-256.
"""

import argparse
import os
import re
import socket
import sys
import time
import urllib.error
import urllib.request


RETRYABLE_STATUSES = {408, 429, 500, 502, 503, 504}


def configure_proxy_environment():
    """Prefer Modelctl-specific proxy variables without exposing credentials."""
    mapping = {
        "MODELCTL_HTTP_PROXY": "HTTP_PROXY",
        "MODELCTL_HTTPS_PROXY": "HTTPS_PROXY",
        "MODELCTL_NO_PROXY": "NO_PROXY",
    }
    for source, target in mapping.items():
        value = os.environ.get(source)
        if value:
            os.environ[target] = value
            os.environ[target.lower()] = value
    for name in ("HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"):
        value = os.environ.get(name) or os.environ.get(name.lower())
        if value:
            os.environ[name] = value
            os.environ[name.lower()] = value


def emit_error(code, message):
    safe = re.sub(r"(?i)(https?://)([^/@\s]+):([^/@\s]+)@", r"\1***:***@", str(message))
    print(f"MODELCTL_ERROR {code} {safe}", file=sys.stderr, flush=True)


def retry_after(response):
    value = response.headers.get("Retry-After") if response else None
    if not value:
        return None
    try:
        return max(0.0, min(60.0, float(value)))
    except ValueError:
        return None


def classify_exception(error):
    text = str(error).lower()
    if isinstance(error, (socket.timeout, TimeoutError)) or "timed out" in text or "timeout" in text:
        return "DOWNLOAD_TIMEOUT"
    if "proxy" in text:
        return "DOWNLOAD_PROXY_ERROR"
    if isinstance(error, urllib.error.HTTPError):
        return "DOWNLOAD_HTTP_ERROR"
    return "DOWNLOAD_NETWORK_ERROR"


def open_url(url, existing, timeout):
    headers = {"User-Agent": "modelctl/0.1"}
    if existing:
        headers["Range"] = f"bytes={existing}-"
    request = urllib.request.Request(url, headers=headers)
    return urllib.request.urlopen(request, timeout=timeout)


def set_read_timeout(response, timeout):
    """Use a separate stall timeout when urllib exposes the underlying socket."""
    socket_obj = getattr(getattr(getattr(response, "fp", None), "raw", None), "_sock", None)
    if socket_obj is not None:
        socket_obj.settimeout(timeout)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("url")
    parser.add_argument("destination")
    parser.add_argument("--resume", action="store_true")
    parser.add_argument("--expected-size", type=int, default=0)
    parser.add_argument("--retries", type=int, default=6)
    parser.add_argument("--connect-timeout", type=float, default=30.0)
    parser.add_argument("--read-timeout", type=float, default=30.0)
    args = parser.parse_args()
    if not args.url.startswith("https://"):
        emit_error("DOWNLOAD_HTTP_ERROR", "artifact URL must use HTTPS")
        return 2
    if args.expected_size < 0:
        emit_error("DOWNLOAD_HTTP_ERROR", "expected size must be non-negative")
        return 2
    configure_proxy_environment()
    os.makedirs(os.path.dirname(os.path.abspath(args.destination)), exist_ok=True)

    max_attempts = max(1, args.retries + 1)
    for attempt in range(max_attempts):
        existing = os.path.getsize(args.destination) if args.resume and os.path.exists(args.destination) else 0
        if args.expected_size and existing > args.expected_size:
            emit_error("ARTIFACT_SIZE_MISMATCH", f"partial file exceeds expected size ({existing} > {args.expected_size})")
            return 2
        response = None
        try:
            response = open_url(args.url, existing, args.connect_timeout)
            set_read_timeout(response, args.read_timeout)
            if not str(response.url).startswith("https://"):
                raise urllib.error.URLError("artifact redirect must use HTTPS")
            if existing and getattr(response, "status", None) != 206:
                response.close()
                existing = 0
                response = open_url(args.url, 0, args.connect_timeout)
                set_read_timeout(response, args.read_timeout)
                if not str(response.url).startswith("https://"):
                    raise urllib.error.URLError("artifact redirect must use HTTPS")
            mode = "ab" if existing else "wb"
            with open(args.destination, mode) as output:
                while True:
                    chunk = response.read(1024 * 1024)
                    if not chunk:
                        break
                    output.write(chunk)
                    output.flush()
                    print(output.tell(), flush=True)
            response.close()
            final_size = os.path.getsize(args.destination)
            if args.expected_size and final_size != args.expected_size:
                raise urllib.error.URLError(f"incomplete response: {final_size}/{args.expected_size} bytes")
            return 0
        except urllib.error.HTTPError as error:
            if response is not None:
                response.close()
            status = error.code
            if status not in RETRYABLE_STATUSES:
                emit_error("DOWNLOAD_HTTP_ERROR", f"HTTP {status}: {error.reason}")
                return 2
            delay = retry_after(error) or min(30.0, 2 ** attempt)
            if attempt + 1 >= max_attempts:
                emit_error("DOWNLOAD_HTTP_ERROR", f"HTTP {status} after {attempt + 1} attempts")
                return 2
            time.sleep(delay)
        except (urllib.error.URLError, socket.timeout, TimeoutError, OSError) as error:
            if response is not None:
                response.close()
            if attempt + 1 >= max_attempts:
                emit_error(classify_exception(error), str(error))
                return 2
            time.sleep(min(30.0, 2 ** attempt))
    emit_error("DOWNLOAD_NETWORK_ERROR", "download exhausted retries")
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
