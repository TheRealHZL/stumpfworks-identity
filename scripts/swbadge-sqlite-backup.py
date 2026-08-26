#!/usr/bin/env python3
import os
import sqlite3
import sys
import time
from pathlib import Path


def main() -> int:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <database> <backup-directory>", file=sys.stderr)
        return 2

    source = Path(sys.argv[1])
    if not source.is_file():
        print(f"CRITICAL: database is not a readable file: {source}", file=sys.stderr)
        return 1
    source = source.resolve(strict=True)
    destination = Path(sys.argv[2])
    destination.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(destination, 0o700)
    os.umask(0o077)

    stamp = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
    final = destination / f"badges-{stamp}.db"
    temporary = destination / f".{final.name}.tmp"

    try:
        source_db = sqlite3.connect(f"file:{source}?mode=ro", uri=True, timeout=10)
        backup_db = sqlite3.connect(temporary, timeout=10)
        with backup_db:
            source_db.backup(backup_db)
        source_db.close()
        result = backup_db.execute("PRAGMA integrity_check").fetchone()
        backup_db.close()
        if result is None or result[0] != "ok":
            raise RuntimeError("backup integrity check failed")
        os.chmod(temporary, 0o600)
        os.replace(temporary, final)
        print(f"OK: SQLite backup created: {final}")
        return 0
    except Exception as error:
        temporary.unlink(missing_ok=True)
        print(f"CRITICAL: SQLite backup failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
