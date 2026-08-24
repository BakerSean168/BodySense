#!/usr/bin/env python3
"""BodySense off-host backup S3/S3-compatible client (stdlib only).

Implements AWS Signature Version 4 (AWS4-HMAC-SHA256) and the small object
store surface the off-host disaster-recovery scripts need: put, get, delete,
list and head. It deliberately depends only on the Python standard library so
the production host never needs an extra SDK, `aws` CLI or container to upload
or retrieve backups. Alibaba OSS and MinIO both accept this signing scheme.

Security contract (BS-PROD-012):
  -   credentials are read from the environment ONLY (OFFHOST_BACKUP_ACCESS_KEY /
      OFFHOST_BACKUP_SECRET_KEY) and never logged; the client refuses
      command-line credentials so secrets can never be observed through the host
      process table or /proc/<pid>/cmdline;
  -   the client never writes credentials into request bodies or metadata;
  -   privacy is PROVEN, not assumed: `check-private` is a fail-closed preflight
      that reads the destination bucket ACL and, in `--policy-proof required`
      mode (the default), also reads bucket policy status; any bucket granting
      public groups is refused.  A store that cannot answer policy status is
      refused in required mode (its policy status cannot be verified); stores
      that do not implement the operation (e.g. Alibaba OSS) must be explicitly
      configured with `--policy-proof acl-only`, where the ACL is the only
      machine-verified proof and an OFFHOST_S3_WARNING is printed.  The client
      never sets a public ACL and rejects any requested object ACL other than
      `private`;
  -   health-data objects are uploaded with server-side encryption by default
      (`--sse AES256`).

Usage (options may appear before or after the command; credentials come from the
environment, never from the command line):
  offhost-s3.py put            --endpoint URL --bucket B --key K --file PATH [--region R] [--url-style path|virtual] [--acl private] [--sse AES256]
  offhost-s3.py get            --endpoint URL --bucket B --key K --file PATH [--region R] [--url-style path|virtual]
  offhost-s3.py delete         --endpoint URL --bucket B --key K [--region R] [--url-style path|virtual]
  offhost-s3.py list           --endpoint URL --bucket B [--prefix P] [--region R] [--url-style path|virtual]
  offhost-s3.py head           --endpoint URL --bucket B --key K [--region R] [--url-style path|virtual]
  offhost-s3.py check-private  --endpoint URL --bucket B [--region R] [--url-style path|virtual] [--policy-proof required|acl-only]
"""

import argparse
import hashlib
import hmac
import sys
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from datetime import datetime, timezone

__all__ = [
    "S3Error",
    "S3Object",
    "S3Client",
]

SERVICE = "s3"
_ALGORITHM = "AWS4-HMAC-SHA256"
_UNRESERVED = frozenset(
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
)
_S3_XML_NS = "{http://s3.amazonaws.com/doc/2006-03-01/}"


class S3Error(Exception):
    """Raised for non-2xx responses or malformed object-store replies."""

    def __init__(self, status, code, message):
        self.status = status
        self.code = code
        super().__init__("S3 error %s (%s): %s" % (status, code, message))


class S3Object:
    __slots__ = ("key", "size", "last_modified")

    def __init__(self, key, size, last_modified):
        self.key = key
        self.size = size
        self.last_modified = last_modified

    def __repr__(self):  # pragma: no cover - debug aid
        return "S3Object(key=%r, size=%r, last_modified=%r)" % (
            self.key,
            self.size,
            self.last_modified,
        )


def _sha256_hex(data):
    return hashlib.sha256(data).hexdigest()


def _hmac(key, msg):
    return hmac.new(key, msg, hashlib.sha256).digest()


def _signing_key(secret_key, date_stamp, region):
    k_date = _hmac(("AWS4" + secret_key).encode("utf-8"), date_stamp.encode("utf-8"))
    k_region = _hmac(k_date, region.encode("utf-8"))
    k_service = _hmac(k_region, SERVICE.encode("utf-8"))
    return _hmac(k_service, b"aws4_request")


def _uri_encode_path(path):
    """Encode each path segment per RFC 3986, preserving '/' separators."""
    segments = path.split("/")
    encoded = []
    for segment in segments:
        encoded.append(
            "".join(ch if ch in _UNRESERVED else "%%%02X" % ord(ch) for ch in segment)
        )
    return "/".join(encoded)


def _canonical_query(query):
    if not query:
        return ""
    pairs = sorted(
        (urllib.parse.quote(k, safe=""), urllib.parse.quote(v, safe="-_.~"))
        for k, v in query.items()
        if k is not None and v is not None
    )
    return "&".join("%s=%s" % (k, v) for k, v in pairs)


