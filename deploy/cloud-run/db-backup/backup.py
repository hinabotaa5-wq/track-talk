"""Read-only PostgreSQL backup. Never prints database values or credentials."""
import base64
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.error
import urllib.request
import uuid

SCHEMAS = ("public", "auth", "storage", "supabase_migrations")
MAX_BYTES = 64 * 1024 * 1024
MEDIA_MAX_FILE_BYTES = 110 * 1024 * 1024
MEDIA_MAX_TOTAL_BYTES = 2 * 1024 * 1024 * 1024
MEDIA_KINDS = [(r, d) for r in ("image", "video", "raw") for d in ("upload", "authenticated", "private")]
CLOUDINARY_API = os.environ.get("CLOUDINARY_API_BASE", "https://api.cloudinary.com")
STORAGE_UPLOAD = os.environ.get("STORAGE_UPLOAD_BASE", "https://storage.googleapis.com")
TOKYO = dt.timezone(dt.timedelta(hours=9))


def failure_details(error, progress=None):
    """Loggable failure fields: type, HTTP status and host only. URLs, queries, signatures and bodies are never logged."""
    details = {"error_type": type(error).__name__}
    if isinstance(error, urllib.error.HTTPError):
        details["http_status"] = error.code
        details["host"] = urllib.parse.urlsplit(error.geturl() or "").hostname
    elif isinstance(error, urllib.error.URLError):
        details["reason_type"] = type(error.reason).__name__
    elif isinstance(error, RuntimeError) and re.fullmatch(r"[a-z0-9_]{1,64}", str(error)):
        details["reason"] = str(error)
    if progress:
        details.update(progress)
    return details


