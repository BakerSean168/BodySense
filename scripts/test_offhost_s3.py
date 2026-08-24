#!/usr/bin/env python3
"""Hermetic tests for offhost-s3.py (stdlib only).

Runs without docker, without botocore and without any object store:

  1. AWS Signature Version 4 known-answer vector taken from the AWS
     documentation, independently cross-checked against botocore during
     development.  The expected signature is hard-coded so the test is
     deterministic and offline.
  2. A fake S3 HTTP server built on the standard library.  It re-implements
     SigV4 *verification* from the raw wire request (method, path, query and
     received headers) so it proves that what the client sends is a valid
     signature computed over exactly the bytes the server received.  It also
     simulates ListObjectsV2 pagination and error responses.

Run directly:   python3 scripts/test_offhost_s3.py
Run via unittest: python3 -m unittest scripts.test_offhost_s3 (with scripts on path)
"""

import hashlib
import hmac
import http.server
import importlib.util
import io
import json
import os
import re
import socket
import subprocess
import sys
import threading
import unittest
import importlib.util
import urllib.parse
import xml.etree.ElementTree as ET
from datetime import datetime, timezone

_SCRIPTS_DIR = os.path.dirname(os.path.abspath(__file__))
_spec = importlib.util.spec_from_file_location("offhost_s3", os.path.join(_SCRIPTS_DIR, "offhost-s3.py"))
offhost_s3 = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(offhost_s3)

# ---------------------------------------------------------------------------
# SigV4 known-answer vector (AWS documentation "Signature Calculations for the
# Authorization Header: Transferring Payload in a Single Chunk").  The golden
# signature was cross-checked against botocore 1.43 (S3SigV4Auth) in the
# sandbox and matches bit-for-bit.
# ---------------------------------------------------------------------------
VECTOR = {
    "access_key": "AKIAIOSFODNN7EXAMPLE",
    "secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
    "region": "us-east-1",
    "service": "s3",
    "amz_date": "20130524T000000Z",
    "date_stamp": "20130524",
    "method": "GET",
    "canonical_uri": "/test.txt",
    "canonical_query": "",
    "payload_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "headers": {
        "host": "examplebucket.s3.amazonaws.com",
        "range": "bytes=0-9",
        "x-amz-content-sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        "x-amz-date": "20130524T000000Z",
    },
    "expected_signature": "f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41",
}


def _canonical_headers_for(headers, order):
    return "".join("%s:%s\n" % (name, headers[name]) for name in order)


def _compute_signature(signing_key, string_to_sign):
    return hmac.new(signing_key, string_to_sign.encode("utf-8"), hashlib.sha256).hexdigest()


class SigV4KnownAnswerTests(unittest.TestCase):
    """Verify the client's signing primitive against the AWS documented vector."""

    def test_known_answer_signature(self):
        v = VECTOR
        signed_headers_list = sorted(v["headers"])  # host;range
        signed_headers = ";".join(signed_headers_list)
        canonical_headers = _canonical_headers_for(v["headers"], signed_headers_list)
        canonical_request = "\n".join(
            [
                v["method"],
                v["canonical_uri"],
                v["canonical_query"],
                canonical_headers,
                signed_headers,
                v["payload_hash"],
            ]
        )
        scope = "%s/%s/%s/aws4_request" % (v["date_stamp"], v["region"], v["service"])
        string_to_sign = "\n".join(
            [
                "AWS4-HMAC-SHA256",
                v["amz_date"],
                scope,
                hashlib.sha256(canonical_request.encode("utf-8")).hexdigest(),
            ]
        )
        key = offhost_s3._signing_key(v["secret_key"], v["date_stamp"], v["region"])
        signature = _compute_signature(key, string_to_sign)
        self.assertEqual(signature, v["expected_signature"])

    def test_signing_key_is_deterministic(self):
        key = offhost_s3._signing_key(VECTOR["secret_key"], "20130524", "us-east-1")
        self.assertEqual(len(key), 32)
        self.assertEqual(
            key,
            offhost_s3._signing_key(VECTOR["secret_key"], "20130524", "us-east-1"),
        )

    def test_uri_encode_path(self):
        self.assertEqual(offhost_s3._uri_encode_path("/a/b c/d"), "/a/b%20c/d")
        self.assertEqual(offhost_s3._uri_encode_path("/k/u/9"), "/k/u/9")
        self.assertEqual(offhost_s3._uri_encode_path("/a+b/%.x"), "/a%2Bb/%25.x")
        self.assertEqual(offhost_s3._uri_encode_path(""), "")

    def test_canonical_query_sorts_and_encodes(self):
        query = {"b": "two", "a": "one", "x": "a b"}
        self.assertEqual(
            offhost_s3._canonical_query(query), "a=one&b=two&x=a%20b"
        )

    def test_missing_credentials_raise(self):
        client = offhost_s3.S3Client("https://example.test", "b", region="r")
        path = os.path.join("/tmp", "offhost-no-cred.bin")
        with open(path, "wb") as f:
            f.write(b"x")
        try:
            with self.assertRaises(offhost_s3.S3Error) as ctx:
                client.put_object("k", path, "", "")
            self.assertEqual(ctx.exception.code, "MissingCredentials")
        finally:
            os.unlink(path)

    def test_invalid_url_style_rejected(self):
        with self.assertRaises(ValueError):
            offhost_s3.S3Client("https://example.test", "b", url_style="query")