class S3Client:
    """Minimal S3-compatible client with SigV4 request signing."""

    def __init__(self, endpoint, bucket, region="us-east-1", url_style="path"):
        if "://" not in endpoint:
            endpoint = "https://" + endpoint
        parsed = urllib.parse.urlparse(endpoint)
        if parsed.scheme not in ("http", "https"):
            raise ValueError("unsupported endpoint scheme: %s" % parsed.scheme)
        self.scheme = parsed.scheme
        self.host = parsed.netloc
        self.base_path = parsed.path.rstrip("/")
        self.bucket = bucket
        self.region = region
        if url_style not in ("path", "virtual"):
            raise ValueError("url-style must be 'path' or 'virtual'")
        self.url_style = url_style

    def _object_host(self):
        if self.url_style == "virtual":
            return "%s.%s" % (self.bucket, self.host)
        return self.host

    def _object_path(self, key):
        """Raw (pre-encoding) request path.  Path-style includes the bucket in
        the path; virtual-host style carries the bucket in the Host header."""
        if self.url_style == "virtual":
            return self.base_path + "/" + key
        return self.base_path + "/" + self.bucket + "/" + key

    def _url(self, key, query=None):
        netloc = self._object_host()
        encoded_path = _uri_encode_path(self._object_path(key))
        if query:
            encoded_path += "?" + _canonical_query(query)
        return urllib.parse.urlunparse((self.scheme, netloc, encoded_path, "", "", ""))

    def _signed_request(
        self,
        method,
        key,
        body,
        query,
        access_key,
        secret_key,
        content_type=None,
        acl=None,
        sse=None,
    ):
        if not access_key or not secret_key:
            raise S3Error(0, "MissingCredentials", "access key and secret key are required")
        now = datetime.now(timezone.utc)
        amz_date = now.strftime("%Y%m%dT%H%M%SZ")
        date_stamp = now.strftime("%Y%m%d")
        payload_hash = _sha256_hex(body)

        canonical_uri = _uri_encode_path(self._object_path(key))
        canonical_query = _canonical_query(query or {})

        headers = {
            "host": self._object_host(),
            "x-amz-content-sha256": payload_hash,
            "x-amz-date": amz_date,
        }
        if content_type:
            headers["content-type"] = content_type
        if acl:
            headers["x-amz-acl"] = acl
        if sse:
            headers["x-amz-server-side-encryption"] = sse
        signed_headers_list = sorted(headers)
        signed_headers = ";".join(signed_headers_list)
        canonical_headers = "".join(
            "%s:%s\n" % (h, headers[h]) for h in signed_headers_list
        )

        canonical_request = "\n".join(
            [method, canonical_uri, canonical_query, canonical_headers, signed_headers, payload_hash]
        )
        scope = "%s/%s/%s/aws4_request" % (date_stamp, self.region, SERVICE)
        string_to_sign = "\n".join(
            [_ALGORITHM, amz_date, scope, _sha256_hex(canonical_request.encode("utf-8"))]
        )
        signature = hmac.new(
            _signing_key(secret_key, date_stamp, self.region),
            string_to_sign.encode("utf-8"),
            hashlib.sha256,
        ).hexdigest()
        authorization = (
            "%s Credential=%s/%s, SignedHeaders=%s, Signature=%s"
            % (_ALGORITHM, access_key, scope, signed_headers, signature)
        )

        request = urllib.request.Request(self._url(key, query), data=body, method=method)
        request.add_header("host", headers["host"])
        request.add_header("x-amz-content-sha256", payload_hash)
        request.add_header("x-amz-date", amz_date)
        if content_type:
            request.add_header("content-type", content_type)
        if acl:
            request.add_header("x-amz-acl", acl)
        if sse:
            request.add_header("x-amz-server-side-encryption", sse)
        request.add_header("Authorization", authorization)
        return request, authorization

    def _send(self, request):
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                return response.status, response.read()
        except urllib.error.HTTPError as error:
            body = error.read()
            code = _extract_error_code(body)
            raise S3Error(error.code, code, body.decode("utf-8", "replace")) from None
        except urllib.error.URLError as error:
            raise S3Error(0, "ConnectionError", str(error)) from None

    def put_object(self, key, local_path, access_key, secret_key, content_type="application/octet-stream", acl=None, sse=None):
        if acl not in (None, "", "private"):
            raise S3Error(0, "InvalidObjectAcl", "only an object ACL of 'private' is allowed")
        with open(local_path, "rb") as handle:
            body = handle.read()
        request, _ = self._signed_request(
            "PUT", key, body, None, access_key, secret_key,
            content_type=content_type, acl=acl or None, sse=sse or None,
        )
        status, _ = self._send(request)
        if status != 200:
            raise S3Error(status, "PutObjectFailed", "unexpected status %d" % status)

    def get_object(self, key, local_path, access_key, secret_key):
        request, _ = self._signed_request(
            "GET", key, b"", None, access_key, secret_key
        )
        status, body = self._send(request)
        if status != 200:
            raise S3Error(status, "GetObjectFailed", "unexpected status %d" % status)
        with open(local_path, "wb") as handle:
            handle.write(body)

    def delete_object(self, key, access_key, secret_key):
        request, _ = self._signed_request(
            "DELETE", key, b"", None, access_key, secret_key
        )
        status, _ = self._send(request)
        if status not in (200, 204):
            raise S3Error(status, "DeleteObjectFailed", "unexpected status %d" % status)

    def head_object(self, key, access_key, secret_key):
        request, _ = self._signed_request("HEAD", key, b"", None, access_key, secret_key)
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                return response.status, response.headers
        except urllib.error.HTTPError as error:
            if error.code == 404:
                return 404, error.headers
            raise S3Error(
                error.code,
                _extract_error_code(error.read()),
                "head failed",
            ) from None

    def list_objects(self, prefix, access_key, secret_key):
        continuation_token = None
        result = []
        while True:
            query = {"list-type": "2", "prefix": prefix}
            if continuation_token:
                query["continuation-token"] = continuation_token
            request, _ = self._signed_request(
                "GET", "", b"", query, access_key, secret_key
            )
            status, body = self._send(request)
            if status != 200:
                raise S3Error(status, "ListObjectsFailed", "unexpected status %d" % status)
            root = ET.fromstring(body)
            namespace = ""
            if root.tag.startswith("{"):
                namespace = root.tag.split("}", 1)[0] + "}"
            for contents in root.findall("%sContents" % namespace):
                key_node = contents.find("%sKey" % namespace)
                size_node = contents.find("%sSize" % namespace)
                modified_node = contents.find("%sLastModified" % namespace)
                if key_node is None or key_node.text is None:
                    continue
                result.append(
                    S3Object(
                        key_node.text,
                        int(size_node.text) if size_node is not None and size_node.text else 0,
                        modified_node.text if modified_node is not None else None,
                    )
                )
            truncated = root.find("%sIsTruncated" % namespace)
            if truncated is not None and truncated.text == "true":
                token = root.find("%sNextContinuationToken" % namespace)
                continuation_token = token.text if token is not None else None
                if not continuation_token:
                    break
            else:
                break
        return result

    def check_private(self, access_key, secret_key, policy_proof=True):
        """Fail-closed private-destination preflight for the backup bucket.

        Returns True only when the bucket ACL can be fetched, parsed and proven
        to grant nothing to public groups.  With policy_proof=True (the default,
        `acl+policy` mode) a GetBucketPolicyStatus result is additionally
        required and is fail-closed:
          - IsPublic=true is refused (BucketPublicPolicy);
          - an error answering the request is refused (BucketPolicyStatusDenied
            for a real authorization failure, BucketPolicyStatusFailed for any
            other non-200) — including a store that does NOT implement the
            operation (BucketPolicyStatusUnsupported), because an absent policy
            status cannot be read and therefore cannot be proven private;
          - a 200 reply without a parseable IsPublic is refused
            (BucketPolicyStatusMalformed).
        With policy_proof=False (`acl` mode) the bucket ACL is the only
        machine-verified privacy proof, the policy status is NOT queried, and an
        OFFHOST_S3_WARNING line is printed so callers' logs record that the
        weaker proof was used.  Either way, a store that cannot PROVE the bucket
        is private never passes on an assumption.
        """
        if not access_key or not secret_key:
            raise S3Error(0, "MissingCredentials", "access key and secret key are required")
        try:
            request, _ = self._signed_request("GET", "", b"", {"acl": ""}, access_key, secret_key)
            status, body = self._send(request)
        except S3Error as error:
            raise S3Error(
                error.status or 0,
                "BucketAclUnreadable",
                "cannot read bucket ACL: %s" % error,
            ) from None
        if status != 200:
            raise S3Error(status, "BucketAclUnreadable", "bucket ACL returned HTTP %d" % status)
        acl_is_private(body.decode("utf-8", "replace"))

        if not policy_proof:
            print(
                "OFFHOST_S3_WARNING policy status is NOT verified (acl-only proof); "
                "the bucket ACL is the only machine-verified privacy proof",
                file=sys.stderr,
            )
            return True

        try:
            request, _ = self._signed_request("GET", "", b"", {"policyStatus": ""}, access_key, secret_key)
            policy_status, policy_body = self._send(request)
        except S3Error as error:
            if _is_operation_unsupported(error.status, error.code):
                raise S3Error(
                    error.status or 0,
                    "BucketPolicyStatusUnsupported",
                    "bucket policy status is not implemented by this store and cannot be verified: %s" % error,
                ) from None
            raise S3Error(
                error.status or 0,
                "BucketPolicyStatusDenied",
                "cannot read bucket policy status: %s" % error,
            ) from None
        if policy_status != 200:
            raise S3Error(
                policy_status,
                "BucketPolicyStatusFailed",
                "bucket policy status returned HTTP %d" % policy_status,
            )
        if policy_status_is_public(policy_body):
            raise S3Error(403, "BucketPublicPolicy", "bucket policy status IsPublic=true")
        return True


