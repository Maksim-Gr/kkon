"""Check snapshot archives and smoke-test the native Linux release binary."""

import datetime
import json
import pathlib
import subprocess
import tarfile
import tempfile
import zipfile


def main():
    dist = pathlib.Path("dist")
    metadata = json.loads((dist / "metadata.json").read_text())
    version = metadata["version"]
    expected = {
        f"kkon_{version}_{platform}_{arch}.{extension}": platform
        for platform, arch, extension in [
            ("linux", "amd64", "tar.gz"),
            ("linux", "arm64", "tar.gz"),
            ("darwin", "amd64", "tar.gz"),
            ("darwin", "arm64", "tar.gz"),
            ("windows", "amd64", "zip"),
        ]
    }
    archives = {path.name for path in dist.glob("*.tar.gz")} | {
        path.name for path in dist.glob("*.zip")
    }
    if archives != expected.keys():
        raise RuntimeError(f"Unexpected archive set: {sorted(archives)}")
    for name, platform in expected.items():
        if platform == "windows":
            with zipfile.ZipFile(dist / name) as archive:
                names = archive.namelist()
        else:
            with tarfile.open(dist / name) as archive:
                names = archive.getnames()
        binary = "kkon.exe" if platform == "windows" else "kkon"
        if not {"README.md", binary}.issubset(names):
            raise RuntimeError(f"Missing README or binary in {name}: {names}")

    # Execute the packaged binary, rather than a separate development build.
    with tempfile.TemporaryDirectory() as directory:
        with tarfile.open(dist / f"kkon_{version}_linux_amd64.tar.gz") as archive:
            archive.extract("kkon", directory, filter="data")
        output = subprocess.check_output(
            [str(pathlib.Path(directory) / "kkon"), "version"], text=True
        )
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    if f"kkon {version}\n" not in output or f"commit: {commit}\n" not in output:
        raise RuntimeError(f"Incorrect release metadata: {output}")
    date = datetime.datetime.fromisoformat(metadata["date"].replace("Z", "+00:00"))
    build_date = date.astimezone(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    if f"built:  {build_date}\n" not in output:
        raise RuntimeError(f"Incorrect build date: {output}")
    print("Verified five archives, packaged binary, and injected build metadata.")


if __name__ == "__main__":
    main()