def pg_env(url):
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme not in ("postgres", "postgresql") or not parsed.hostname:
        raise RuntimeError("invalid_database_configuration")
    query = urllib.parse.parse_qs(parsed.query)
    sslmode = query.get("sslmode", ["require"])[0]
    if sslmode not in ("require", "verify-ca", "verify-full"):
        raise RuntimeError("encrypted_database_connection_required")
    result = os.environ.copy()
    result.pop("DATABASE_URL", None)
    result.update(PGHOST=parsed.hostname, PGPORT=str(parsed.port or 5432),
                  PGUSER=urllib.parse.unquote(parsed.username or ""),
                  PGPASSWORD=urllib.parse.unquote(parsed.password or ""),
                  PGDATABASE=urllib.parse.unquote(parsed.path.lstrip("/")),
                  PGSSLMODE=sslmode, PGCONNECT_TIMEOUT="15",
                  PGAPPNAME="track-talk-db-backup",
                  PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=180000 -c lock_timeout=5000")
    if query.get("sslrootcert"):
        result["PGSSLROOTCERT"] = query["sslrootcert"][0]
    return result


class Snapshot:
    def __init__(self, env, errors):
        self.process = subprocess.Popen(["psql", "-XAtq", "--no-password", "-v", "ON_ERROR_STOP=1"],
                                        env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                        stderr=errors, text=True, bufsize=1)
        self.query("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;")
        self.info = json.loads(self.query("SELECT json_build_object('snapshot',pg_export_snapshot(),'server_version',current_setting('server_version_num'),'read_only',current_setting('transaction_read_only'));")[0])
        if self.info["read_only"] != "on":
            raise RuntimeError("read_only_transaction_required")

    def query(self, sql):
        marker = "end_" + uuid.uuid4().hex
        self.process.stdin.write(sql + "\n\\echo " + marker + "\n")
        self.process.stdin.flush()
        rows = []
        while True:
            line = self.process.stdout.readline()
            if not line:
                raise RuntimeError("database_snapshot_query_failed")
            line = line.rstrip("\n")
            if line == marker:
                return rows
            rows.append(line)

    def audit(self):
        names = ",".join("'" + x + "'" for x in SCHEMAS)
        tables = [json.loads(x) for x in self.query(f"SELECT json_build_array(n.nspname,c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND n.nspname IN ({names}) ORDER BY n.nspname,c.relname;")]
        audit = []
        for schema, table in tables:
            identifier = '.'.join('"' + x.replace('"', '""') + '"' for x in (schema, table))
            result = json.loads(self.query(f"SELECT json_build_object('rows',count(*),'digest',md5(coalesce(string_agg(md5(to_jsonb(t)::text),'' ORDER BY md5(to_jsonb(t)::text)),''))) FROM {identifier} t;")[0])
            audit.append(dict(schema=schema, table=table, **result))
        metadata = json.loads(self.query(f"SELECT coalesce(jsonb_agg(x ORDER BY x.schema,x.name),'[]'::jsonb) FROM (SELECT n.nspname AS schema,c.relname AS name,c.relrowsecurity AS rls,c.relforcerowsecurity AS force_rls,(SELECT count(*) FROM pg_policy p WHERE p.polrelid=c.oid) AS policies,(SELECT count(*) FROM pg_constraint k WHERE k.conrelid=c.oid) AS constraints,(SELECT count(*) FROM pg_index i WHERE i.indrelid=c.oid) AS indexes FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND n.nspname IN ({names})) x;")[0])
        return {"tables": audit, "metadata": metadata}

    def close(self):
        if self.process.poll() is None:
            try:
                self.process.stdin.write("ROLLBACK;\n\\q\n")
                self.process.stdin.flush()
                self.process.communicate(timeout=10)
            except (BrokenPipeError, subprocess.TimeoutExpired):
                self.process.kill()
                self.process.wait()


def token():
    req = urllib.request.Request("http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token", headers={"Metadata-Flavor": "Google"})
    with urllib.request.urlopen(req, timeout=10) as response:
        return json.load(response)["access_token"]


def upload(bucket, name, data, access_token, limit=MAX_BYTES):
    if len(data) > limit:
        raise RuntimeError("backup_size_limit_exceeded")
    query = urllib.parse.urlencode({"uploadType": "media", "name": name, "ifGenerationMatch": "0"})
    url = STORAGE_UPLOAD + "/upload/storage/v1/b/" + urllib.parse.quote(bucket, safe="") + "/o?" + query
    request = urllib.request.Request(url, data=data, method="POST", headers={"Authorization": "Bearer " + access_token, "Content-Type": "application/octet-stream"})
    with urllib.request.urlopen(request, timeout=90) as response:
        stored = json.load(response)
    if stored.get("md5Hash") != base64.b64encode(hashlib.md5(data).digest()).decode():
        raise RuntimeError("uploaded_object_checksum_mismatch")
    return stored["generation"]


def media_due(now):
    # Weekly media copy on Sunday (Asia/Tokyo); MEDIA_BACKUP_FORCE=1 runs it on demand.
    return os.environ.get("MEDIA_BACKUP_FORCE") == "1" or now.astimezone(TOKYO).weekday() == int(os.environ.get("MEDIA_BACKUP_WEEKDAY", "6"))


def cloudinary_get(cfg, url, signed_params=None):
    if signed_params is not None:
        params = dict(signed_params, timestamp=str(int(dt.datetime.now(dt.timezone.utc).timestamp())))
        payload = "&".join(f"{k}={params[k]}" for k in sorted(params)) + cfg["secret"]
        params.update(api_key=cfg["key"], signature=hashlib.sha1(payload.encode()).hexdigest())
        url += "?" + urllib.parse.urlencode(params)
        request = urllib.request.Request(url)
    else:
        auth = base64.b64encode((cfg["key"] + ":" + cfg["secret"]).encode()).decode()
        request = urllib.request.Request(url, headers={"Authorization": "Basic " + auth})
    with urllib.request.urlopen(request, timeout=90) as response:
        data = response.read(MEDIA_MAX_FILE_BYTES + 1)
    if len(data) > MEDIA_MAX_FILE_BYTES:
        raise RuntimeError("media_file_size_limit_exceeded")
    return data


def list_media(cfg, progress):
    assets = []
    for resource, delivery in MEDIA_KINDS:
        progress.update(step="list", kind=resource + "/" + delivery)
        cursor = None
        while True:
            query = {"max_results": "500"}
            if cursor:
                query["next_cursor"] = cursor
            url = f"{CLOUDINARY_API}/v1_1/{urllib.parse.quote(cfg['cloud'])}/resources/{resource}/{delivery}?" + urllib.parse.urlencode(query)
            page = json.loads(cloudinary_get(cfg, url))
            for item in page.get("resources", []):
                assets.append(dict(item, resource_type=resource, type=delivery))
            cursor = page.get("next_cursor")
            if not cursor:
                break
    return assets


def fetch_media(cfg, asset):
    if asset["type"] == "upload":
        data = cloudinary_get(cfg, asset["secure_url"])
    else:
        params = {"public_id": asset["public_id"], "type": asset["type"]}
        if asset.get("format"):
            params["format"] = asset["format"]
        data = cloudinary_get(cfg, f"{CLOUDINARY_API}/v1_1/{urllib.parse.quote(cfg['cloud'])}/{asset['resource_type']}/download", params)
    etag = asset.get("etag")
    if not etag:
        # Listings may omit the checksum; the per-asset details carry it.
        url = f"{CLOUDINARY_API}/v1_1/{urllib.parse.quote(cfg['cloud'])}/resources/{asset['resource_type']}/{asset['type']}/{urllib.parse.quote(asset['public_id'])}"
        etag = json.loads(cloudinary_get(cfg, url)).get("etag")
    if asset.get("bytes") is not None and len(data) != asset["bytes"]:
        raise RuntimeError("media_size_mismatch")
    if etag and hashlib.md5(data).hexdigest() != etag:
        raise RuntimeError("media_checksum_mismatch")
    return data, "md5" if etag else "size"


def copy_media(cfg, bucket, prefix, access_token, upload_fn, progress):
    """Full weekly snapshot under media/; the bucket's 30-day lifecycle removes old snapshots."""
    assets = list_media(cfg, progress)
    if sum(a.get("bytes", 0) for a in assets) > MEDIA_MAX_TOTAL_BYTES:
        raise RuntimeError("media_total_size_limit_exceeded")
    entries = []
    progress.update(listed=len(assets), copied=0)
    for asset in assets:
        progress.update(step="download", kind=asset["resource_type"] + "/" + asset["type"])
        data, verified = fetch_media(cfg, asset)
        name = "/".join((asset["resource_type"], asset["type"], asset["public_id"])) + ("." + asset["format"] if asset.get("format") else "")
        progress["step"] = "upload"
        upload_fn(bucket, prefix + "/" + name, data, access_token)
        entries.append({"name": name, "bytes": len(data), "md5": hashlib.md5(data).hexdigest(), "verified_by": verified})
        progress["copied"] = len(entries)
    manifest = {"format_version": 1, "created_at": dt.datetime.now(dt.timezone.utc).isoformat(), "files": len(entries),
                "verified_by_md5": sum(1 for e in entries if e["verified_by"] == "md5"),
                "bytes": sum(e["bytes"] for e in entries), "entries": entries}
    # The manifest is the completion marker, as for the database set.
    progress.update(step="manifest", kind=None)
    upload_fn(bucket, prefix + "/manifest.json", (json.dumps(manifest, ensure_ascii=False, indent=2) + "\n").encode(), access_token)
    return manifest


def create_backup(directory, env):
    directory = Path(directory)
    errors_path = directory / "private-errors.log"
    with errors_path.open("w+") as errors:
        snapshot = Snapshot(env, errors)
        try:
            dump = directory / "database.dump"
            with dump.open("wb") as output:
                process = subprocess.Popen(["pg_dump", "--format=custom", "--compress=6", "--lock-wait-timeout=5s", "--snapshot=" + snapshot.info["snapshot"]], env=env, stdout=subprocess.PIPE, stderr=errors)
                try:
                    while chunk := process.stdout.read(1024 * 1024):
                        output.write(chunk)
                        if output.tell() > MAX_BYTES:
                            raise RuntimeError("backup_size_limit_exceeded")
                    if process.wait(timeout=10) != 0:
                        raise RuntimeError("database_dump_failed")
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.wait()
            if subprocess.run(["pg_restore", "--file=/dev/null", str(dump)], stderr=errors, timeout=60).returncode:
                raise RuntimeError("archive_validation_failed")
            roles = directory / "roles.sql"
            with roles.open("wb") as output:
                if subprocess.run(["pg_dumpall", "--roles-only", "--no-role-passwords"], env=env, stdout=output, stderr=errors, timeout=30).returncode:
                    raise RuntimeError("role_backup_failed")
            audit = snapshot.audit()
            manifest = dict(format_version=1, created_at=dt.datetime.now(dt.timezone.utc).isoformat(),
                            server_version=snapshot.info["server_version"], read_only=True,
                            database_bytes=dump.stat().st_size,
                            database_sha256=hashlib.sha256(dump.read_bytes()).hexdigest(),
                            roles_sha256=hashlib.sha256(roles.read_bytes()).hexdigest(), **audit)
            (directory / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
            return manifest
        finally:
            snapshot.close()


def main():
    os.umask(0o077)
    bucket = os.environ["BACKUP_BUCKET"]
    if not re.fullmatch(r"[a-z0-9][a-z0-9._-]{1,220}[a-z0-9]", bucket):
        raise RuntimeError("invalid_bucket_configuration")
    env = pg_env(os.environ["DATABASE_URL"])
    with tempfile.TemporaryDirectory(prefix="tracktalk-backup-") as temp:
        manifest = create_backup(temp, env)
        stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d/%Y%m%dT%H%M%SZ")
        prefix = "database/" + stamp + "-" + uuid.uuid4().hex
        access_token = token()
        for filename in ("database.dump", "roles.sql"):
            upload(bucket, prefix + "/" + filename, (Path(temp) / filename).read_bytes(), access_token)
        # The manifest is the completion marker; incomplete sets are never reported as successful.
        upload(bucket, prefix + "/manifest.json", (Path(temp) / "manifest.json").read_bytes(), access_token)
        print(json.dumps({"severity": "INFO", "event": "database_backup_completed", "prefix": prefix,
                          "bytes": manifest["database_bytes"], "tables": len(manifest["tables"])}, ensure_ascii=False), flush=True)
    if os.environ.get("CLOUDINARY_BACKUP") and media_due(dt.datetime.now(dt.timezone.utc)):
        # Runs after the database set is stored, so a media failure never costs the database backup.
        progress = {"step": "config"}
        try:
            cfg = json.loads(os.environ["CLOUDINARY_BACKUP"])
            media_prefix = "media/" + dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d/%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex
            media = copy_media(cfg, bucket, media_prefix, token(),
                               lambda b, n, d, t: upload(b, n, d, t, limit=MEDIA_MAX_FILE_BYTES), progress)
        except Exception as error:
            print(json.dumps({"severity": "ERROR", "event": "media_backup_failed", **failure_details(error, progress)}), flush=True)
            sys.exit(1)
        print(json.dumps({"severity": "INFO", "event": "media_backup_completed", "prefix": media_prefix,
                          "files": media["files"], "bytes": media["bytes"]}), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        # Do not log exception text: database clients and HTTP errors can contain secrets or data.
        print(json.dumps({"severity": "ERROR", "event": "database_backup_failed", **failure_details(error)}), flush=True)
        sys.exit(1)