def _extract_error_code(body):
    try:
        root = ET.fromstring(body)
        namespace = ""
        if root.tag.startswith("{"):
            namespace = root.tag.split("}", 1)[0] + "}"
        code = root.find("%sCode" % namespace)
        if code is not None and code.text:
            return code.text
    except ET.ParseError:
        pass
    return "Unknown"


# Stores that do not implement the GetBucketPolicyStatus operation answer it
# with one of these machine-readable signals.  A store that cannot answer policy
# status cannot have its bucket policy machine-verified.
_UNSUPPORTED_POLICY_STATUS_ERROR_CODES = ("NotImplemented", "MethodNotAllowed", "UnknownOperation")


def _is_operation_unsupported(status, code):
    if status == 501:
        return True
    return bool(code) and code in _UNSUPPORTED_POLICY_STATUS_ERROR_CODES


_PUBLIC_ACL_URI_MARKERS = ("AllUsers", "AuthenticatedUsers", "groups/global/")


def acl_is_private(acl_body):
    """Fail-closed check of a bucket ACL: True only when the ACL is readable,
    well-formed and grants nothing to any public group.

    Any of the following is a FAILURE (raises), never a pass:
      - the body is not XML (BucketAclMalformed);
      - the ACL structure contains no grants at all (BucketAclMalformed);
      - any grantee grants a public group (AllUsers / AuthenticatedUsers, or any
        group URI under groups/global/) regardless of the stated type
        (BucketPublicAcl).
    """
    try:
        root = ET.fromstring(acl_body)
    except ET.ParseError:
        raise S3Error(0, "BucketAclMalformed", "bucket ACL is not valid XML") from None
    grants = root.findall(".//%sGrant" % _S3_XML_NS)
    if not grants:
        raise S3Error(0, "BucketAclMalformed", "bucket ACL contains no readable grants")
    for grant in grants:
        grantee = grant.find("%sGrantee" % _S3_XML_NS)
        if grantee is None:
            continue
        uri_node = grantee.find("%sURI" % _S3_XML_NS)
        uri = (uri_node.text or "") if uri_node is not None else ""
        if uri:
            for marker in _PUBLIC_ACL_URI_MARKERS:
                if marker in uri:
                    raise S3Error(403, "BucketPublicAcl", "bucket ACL grants public access to %s" % uri)
    return True