# ---------------------------------------------------------------------------
# Independent SigV4 verifier used by the fake server.  Re-implemented here so
# the test does not reuse the client's own canonicalization code.
# ---------------------------------------------------------------------------
_UNRESERVED = frozenset("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")


def _signing_key(secret, date_stamp, region, service):
    def _hmac(key, msg):
        return hmac.new(key, msg, hashlib.sha256).digest()

    k_date = _hmac(("AWS4" + secret).encode("utf-8"), date_stamp.encode("utf-8"))
    k_region = _hmac(k_date, region.encode("utf-8"))
    k_service = _hmac(k_region, service.encode("utf-8"))
    return _hmac(k_service, b"aws4_request")


def _quote_segment(value):
    return "".join(ch if ch in _UNRESERVED else "%%%02X" % ord(ch) for ch in value)


def _verify_auth(method, raw_path, headers, body, secret):
    """Verify the Authorization header on a raw HTTP request.

    Returns True if the signature recomputed from the wire bytes matches the
    one in the Authorization header.
    """
    auth = headers.get("Authorization")
    if not auth:
        return False
    m = re.match(
        r"^AWS4-HMAC-SHA256 Credential=([^/]+)/(\d{8})/([^/]+)/([^/]+)/aws4_request, "
        r"SignedHeaders=([^,]+), Signature=([0-9a-f]{64})$",
        auth,
    )
    if not m:
        return False
    access_key, date_stamp, region, service, signed_headers, expected = m.groups()

    path, _, query_raw = raw_path.partition("?")
    canonical_uri = "/".join(_quote_segment(seg) for seg in path.split("/"))

    query_pairs = urllib.parse.parse_qsl(query_raw, keep_blank_values=True)
    canonical_query = "&".join(
        "%s=%s"
        % (urllib.parse.quote(k, safe=""), urllib.parse.quote(v, safe="-_.~"))
        for k, v in sorted(query_pairs)
        if k is not None
    )

    signed_list = signed_headers.split(";")
    canonical_headers = "".join(
        "%s:%s\n" % (name, headers.get(name, "").strip()) for name in signed_list
    )

    payload_hash = headers.get("x-amz-content-sha256", hashlib.sha256(body).hexdigest())
    canonical_request = "\n".join(
        [method, canonical_uri, canonical_query, canonical_headers, signed_headers, payload_hash]
    )
    amz_date = headers.get("x-amz-date", "")
    scope = "%s/%s/%s/aws4_request" % (date_stamp, region, service)
    string_to_sign = "\n".join(
        [
            "AWS4-HMAC-SHA256",
            amz_date,
            scope,
            hashlib.sha256(canonical_request.encode("utf-8")).hexdigest(),
        ]
    )
    computed = _compute_signature(_signing_key(secret, date_stamp, region, service), string_to_sign)
    return hmac.compare_digest(computed, expected) and access_key == _KNOWN_ACCESS_KEY


_KNOWN_ACCESS_KEY = "AKID0123456789TESTKEY"
_KNOWN_SECRET_KEY = "S3cr3t+S3cr3t/K7MDENG/bPxRfiCYEXAMPLESecret"


