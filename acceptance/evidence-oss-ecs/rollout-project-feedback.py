#!/usr/bin/env python3
"""Auth58-only, Project-only UI rollout; cold dump and empty restore first.

Run as root on the ECS. Private Compose configuration and diagnostics never
leave the host. This is not a Suite or Evidence rollout, or a migration runner.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess

IMAGE = "crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/project-workspace@sha256:b03d2cda182d420e6db6f1c937d6f66512136df49652ff055e3300a2a07140aa"
CONFIG_ID = "sha256:0089b84c81ea84789a4eb434a2d1e24f66ac736e6e6832995d9c4847a5041605"
ROOT = Path("/var/backups/omsbizfoundry/project-ui-test43-20261001")
ENV = Path("/opt/omsbizfoundry/auth58-v0.1.0-test.20-full.https.env")
PROJECT = "omsbizfoundry-project-workspace-1"
PG = "omsbizfoundry-postgres-1"
DB = "project_workspace"
RESTORE_DB = "project_ui_auth58_20261001p43"
RELEASE = "test43"
SERVICES = {"POSTGRES": "postgres", "IDENTITY": "identity-access", "CONTRACT": "contract-ledger",
            "MONEY": "money-flow-ledger", "PEOPLE": "people-groups", "PARTY": "party-directory",
            "PROJECT": "project-workspace", "EVIDENCE": "evidence-service", "BUSINESS_ENGAGEMENT": "business-engagement",
            "BILLING": "billing-invoice", "PRODUCTION": "production-operations", "APPROVAL_AUDIT_RESULTS": "approval-audit-results",
            "COST_OUTSOURCING": "cost-outsourcing", "BONUS_DISTRIBUTION": "bonus-distribution",
            "WORKBENCH": "unified-workbench", "GATEWAY": "gateway", "NGINX": "nginx"}


def run(args, **kwargs):
    return subprocess.run(args, check=True, capture_output=True, timeout=180, **kwargs).stdout


def inspect(name):
    return json.loads(run(["docker", "inspect", name]))[0]


def sql(db, statement):
    return run(["docker", "exec", "-i", PG, "psql", "-X", "-q", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-U", "oms_admin", "-d", db], input=statement.encode()).decode().strip()


def facts(db):
    tables = json.loads(sql(db, "SELECT coalesce(json_agg(tablename ORDER BY tablename),'[]') FROM pg_tables WHERE schemaname='public';"))
    values = []
    for table in tables:
        ident = '"' + table.replace('"', '""') + '"'
        values.append([table, sql(db, "SELECT count(*)::text || ':' || md5(coalesce(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),'')) FROM public." + ident + " t;")])
    sequences = json.loads(sql(db, "SELECT coalesce(json_agg(sequencename ORDER BY sequencename),'[]') FROM pg_sequences WHERE schemaname='public';"))
    for sequence in sequences:
        ident = '"' + sequence.replace('"', '""') + '"'
        values.append(["sequence:" + sequence, sql(db, "SELECT last_value::text || ':' || is_called::text FROM public." + ident + ";")])
    digest = hashlib.sha256(json.dumps(values, separators=(",", ":")).encode()).hexdigest()
    return {"tables": len(tables), "sequences": len(sequences), "canonical_digest": digest}


def private_write(path, content):
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "wb") as target:
        target.write(content)


def replace_images(content, images):
    names = {"D1_IMAGE_MODE": "pull", **{"D1_" + k + "_IMAGE": v for k, v in images.items()}}
    result = []
    for line in content.splitlines():
        key = line.split("=", 1)[0]
        if key in names:
            result.append(key + "=" + names.pop(key))
        else:
            result.append(line)
    result.extend(k + "=" + v for k, v in names.items())
    return ("\n".join(result) + "\n").encode()


def main():
    global IMAGE, CONFIG_ID, ROOT, RESTORE_DB, RELEASE
    requested = os.environ.get("ALLOW_PROJECT_UI_ROLLOUT")
    assert requested in ("test43", "test46", "test47", "test48"), "only explicitly pinned Auth58 Project releases are supported"
    if requested != "test43":
        RELEASE = requested
        digest, config = {
            "test46": ("70e0d10bb531412aef496678cd3ea5d25faf56bf107ce913417b2fbaf267b84d", "1554e5661f1e38fe1132b5711b2c9cc4ec7d2559884749925d42baed543a928d"),
            "test47": ("b2dc8f7fe97ff32743e4c6c59eec5d173e117021a2812d6c10a75059d5308198", "6059d5e925db19f6582fe72195599f5a68741b6eb4cd9e0cd383c097b62157c2"),
            "test48": ("d671251d6cc65e74f11f673d73f1cc57a9ae40f59a46599e9c75311c70dfdd6e", "3cc681e057e813fdce922524447c60afaf36c8e0cb4fdd0cbd16376b676d4385"),
        }[requested]
        IMAGE = "crpi-bin16egpr6fh7gxw.cn-beijing.personal.cr.aliyuncs.com/omsbizfoundry/project-workspace@sha256:" + digest
        CONFIG_ID = "sha256:" + config
        ROOT = Path("/var/backups/omsbizfoundry/project-ui-" + requested + "-20261001")
        RESTORE_DB = "project_ui_auth58_20261001p" + requested.removeprefix("test")
    assert os.geteuid() == 0 and ENV.stat().st_mode & 0o077 == 0
    original = ENV.read_bytes()
    current = {key: inspect("omsbizfoundry-" + service + "-1") for key, service in SERVICES.items()}
    assert all(c["State"].get("Health", {}).get("Status") == "healthy" for c in current.values())
    assert dict(v.split("=", 1) for v in current["EVIDENCE"]["Config"]["Env"]).get("STORAGE_PROVIDER") == "local"
    labels = current["PROJECT"]["Config"]["Labels"]
    cwd = labels["com.docker.compose.project.working_dir"]
    assert cwd == "/opt/omsbizfoundry/releases/omsbizfoundry-d1_v0.1.0-test.20_linux_amd64/deployments/d1"
    assert labels["com.docker.compose.project.environment_file"] == str(ENV)
    files = labels["com.docker.compose.project.config_files"].split(",")
    assert all(Path(f).is_file() and str(Path(f).parent) == cwd for f in files)
    images = {}
    for key, container in current.items():
        image = json.loads(run(["docker", "image", "inspect", container["Image"]]))[0]
        refs = [ref for ref in image.get("RepoDigests", []) if ".aliyuncs.com/omsbizfoundry/" in ref]
        assert len(refs) == 1
        images[key] = refs[0]
    assert images["PROJECT"] != IMAGE
    staged = {"original.env": original, "rollback.env": replace_images(original.decode(), images),
              "candidate.env": replace_images(original.decode(), {**images, "PROJECT": IMAGE})}
    if ROOT.exists():
        assert ROOT.stat().st_mode & 0o077 == 0 and set(f.name for f in ROOT.iterdir()) == set(staged), "refuse to replace a previous rollout or backup"
        assert all((ROOT / name).read_bytes() == content and (ROOT / name).stat().st_mode & 0o077 == 0 for name, content in staged.items()), "only an unchanged preflight may resume"
    else:
        ROOT.mkdir(mode=0o700)
        for name, content in staged.items():
            private_write(ROOT / name, content)

    def compose(environment, *command):
        args = ["docker", "compose", "--project-directory", cwd, "-p", "omsbizfoundry", "--env-file", str(environment)]
        for f in files:
            args += ["-f", f]
        return run(args + ["--profile", "*"] + list(command))

    rendered = json.loads(compose(ROOT / "candidate.env", "config", "--format", "json"))["services"]["project-workspace"]
    assert rendered["image"] == IMAGE and not rendered.get("ports")
    actual_env = dict(v.split("=", 1) for v in current["PROJECT"]["Config"]["Env"])
    assert all(str(value) == actual_env.get(key) for key, value in rendered["environment"].items()), "refuse non-image runtime configuration changes"
    run(["docker", "pull", IMAGE])
    candidate = json.loads(run(["docker", "image", "inspect", IMAGE]))[0]
    # Docker's containerd image store reports the index descriptor as Id;
    # the classic store reports the selected platform configuration digest.
    assert candidate["Id"] in (CONFIG_ID, IMAGE.split("@", 1)[1]) and IMAGE in candidate["RepoDigests"]
    assert candidate["Architecture"] == "amd64" and candidate["Os"] == "linux"
    assert candidate["Config"]["User"] == "postgres"
    print(json.dumps({"phase": "preflight", "result": "pass", "changed_service": "Project UI only"}), flush=True)
    stopped = False
    updated = False
    environment_updated = False
    try:
        run(["docker", "stop", "--time", "30", PROJECT])
        stopped = True
        before = facts(DB)
        owner = sql("postgres", "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='project_workspace';")
        assert owner.replace("_", "").isalnum()
        assert not sql("postgres", "SELECT 1 FROM pg_database WHERE datname='" + RESTORE_DB + "';")
        dump = ROOT / "project-workspace.dump"
        descriptor = os.open(dump, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "wb") as target:
            result = subprocess.run(["docker", "exec", PG, "pg_dump", "-U", "oms_admin", "-d", DB, "-Fc", "--no-owner", "--no-privileges"], stdout=target, stderr=subprocess.PIPE, timeout=120)
            assert result.returncode == 0
        sql("postgres", 'CREATE DATABASE "' + RESTORE_DB + '" OWNER "' + owner + '";')
        with dump.open("rb") as source:
            restored = subprocess.run(["docker", "exec", "-i", PG, "pg_restore", "-U", "oms_admin", "-d", RESTORE_DB, "--single-transaction", "--exit-on-error", "--no-owner", "--no-privileges"], stdin=source, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=120)
            assert restored.returncode == 0
        assert facts(RESTORE_DB) == before and facts(DB) == before
        backup_sha = hashlib.sha256(dump.read_bytes()).hexdigest()
        private_write(ROOT / "recovery-evidence.json", json.dumps({"result": "pass", "source": DB, "empty_restore": RESTORE_DB, "facts": before, "backup_sha256": backup_sha}).encode())
        print(json.dumps({"phase": "cold_backup_empty_restore", "result": "pass", "facts": before, "backup_sha256": backup_sha}), flush=True)
        assert ENV.read_bytes() == original, "authoritative environment changed concurrently"
        temporary = ENV.with_name(ENV.name + "." + RELEASE + ".tmp")
        private_write(temporary, (ROOT / "candidate.env").read_bytes())
        os.replace(temporary, ENV)
        environment_updated = True
        compose(ENV, "up", "-d", "--no-deps", "--no-build", "--pull", "never", "--wait", "--wait-timeout", "90", "project-workspace")
        updated = True
        new = inspect(PROJECT)
        assert new["Image"] == candidate["Id"] and new["State"]["Health"]["Status"] == "healthy"
        run(["docker", "exec", PROJECT, "/app/ops", "check"])
        assert facts(DB) == before
        assert all(inspect("omsbizfoundry-" + SERVICES[key] + "-1")["Id"] == old["Id"] for key, old in current.items() if key != "PROJECT")
        assert ENV.read_bytes() == (ROOT / "candidate.env").read_bytes()
        assert all(inspect("omsbizfoundry-" + service + "-1")["State"].get("Health", {}).get("Status") == "healthy" for service in SERVICES.values())
        result = {"result": "pass", "rollout": "Project UI only", "project_image": IMAGE, "database_migration": False, "facts_unchanged": True, "other_16_containers_unchanged": True, "evidence_provider": "local", "backup_sha256": backup_sha, "facts": before}
        private_write(ROOT / "rollout-evidence.json", json.dumps(result).encode())
        print(json.dumps(result), flush=True)
    except Exception:
        if environment_updated:
            assert ENV.read_bytes() == (ROOT / "candidate.env").read_bytes(), "refuse to overwrite a concurrent environment change"
            temporary = ENV.with_name(ENV.name + "." + RELEASE + ".rollback.tmp")
            private_write(temporary, original)
            os.replace(temporary, ENV)
        if stopped:
            compose(ROOT / "rollback.env", "up", "-d", "--no-deps", "--no-build", "--pull", "never", "--wait", "--wait-timeout", "90", "project-workspace")
        # Never restore a database over potentially newer confirmed facts.
        print(json.dumps({"result": "fail", "rollback_attempted": stopped, "new_image_started": updated, "backup_retained": True, "private_diagnostics": "suppressed"}), flush=True)
        raise SystemExit(1)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        raise SystemExit("Project-only preflight failed; private diagnostics suppressed")