def policy_status_is_public(body):
    """Parse a GetBucketPolicyStatus reply; True means IsPublic == 'true'.

    FAIL-CLOSED: a 200 reply that is not valid XML, or that carries no parseable
    IsPublic element, raises BucketPolicyStatusMalformed rather than being
    treated as "not public".  A malformed policy status can never be a pass.
    """
    try:
        root = ET.fromstring(body)
    except ET.ParseError:
        raise S3Error(0, "BucketPolicyStatusMalformed", "bucket policy status is not valid XML") from None
    node = root.find(".//{http://s3.amazonaws.com/doc/2006-03-01/}IsPublic")
    if node is None or not isinstance(node.text, str) or not node.text.strip():
        raise S3Error(
            0,
            "BucketPolicyStatusMalformed",
            "bucket policy status reply is missing a parseable IsPublic value",
        )
    return node.text.strip().lower() == "true"


def _open_output_for_list(objects):
    for obj in objects:
        print("%s\t%s\t%s" % (obj.key, obj.size, obj.last_modified or ""))


def _parse_args(argv):
    parser = argparse.ArgumentParser(description="BodySense off-host S3 client")
    parser.add_argument(
        "command",
        choices=("put", "get", "delete", "head", "list", "check-private"),
    )
    parser.add_argument("--endpoint", required=True, help="object store endpoint URL")
    parser.add_argument("--bucket", required=True, help="private bucket name")
    parser.add_argument("--region", default="us-east-1", help="signing region")
    parser.add_argument("--url-style", choices=("path", "virtual"), default="path")
    parser.add_argument(
        "--access-key",
        help="DEPRECATED: refused for security; credentials are env-only",
    )
    parser.add_argument(
        "--secret-key",
        help="DEPRECATED: refused for security; credentials are env-only",
    )
    parser.add_argument("--key", help="object key (required for put/get/delete/head)")
    parser.add_argument("--file", help="local file path (required for put/get)")
    parser.add_argument("--prefix", default="", help="object key prefix for list")
    parser.add_argument(
        "--acl",
        default="",
        help="object ACL for put; only 'private' or empty is accepted "
        "(the backup script always sends 'private' by default; empty means the "
        "x-amz-acl header is not sent)",
    )
    parser.add_argument(
        "--sse",
        default="",
        help="server-side encryption algorithm for put: AES256 or aws:kms; empty means "
        "the header is not sent (the backup script defaults to AES256)",
    )
    parser.add_argument(
        "--policy-proof",
        choices=("required", "acl-only"),
        default="required",
        help="check-private privacy proof: 'required' (default) verifies the ACL AND "
        "GetBucketPolicyStatus fail-closed; 'acl-only' verifies only the bucket ACL "
        "and prints an OFFHOST_S3_WARNING (for stores that do not implement policy "
        "status, e.g. Alibaba OSS)",
    )
    return parser.parse_args(argv)