class FakeS3Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    store = {}  # key -> bytes
    forbidden = False
    corrupt = False
    corrupt_file = None
    corrupt_suffix = None
    page_size = 2
    bucket = "testbucket"

    def log_message(self, *args):
        return

    def _key_from_path(self):
        """Strip the bucket prefix from a path-style request (virtual-style
        requests carry the bucket in the Host header and have no prefix)."""
        path = self.path.split("?")[0].lstrip("/")
        if path.startswith(self.bucket + "/"):
            return path[len(self.bucket) + 1 :]
        return path

    def _read_body(self):
        length = int(self.headers.get("Content-Length", 0))
        return self.rfile.read(length) if length else b""

    def _authorized(self, body):
        if self.forbidden:
            return False
        return _verify_auth(self.command, self.path, self.headers, body, _KNOWN_SECRET_KEY)

    def _reject(self, status, code, message):
        body = (
            '<?xml version="1.0" encoding="UTF-8"?>'
            "<Error><Code>%s</Code><Message>%s</Message></Error>" % (code, message)
        )
        self.send_response(status)
        self.send_header("Content-Type", "application/xml")
        self.send_header("Content-Length", str(len(body.encode())))
        self.end_headers()
        self.wfile.write(body.encode())

    def _respond_xml(self, status, root):
        body = ET.tostring(root, encoding="utf-8", xml_declaration=True)
        self.send_response(status)
        self.send_header("Content-Type", "application/xml")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_PUT(self):
        body = self._read_body()
        if not self._authorized(body):
            return self._reject(403, "AccessDenied", "signature or credentials invalid")
        key = self._key_from_path()
        self.store[key] = body
        self.send_response(200)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def do_GET(self):
        path, _, query_raw = self.path.partition("?")
        body = b""
        if not self._authorized(body):
            return self._reject(403, "AccessDenied", "signature or credentials invalid")
        if "list-type=2" in query_raw:
            return self._list(query_raw)
        key = self._key_from_path()
        if key not in self.store:
            return self._reject(404, "NoSuchKey", "The specified key does not exist.")
        data = self.store[key]
        corrupting = self.corrupt or (
            self.corrupt_file and os.path.exists(self.corrupt_file)
        )
        if corrupting and (
            not self.corrupt_suffix or key.endswith(self.corrupt_suffix)
        ):
            data = b"\x00CORRUPTED-" + data
        self.send_response(200)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_HEAD(self):
        body = b""
        if not self._authorized(body):
            return self._reject(403, "AccessDenied", "signature or credentials invalid")
        key = self._key_from_path()
        if key not in self.store:
            return self._reject(404, "NoSuchKey", "The specified key does not exist.")
        self.send_response(200)
        self.send_header("Content-Length", str(len(self.store[key])))
        self.end_headers()

    def do_DELETE(self):
        body = b""
        if not self._authorized(body):
            return self._reject(403, "AccessDenied", "signature or credentials invalid")
        key = self._key_from_path()
        self.store.pop(key, None)
        self.send_response(204)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _list(self, query_raw):
        params = dict(urllib.parse.parse_qsl(query_raw))
        prefix = params.get("prefix", "")
        token = int(params.get("continuation-token", "0"))
        matched = sorted(k for k in self.store if k.startswith(prefix))
        page = matched[token : token + self.page_size]
        truncated = token + self.page_size < len(matched)

        ns = "http://s3.amazonaws.com/doc/2006-03-01/"
        root = ET.Element("{%s}ListBucketResult" % ns)
        for child, text in (("Name", "testbucket"), ("Prefix", prefix), ("IsTruncated", "true" if truncated else "false")):
            node = ET.SubElement(root, "{%s}%s" % (ns, child))
            node.text = text
        for key in page:
            contents = ET.SubElement(root, "{%s}Contents" % ns)
            k = ET.SubElement(contents, "{%s}Key" % ns)
            k.text = key
            s = ET.SubElement(contents, "{%s}Size" % ns)
            s.text = str(len(self.store[key]))
            lm = ET.SubElement(contents, "{%s}LastModified" % ns)
            lm.text = "2026-08-24T02:00:00.000Z"
        if truncated:
            nxt = ET.SubElement(root, "{%s}NextContinuationToken" % ns)
            nxt.text = str(token + self.page_size)
        self._respond_xml(200, root)


