import io
import json
import unittest
import urllib.error
from unittest import mock

import backup


def http_error(url, code):
    return urllib.error.HTTPError(url, code, "error", {}, io.BytesIO(b'{"error":{"message":"Invalid api_key 123"}}'))


class FailureDetailsTest(unittest.TestCase):
    def test_http_error_keeps_status_and_host_only(self):
        error = http_error("https://api.cloudinary.com/v1_1/demo/image/download?api_key=123&signature=abc", 401)
        details = backup.failure_details(error, {"step": "download", "kind": "image/private"})
        self.assertEqual(details, {"error_type": "HTTPError", "http_status": 401, "host": "api.cloudinary.com",
                                   "step": "download", "kind": "image/private"})
        self.assertNotIn("123", json.dumps(details))
        self.assertNotIn("signature", json.dumps(details))

    def test_url_error_reports_reason_type(self):
        details = backup.failure_details(urllib.error.URLError(TimeoutError("x")))
        self.assertEqual(details, {"error_type": "URLError", "reason_type": "TimeoutError"})

    def test_runtime_error_reason_only_for_fixed_codes(self):
        self.assertEqual(backup.failure_details(RuntimeError("media_size_mismatch"))["reason"], "media_size_mismatch")
        self.assertNotIn("reason", backup.failure_details(RuntimeError("password=secret value")))


class CopyMediaProgressTest(unittest.TestCase):
    def test_progress_names_failing_listing(self):
        progress = {}
        with mock.patch.object(backup, "cloudinary_get", side_effect=[b'{"resources": []}', http_error("https://api.cloudinary.com/x", 404)]):
            with self.assertRaises(urllib.error.HTTPError):
                backup.copy_media({"cloud": "demo", "key": "k", "secret": "s"}, "bucket", "media/x", "t", lambda *a: None, progress)
        self.assertEqual(progress, {"step": "list", "kind": "image/authenticated"})

    def test_progress_counts_copied_files(self):
        listing = json.dumps({"resources": [{"public_id": "a", "format": "png", "bytes": 3, "etag": "acbd18db4cc2f85cedef654fccc4a4d8",
                                             "secure_url": "https://res.cloudinary.com/demo/image/upload/a.png"}]}).encode()
        responses = [listing] + [b'{"resources": []}'] * (len(backup.MEDIA_KINDS) - 1) + [b"foo"]
        uploads, progress = [], {}
        with mock.patch.object(backup, "cloudinary_get", side_effect=responses):
            manifest = backup.copy_media({"cloud": "demo", "key": "k", "secret": "s"}, "bucket", "media/x", "t",
                                         lambda b, n, d, t: uploads.append(n), progress)
        self.assertEqual(manifest["files"], 1)
        self.assertEqual(uploads, ["media/x/image/upload/a.png", "media/x/manifest.json"])
        self.assertEqual(progress, {"step": "manifest", "kind": None, "listed": 1, "copied": 1})


class CloudinaryGetAuthTest(unittest.TestCase):
    cfg = {"cloud": "demo", "key": "k", "secret": "s"}

    def sent_request(self, *args):
        response = mock.MagicMock()
        response.__enter__.return_value.read.return_value = b"foo"
        with mock.patch.object(backup.urllib.request, "urlopen", return_value=response) as urlopen:
            backup.cloudinary_get(self.cfg, *args)
        return urlopen.call_args.args[0]

    def test_admin_api_listing_sends_basic_auth(self):
        request = self.sent_request("https://api.cloudinary.com/v1_1/demo/resources/image/upload?max_results=500")
        self.assertTrue(request.get_header("Authorization").startswith("Basic "))

    def test_delivery_fetch_sends_no_credentials(self):
        for url in ("https://res.cloudinary.com/demo/image/upload/a.png", "https://cdn.example.com/demo/image/upload/a.png"):
            request = self.sent_request(url)
            self.assertIsNone(request.get_header("Authorization"))
            self.assertNotIn("api_key", request.full_url)

    def test_signed_download_keeps_query_signature_without_header(self):
        request = self.sent_request("https://api.cloudinary.com/v1_1/demo/image/download", {"public_id": "a", "type": "private"})
        self.assertIsNone(request.get_header("Authorization"))
        self.assertIn("signature=", request.full_url)


if __name__ == "__main__":
    unittest.main()