def main(argv=None):
    args = _parse_args(argv if argv is not None else sys.argv[1:])
    if args.access_key or args.secret_key:
        print(
            "OFFHOST_S3_ERROR status=0 code=CliCredentialsRefused "
            "message=credentials must come from the environment, not the command line",
            file=sys.stderr,
        )
        sys.exit(1)
    access_key = _env_or_empty("OFFHOST_BACKUP_ACCESS_KEY")
    secret_key = _env_or_empty("OFFHOST_BACKUP_SECRET_KEY")
    client = S3Client(args.endpoint, args.bucket, region=args.region, url_style=args.url_style)

    if args.command == "put":
        if not args.key or not args.file:
            raise S3Error(0, "MissingArgument", "put requires --key and --file")
        if args.acl and args.acl != "private":
            raise S3Error(0, "InvalidObjectAcl", "only an object ACL of 'private' is allowed")
        client.put_object(
            args.key,
            args.file,
            access_key,
            secret_key,
            acl=args.acl or None,
            sse=args.sse or None,
        )
        print("PUT OK %s" % args.key)
    elif args.command == "get":
        if not args.key or not args.file:
            raise S3Error(0, "MissingArgument", "get requires --key and --file")
        client.get_object(args.key, args.file, access_key, secret_key)
        print("GET OK %s" % args.key)
    elif args.command == "delete":
        if not args.key:
            raise S3Error(0, "MissingArgument", "delete requires --key")
        client.delete_object(args.key, access_key, secret_key)
        print("DELETE OK %s" % args.key)
    elif args.command == "head":
        if not args.key:
            raise S3Error(0, "MissingArgument", "head requires --key")
        status, _headers = client.head_object(args.key, access_key, secret_key)
        print("HEAD %d %s" % (status, args.key))
        if status == 404:
            return 1
    elif args.command == "list":
        _open_output_for_list(client.list_objects(args.prefix, access_key, secret_key))
    elif args.command == "check-private":
        policy_proof = args.policy_proof != "acl-only"
        client.check_private(access_key, secret_key, policy_proof=policy_proof)
        print(
            "PRIVATE_PREFLIGHT=PASS proof=%s bucket=%s"
            % ("acl+policy" if policy_proof else "acl", args.bucket)
        )
    return 0


def _env_or_empty(name):
    import os

    return os.environ.get(name, "")


if __name__ == "__main__":
    try:
        sys.exit(main())
    except S3Error as error:
        print("OFFHOST_S3_ERROR status=%s code=%s message=%s" % (error.status, error.code, error), file=sys.stderr)
        sys.exit(1)