class FakeServer:
    def __init__(self, handler=None):
        self.handler = handler or FakeS3Handler
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), self.handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def start(self):
        self.thread.start()
        return self

    def stop(self):
        self.server.shutdown()
        self.server.server_close()

    @property
    def url(self):
        host, port = self.server.server_address
        return "http://%s:%d" % (host, port)


class FakeS3IntegrationTests(unittest.TestCase):
    def setUp(self):
        FakeS3Handler.store = {}
        FakeS3Handler.forbidden = False
        self.server = FakeServer().start()
        self.client = offhost_s3.S3Client(
            self.server.url, "testbucket", region="cn-hangzhou", url_style="path"
        )

    def tearDown(self):
        self.server.stop()

    def test_put_get_roundtrip(self):
        path = os.path.join("/tmp", "offhost-put-test.bin")
        payload = b"custom-format pg archive bytes" * 100
        with open(path, "wb") as f:
            f.write(payload)
        try:
            self.client.put_object("b/bodysense.dump", path, _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            out = os.path.join("/tmp", "offhost-get-test.bin")
            self.client.get_object("b/bodysense.dump", out, _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            with open(out, "rb") as f:
                self.assertEqual(f.read(), payload)
            os.unlink(out)
        finally:
            os.unlink(path)

    def test_virtual_host_style(self):
        path = os.path.join("/tmp", "offhost-virtual-test.bin")
        with open(path, "wb") as f:
            f.write(b"virtual style payload")
        try:
            virtual = offhost_s3.S3Client(
                self.server.url, "testbucket", region="cn-hangzhou", url_style="virtual"
            )
            original_getaddrinfo = socket.getaddrinfo
            socket.getaddrinfo = lambda *args, **kwargs: original_getaddrinfo(
                "127.0.0.1", args[1], *args[2:], **kwargs
            )
            try:
                virtual.put_object("k/x.dump", path, _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            finally:
                socket.getaddrinfo = original_getaddrinfo
            # virtual-host style puts the bucket in the Host header, not the path
            self.assertIn("k/x.dump", FakeS3Handler.store)
            self.assertNotIn("testbucket/k/x.dump", FakeS3Handler.store)
        finally:
            os.unlink(path)

    def test_head_and_delete(self):
        path = os.path.join("/tmp", "offhost-head-test.bin")
        with open(path, "wb") as f:
            f.write(b"head me")
        try:
            self.client.put_object("k/h.dump", path, _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            status, _ = self.client.head_object("k/h.dump", _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            self.assertEqual(status, 200)
            status, _ = self.client.head_object("k/missing", _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            self.assertEqual(status, 404)
            self.client.delete_object("k/h.dump", _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            status, _ = self.client.head_object("k/h.dump", _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            self.assertEqual(status, 404)
        finally:
            os.unlink(path)

    def test_list_with_pagination(self):
        for i in range(5):
            path = os.path.join("/tmp", "offhost-list-%d.bin" % i)
            with open(path, "wb") as f:
                f.write(b"x" * (i + 1))
            try:
                self.client.put_object(
                    "p/20260824T020000Z/a-%d.dump" % i, path, _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY
                )
            finally:
                os.unlink(path)
        # a couple of objects that must not match the prefix
        for i in range(2):
            path = os.path.join("/tmp", "offhost-other-%d.bin" % i)
            with open(path, "wb") as f:
                f.write(b"y")
            try:
                self.client.put_object("q/other-%d.dump" % i, path, _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
            finally:
                os.unlink(path)

        objects = self.client.list_objects("p/", _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
        keys = sorted(o.key for o in objects)
        self.assertEqual(
            keys,
            sorted("p/20260824T020000Z/a-%d.dump" % i for i in range(5)),
        )
        self.assertEqual(
            {o.size for o in objects}, set(range(1, 6))
        )

    def test_forbidden_raises_s3_error(self):
        FakeS3Handler.forbidden = True
        with self.assertRaises(offhost_s3.S3Error) as ctx:
            self.client.list_objects("p/", _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
        self.assertEqual(ctx.exception.status, 403)
        self.assertEqual(ctx.exception.code, "AccessDenied")

    def test_missing_object_get_raises(self):
        out = os.path.join("/tmp", "offhost-missing-get.bin")
        with self.assertRaises(offhost_s3.S3Error) as ctx:
            self.client.get_object("k/absent.dump", out, _KNOWN_ACCESS_KEY, _KNOWN_SECRET_KEY)
        self.assertEqual(ctx.exception.code, "NoSuchKey")


class CliTests(unittest.TestCase):
    def setUp(self):
        FakeS3Handler.store = {}
        FakeS3Handler.forbidden = False
        self.server = FakeServer().start()

    def tearDown(self):
        self.server.stop()

    def run_cli(self, argv, env=None):
        merged = dict(os.environ)
        merged.update(env or {})
        return subprocess.run(
            [sys.executable, os.path.join(os.path.dirname(__file__), "offhost-s3.py")] + argv,
            capture_output=True,
            text=True,
            env=merged,
        )

    def test_cli_put_get_list(self):
        src = os.path.join("/tmp", "offhost-cli-src.bin")
        with open(src, "wb") as f:
            f.write(b"cli roundtrip payload")
        env = {"OFFHOST_BACKUP_ACCESS_KEY": _KNOWN_ACCESS_KEY, "OFFHOST_BACKUP_SECRET_KEY": _KNOWN_SECRET_KEY}
        try:
            put = self.run_cli(
                [
                    "put", "--endpoint", self.server.url, "--bucket", "testbucket",
                    "--key", "c/20260824T020000Z/f.dump", "--file", src, "--region", "cn-hangzhou",
                ],
                env=env,
            )
            self.assertEqual(put.returncode, 0, put.stderr)
            self.assertIn("PUT OK", put.stdout)

            listing = self.run_cli(
                ["list", "--endpoint", self.server.url, "--bucket", "testbucket",
                 "--prefix", "c/", "--region", "cn-hangzhou"],
                env=env,
            )
            self.assertEqual(listing.returncode, 0, listing.stderr)
            self.assertIn("c/20260824T020000Z/f.dump", listing.stdout)

            head = self.run_cli(
                ["head", "--endpoint", self.server.url, "--bucket", "testbucket",
                 "--key", "c/20260824T020000Z/f.dump", "--region", "cn-hangzhou"],
                env=env,
            )
            self.assertEqual(head.returncode, 0, head.stderr)
            self.assertIn("HEAD 200", head.stdout)
        finally:
            os.unlink(src)

    def test_cli_error_line_on_stderr(self):
        FakeS3Handler.forbidden = True
        env = {"OFFHOST_BACKUP_ACCESS_KEY": _KNOWN_ACCESS_KEY, "OFFHOST_BACKUP_SECRET_KEY": _KNOWN_SECRET_KEY}
        listing = self.run_cli(
            ["list", "--endpoint", self.server.url, "--bucket", "testbucket", "--region", "cn-hangzhou"],
            env=env,
        )
        self.assertNotEqual(listing.returncode, 0)
        self.assertIn("OFFHOST_S3_ERROR", listing.stderr)
        self.assertIn("status=403", listing.stderr)

    def test_cli_secret_not_leaked(self):
        env = {"OFFHOST_BACKUP_ACCESS_KEY": _KNOWN_ACCESS_KEY, "OFFHOST_BACKUP_SECRET_KEY": _KNOWN_SECRET_KEY}
        listing = self.run_cli(
            ["list", "--endpoint", self.server.url, "--bucket", "testbucket", "--region", "cn-hangzhou"],
            env=env,
        )
        combined = listing.stdout + listing.stderr
        self.assertNotIn(_KNOWN_SECRET_KEY, combined)
        self.assertNotIn(_KNOWN_ACCESS_KEY, combined)

    def test_cli_missing_secret_fails(self):
        listing = self.run_cli(
            ["list", "--endpoint", self.server.url, "--bucket", "testbucket", "--region", "cn-hangzhou"],
            env={},
        )
        self.assertNotEqual(listing.returncode, 0)
        self.assertIn("OFFHOST_S3_ERROR", listing.stderr)


def suite():
    return unittest.defaultTestLoader.loadTestsFromModule(sys.modules[__name__])


if __name__ == "__main__":
    unittest.main(verbosity=2)
