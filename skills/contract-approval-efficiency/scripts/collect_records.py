#!/usr/bin/env python3
"""Collect read-only CLM workflow records into an analyzer bundle.

Discovery is a candidate inventory, not proof of historical coverage. All task
records are retained before person/date filtering. Errors, incomplete discovery,
and historical instance identities remain inspectable in the generated bundle.
"""
import argparse
from collections import Counter
import concurrent.futures
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time
from zoneinfo import ZoneInfo


PREFIX = "/open-web/web/clm/"
NORMAL_TYPES = {0, 1, 2, 3, 9, 11, 12, 16}
ASSEMBLY_TYPES = {10, 17}
CACHE_VERSION = 2
KEEP = {
    "done", "todo", "forecast", "contractId", "contractAssemblyId", "assemblyId",
    "contractName", "contractNumber", "businessId", "businessTypeCode",
    "processInstanceId", "processDefinitionId", "processDefinitionKey", "processName",
    "approveRecord", "records", "nodeId", "nodeName", "nodeDisplayName", "nodeTypeCode",
    "taskModeCode", "processTaskInstanceId", "parentProcessTaskInstanceId",
    "parentCommandTypeCode", "createTime", "endTime", "taskStatusCode", "commandTypeCode",
    "assigneeEmployees", "agentEmployees", "endorsementEmployees", "employeeId", "name", "displayName", "departments",
    "departmentId", "parentDepartmentId", "status",
}


def identifier(value):
    if isinstance(value, bool) or not isinstance(value, (str, int)) or not str(value).strip():
        raise ValueError("missing_or_invalid_internal_id")
    return str(value)


def code(value):
    if isinstance(value, bool):
        raise ValueError("invalid_business_type")
    if isinstance(value, int) or isinstance(value, str) and value.lstrip("-").isdigit():
        return int(value)
    raise ValueError("invalid_business_type")


def clean(value):
    """Keep task identities/participants/times, never bodies, files or credentials."""
    if isinstance(value, list):
        return [clean(item) for item in value]
    if isinstance(value, dict):
        return {key: clean(item) for key, item in value.items() if key in KEEP}
    return value


def utc_now():
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def write_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def cli_caller(binary, profile, attempts=3):
    if profile != "contract-test":
        raise ValueError("only_verified_test_profile_is_supported")

    def call(path, query):
        args = [binary, "api", "call", "--channel", "web", "--profile", profile,
                "--as", "user", "--output", "json", "GET", PREFIX + path,
                "--header", "Timezone-Offset: -480"]
        for key, value in query.items():
            args.extend(["--query", f"{key}={value}"])
        for attempt in range(attempts):
            retry = False
            try:
                result = subprocess.run(args, capture_output=True, text=True, timeout=90)
            except subprocess.TimeoutExpired:
                envelope = {"collection_error": "timeout"}
                retry = True
            except OSError as exc:
                return {"collection_error": type(exc).__name__}
            else:
                try:
                    envelope = json.loads(result.stdout)
                    if not isinstance(envelope, dict):
                        raise ValueError("not_object")
                except ValueError as exc:
                    envelope = {"collection_error": "invalid_json", "cli_exit_code": result.returncode,
                                "stdout_bytes": len(result.stdout.encode("utf-8")),
                                "json_error_position": getattr(exc, "pos", None)}
                    # A malformed read response can be transient (large CLI
                    # outputs); retry only within this bounded read-only call.
                    retry = True
                else:
                    envelope["cli_exit_code"] = result.returncode
                    retry = envelope.get("code") in (429, 503)
                    if result.returncode:
                        envelope["collection_error"] = "cli_failure"
                # Do not echo stdout/stderr. Authentication belongs to the CLI.
            if not retry or attempt + 1 == attempts:
                return envelope
            time.sleep(min(1.5 * (2 ** attempt), 6))
        raise AssertionError("unreachable")
    return call


def successful(envelope):
    return (isinstance(envelope, dict) and envelope.get("cli_exit_code", 0) == 0
            and not envelope.get("collection_error") and envelope.get("success") is True
            and envelope.get("code") == 200 and isinstance(envelope.get("data"), dict))


def valid_records(data):
    if not isinstance(data, dict) or not any(key in data for key in ("done", "todo")):
        return False
    for section in ("done", "todo", "forecast"):
        nodes = data.get(section)
        if nodes is None:
            continue
        if not isinstance(nodes, list):
            return False
        for node in nodes:
            if not isinstance(node, dict):
                return False
            records = node.get("records")
            if records is not None and (not isinstance(records, list)
                                        or any(not isinstance(row, dict) for row in records)):
                return False
    return True


def record_rows(data):
    for section in ("done", "todo"):
        for node in data.get(section) or []:
            for row in node.get("records") or []:
                yield row


def normalize_business(item):
    if not isinstance(item, dict):
        raise ValueError("business_must_be_object")
    result = clean(item)
    result["businessId"] = identifier(item.get("businessId") or item.get("contractId") or item.get("contract_id"))
    result["businessTypeCode"] = code(item["businessTypeCode"]) if item.get("businessTypeCode") is not None else None
    ids = []
    explicit_pid = item.get("processInstanceId") or item.get("process_instance_id")
    for raw in ([explicit_pid] if explicit_pid else []) + (item.get("processInstanceIds") or []):
        pid = identifier(raw)
        if pid not in ids:
            ids.append(pid)
    if ids:
        result["processInstanceIds"] = ids
    if result["businessTypeCode"] is None and not ids:
        raise ValueError("business_type_or_explicit_process_instance_required")
    return result


def definition_metadata(data):
    """Current definition evidence stays bound to its own current instance."""
    result = {}
    for key in ("formId", "processInstanceId", "processDefinitionId"):
        if data.get(key):
            result[key] = identifier(data[key])
    submitter = data.get("submitterEmployee")
    if isinstance(submitter, dict) and submitter.get("employeeId"):
        result["submitterEmployee"] = {"employeeId": identifier(submitter["employeeId"])}
    config = data.get("nodeConfigMap")
    if isinstance(config, dict):
        result["nodeConfigMap"] = {
            key: {"dynamicApproverConfig": None if value.get("dynamicApproverConfig") is None else {"configured": True}}
            for key, value in config.items() if isinstance(value, dict) and "dynamicApproverConfig" in value
        }
    return result


def normalize_evidence(raw):
    if isinstance(raw, dict):
        raw = raw.get("node_participant_evidence")
    if not isinstance(raw, list):
        raise ValueError("participant_evidence_must_be_array")
    result = {}
    for row in raw:
        if not isinstance(row, dict):
            raise ValueError("participant_evidence_must_contain_objects")
        key = tuple(identifier(row.get(k)) for k in ("processInstanceId", "processDefinitionId", "nodeId"))
        ids = sorted({identifier(x) for x in row.get("employee_ids") or []})
        if not ids or row.get("complete") is True and not str(row.get("source") or "").strip():
            raise ValueError("participant_evidence_requires_ids_and_source")
        evidence = dict(zip(("processInstanceId", "processDefinitionId", "nodeId"), key))
        evidence.update(employee_ids=ids, complete=row.get("complete") is True,
                        source=str(row.get("source") or ""))
        if key in result and result[key] != evidence:
            raise ValueError("conflicting_node_participant_evidence")
        result[key] = evidence
    return list(result.values())


class Collector:
    def __init__(self, call, cache_dir=None, cache_max_age=0, profile="contract-test", with_definition_metadata=False):
        self.call = call
        self.cache_dir = Path(cache_dir) if cache_dir else None
        self.cache_max_age = cache_max_age
        self.profile = profile
        self.with_definition_metadata = with_definition_metadata

    @staticmethod
    def failure(item, route, envelope=None, reason=None, process_id=None):
        envelope = envelope or {}
        if (reason is None and not envelope.get("collection_error") and envelope.get("success") is True
                and envelope.get("code") == 200 and not isinstance(envelope.get("data"), dict)):
            reason = "invalid_response_data"
        value = {"businessId": item["businessId"], "businessTypeCode": item["businessTypeCode"],
                 "route": route, "error": reason or envelope.get("collection_error") or "business_failure",
                 "code": envelope.get("code")}
        if "success" in envelope:
            value["source_success"] = envelope["success"]
            value["data_type"] = type(envelope.get("data")).__name__
        for key in ("cli_exit_code", "stdout_bytes", "json_error_position"):
            if key in envelope:
                value[key] = envelope[key]
        if process_id:
            value["processInstanceId"] = process_id
        return value

    def records(self, item, process_id):
        query = {"processInstanceId": process_id}
        # A task inventory can provide only a genuine process ID. The endpoint
        # checks read permission on that process and returns each task's real
        # business identity. Never manufacture type=0 from a contract-looking ID.
        if item.get("businessTypeCode") is not None:
            query.update(businessId=item["businessId"], businessTypeCode=item["businessTypeCode"])
        return self.call("workflow/runtime/process/records", query)

    def instance(self, item, detail_data, process_id, response, route):
        data = response.get("data")
        if not successful(response) or not valid_records(data):
            return None, self.failure(item, route, response,
                reason="invalid_records_response" if successful(response) else None,
                process_id=process_id)
        represented = sorted({identifier(row["processInstanceId"]) for row in record_rows(data)
                              if row.get("processInstanceId")})
        business_type = item.get("businessTypeCode")
        if business_type is None:
            observed_types = {code(row["businessTypeCode"]) for row in record_rows(data)
                              if row.get("businessId") is not None
                              and identifier(row["businessId"]) == item["businessId"]
                              and row.get("businessTypeCode") is not None}
            if len(observed_types) == 1:
                business_type = observed_types.pop()
        envelope = {"success": True, "code": 200, "data": clean(data)}
        if response.get("requestId"):
            envelope["requestId"] = response["requestId"]
        result = {"businessId": item["businessId"], "businessTypeCode": business_type,
                  "processInstanceId": process_id,
                  "contractName": detail_data.get("contractName") or item.get("contractName") or "",
                  "contractNumber": detail_data.get("contractNumber") or item.get("contractNumber") or "",
                  "source_recipe": route, "response": envelope,
                  "represented_process_instance_ids": represented,
                  "definition_metadata": definition_metadata(detail_data)}
        return result, None

    def detail(self, item):
        kind = item["businessTypeCode"]
        if kind in NORMAL_TYPES:
            route = "contract_info"
            response = self.call("contract/info", {"contractId": item["businessId"],
                "tianyancha": "false", "generateContractName": "false"})
            # Permission failures never trigger another endpoint or identity.
            if response.get("code") == 110002 and not response.get("collection_error"):
                route = "contract_brief_info"
                response = self.call("contract/briefInfo", {"contractId": item["businessId"]})
        elif kind in ASSEMBLY_TYPES:
            route = "assembly_info"
            response = self.call("contractAssembly/info", {
                "contractAssemblyId": item["businessId"], "needChildrenInfo": "false"})
        else:
            return [], [self.failure(item, "unsupported", reason="unsupported_business_type")]
        if not successful(response):
            return [], [self.failure(item, route, response)]
        data = response["data"]
        if data.get("businessTypeCode") is not None and code(data["businessTypeCode"]) != kind:
            return [], [self.failure(item, route, response, "business_type_mismatch")]
        returned_id = data.get("contractAssemblyId") if kind in ASSEMBLY_TYPES else data.get("contractId")
        if returned_id is not None and identifier(returned_id) != item["businessId"]:
            return [], [self.failure(item, route, response, "business_id_mismatch")]
        pid = identifier(data["processInstanceId"]) if data.get("processInstanceId") else ""
        if valid_records(data.get("approveRecord")):
            # approveRecord can contain historical rows even without a current instance.
            records_response = {"success": response["success"], "code": response["code"],
                                "data": data["approveRecord"], "requestId": response.get("requestId")}
        elif pid:
            route = "process_records"
            records_response = self.records(item, pid)
        else:
            return [], [self.failure(item, route, response, "missing_workflow_evidence")]
        instance, error = self.instance(item, data, pid, records_response, route)
        return ([instance] if instance else []), ([error] if error else [])

    def enrich_metadata(self, item, instances):
        """Optional business context for definition verification, fetched once.

        The existing record responses remain authoritative. A metadata failure
        is recorded separately and never erases successfully collected tasks.
        """
        typed = {row.get("businessTypeCode") for row in instances if row.get("businessTypeCode") is not None}
        if len(typed) != 1:
            return [self.failure(item, "definition_metadata", reason="metadata_business_type_unresolved")]
        kind = next(iter(typed))
        if kind in NORMAL_TYPES:
            path = "contract/info"
            query = {"contractId": item["businessId"], "tianyancha": "false", "generateContractName": "false"}
        elif kind in ASSEMBLY_TYPES:
            path = "contractAssembly/info"
            query = {"contractAssemblyId": item["businessId"], "needChildrenInfo": "false"}
        else:
            return [self.failure(item, "definition_metadata", reason="unsupported_metadata_business_type")]
        response = self.call(path, query)
        if not successful(response):
            return [self.failure(item, "definition_metadata", response)]
        data = response["data"]
        bid = data.get("contractAssemblyId") if kind in ASSEMBLY_TYPES else data.get("contractId")
        if bid is not None and identifier(bid) != item["businessId"]:
            return [self.failure(item, "definition_metadata", reason="business_id_mismatch")]
        if data.get("businessTypeCode") is not None and code(data["businessTypeCode"]) != kind:
            return [self.failure(item, "definition_metadata", reason="business_type_mismatch")]
        metadata = definition_metadata(data)
        for instance in instances:
            # The explicit current-process ID in metadata prevents use as proof
            # of the definition/submitter for a different historical instance.
            instance["definition_metadata"] = metadata
            for field in ("contractName", "contractNumber"):
                if data.get(field):
                    instance[field] = data[field]
        return []

    def collect_business(self, raw):
        item = normalize_business(raw)
        fingerprint = hashlib.sha256(json.dumps({"profile": self.profile,
            "businessId": item["businessId"], "businessTypeCode": item["businessTypeCode"],
            "processInstanceIds": item.get("processInstanceIds", []),
            "with_definition_metadata": self.with_definition_metadata}, sort_keys=True).encode()).hexdigest()
        cache_file = self.cache_dir / f"business-{item['businessTypeCode']}-{item['businessId']}.json" if self.cache_dir else None
        if cache_file and cache_file.exists() and self.cache_max_age > 0:
            try:
                cached = json.loads(cache_file.read_text(encoding="utf-8"))
                age = (datetime.now(timezone.utc) - datetime.fromisoformat(cached["collected_at"])).total_seconds()
                if (cached.get("cache_version") == CACHE_VERSION and cached.get("fingerprint") == fingerprint
                        and not cached.get("errors") and 0 <= age <= self.cache_max_age):
                    cached["cache_reused"] = True
                    return cached
            except (ValueError, TypeError, KeyError):
                pass
        instances, errors = [], []
        process_ids = item.get("processInstanceIds") or []
        if process_ids:
            for pid in process_ids:
                response = self.records(item, pid)
                instance, error = self.instance(item, {}, pid, response, "process_records_explicit")
                if instance:
                    instances.append(instance)
                if error:
                    errors.append(error)
            if instances and self.with_definition_metadata:
                errors.extend(self.enrich_metadata(item, instances))
        else:
            instances, errors = self.detail(item)
        result = {"cache_version": CACHE_VERSION, "fingerprint": fingerprint,
                  "collected_at": utc_now(), "cache_reused": False, "business": item,
                  "instances": instances, "errors": errors,
                  "status": "partial" if errors else "complete"}
        if cache_file:
            write_json(cache_file, result)
        return result

    def run(self, discovery, args, external_evidence=()):
        if discovery.get("profile", args.profile) != args.profile:
            raise ValueError("discovery_profile_mismatch")
        if discovery.get("status") not in {"complete", "partial", "resolved"}:
            raise ValueError("discovery_roster_not_resolved")
        businesses = {}
        for raw in discovery.get("businesses") or []:
            row = normalize_business(raw)
            key = (row["businessTypeCode"], row["businessId"])
            if key in businesses:
                old = businesses[key]
                old["processInstanceIds"] = sorted(set(old.get("processInstanceIds", [])) | set(row.get("processInstanceIds", [])))
            else:
                businesses[key] = row
        for raw in discovery.get("process_instances") or []:
            row = normalize_business(raw)
            if not row.get("processInstanceIds"):
                raise ValueError("explicit_process_instance_missing_id")
            key = (row["businessTypeCode"], row["businessId"])
            if key in businesses:
                businesses[key]["processInstanceIds"] = sorted(set(businesses[key].get("processInstanceIds", [])) | set(row["processInstanceIds"]))
            else:
                businesses[key] = row
        items = list(businesses.values())
        results = []
        with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
            for index, result in enumerate(pool.map(self.collect_business, items), 1):
                results.append(result)
                if index % 50 == 0 or index == len(items):
                    print(json.dumps({"processed": index, "total": len(items),
                        "readable_businesses": sum(bool(r["instances"]) for r in results),
                        "businesses_with_errors": sum(bool(r["errors"]) for r in results)}, ensure_ascii=False), flush=True)
        instances = [instance for result in results for instance in result["instances"]]
        errors = [error for result in results for error in result["errors"]]
        route_counts = dict(Counter(instance["source_recipe"] for instance in instances))
        error_counts = dict(Counter(error["error"] for error in errors))
        subject = args.analysis_subject or {"employee": "people", "department": "department"}.get(discovery.get("scope", {}).get("kind"), "other")
        discovery_complete = discovery.get("status") == "complete"
        source_coverage = discovery.get("coverage") or {}
        readable_count = sum(bool(r["instances"]) for r in results)
        coverage_text = (f"test；候选业务{len(items)}份，取得流程记录{readable_count}份，"
            f"采集有缺口{sum(bool(r['errors']) for r in results)}份；发现状态{discovery.get('status')}。"
            "按任务完成时间筛选，在途为采集时快照；分页完成不代表历史审批记录全集，"
            "覆盖范围以 discovery_coverage 和 collection_errors 为准。")
        evidence = normalize_evidence([*(discovery.get("node_participant_evidence") or []), *external_evidence])
        now = datetime.now(ZoneInfo("Asia/Shanghai")).isoformat(timespec="seconds")
        scope = discovery.get("scope") or "当前授权账号可读取、符合筛选条件的目标对象流程"
        outcomes = [{"businessId": result["business"]["businessId"],
                     "businessTypeCode": result["business"]["businessTypeCode"],
                     "status": result["status"], "instance_count": len(result["instances"]),
                     "cache_reused": result["cache_reused"], "collected_at": result["collected_at"],
                     "errors": result["errors"]} for result in results]
        bundle = {
            "meta": {"start": args.start, "end": args.end, "timezone": "Asia/Shanghai",
                "timezone_confirmed": True,
                "timezone_source": "test contract_info/process_records 已验证 Timezone-Offset=-480；所有请求显式传此头",
                "analysis_subject": subject, "environment": "test", "collected_at": now,
                "scope": scope,
                "coverage": coverage_text, "discovery_status": discovery.get("status"),
                "discovery_coverage": source_coverage, "discovery_diagnostics": discovery.get("diagnostics") or [],
                "collection_status": "complete" if discovery_complete and not errors else "partial",
                "collection_route_counts": route_counts, "collection_error_counts": error_counts,
                "readable_business_count": readable_count, "candidate_business_count": len(items),
                "source": "CLI test/user：发现候选→详情或明确实例→原始流程记录；未预筛任务"},
            "employees": discovery.get("employees") or [], "approver_ids": discovery.get("approver_ids") or [],
            "task_records": discovery.get("task_records") or [],
            "businesses": items, "node_participant_evidence": evidence, "instances": instances,
            "collection_errors": errors,
        }
        return bundle, outcomes


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--discovery", required=True, type=Path)
    parser.add_argument("--start", required=True, help="含时区的完成时间下界（含）")
    parser.add_argument("--end", required=True, help="含时区的完成时间上界（不含）")
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--cache-dir", type=Path)
    parser.add_argument("--cache-max-age", type=float, default=0,
                        help="仅成功采集可在指定秒数内复用；0（默认）每次刷新；失败从不复用")
    parser.add_argument("--profile", choices=["contract-test"], default="contract-test")
    parser.add_argument("--cli", default="contract-cli")
    parser.add_argument("--workers", type=int, default=8)
    parser.add_argument("--analysis-subject", choices=["people", "department", "other"])
    parser.add_argument("--participant-evidence", action="append", type=Path, default=[])
    parser.add_argument("--with-definition-metadata", action="store_true",
                        help="明确实例取记录后，额外读取最小合同表单/发起人上下文用于参与人数审计及缺原始任务时的单人回退证明")
    args = parser.parse_args(argv)
    try:
        bounds = [datetime.fromisoformat(value.replace("Z", "+00:00")) for value in (args.start, args.end)]
    except ValueError:
        parser.error("start/end 必须是含时区的 ISO 日期时间")
    if any(value.utcoffset() is None for value in bounds) or bounds[0] >= bounds[1]:
        parser.error("start/end 必须含时区且 start < end")
    if not 1 <= args.workers <= 16 or args.cache_max_age < 0:
        parser.error("workers 必须为1至16，cache-max-age不能为负")
    return args


def main(argv=None):
    args = parse_args(argv)
    discovery = json.loads(args.discovery.read_text(encoding="utf-8"))
    evidence = []
    for path in args.participant_evidence:
        evidence.extend(normalize_evidence(json.loads(path.read_text(encoding="utf-8"))))
    collector = Collector(cli_caller(args.cli, args.profile), args.cache_dir or args.output_dir / "raw",
                          args.cache_max_age, args.profile, args.with_definition_metadata)
    bundle, outcomes = collector.run(discovery, args, evidence)
    write_json(args.output_dir / "bundle.json", bundle)
    write_json(args.output_dir / "collection-outcomes.json", outcomes)
    print(json.dumps({"bundle": str(args.output_dir / "bundle.json"),
        "status": bundle["meta"]["collection_status"], "instances": len(bundle["instances"]),
        "errors": bundle["meta"]["collection_error_counts"]}, ensure_ascii=False))
    return 0 if bundle["meta"]["collection_status"] == "complete" else 2


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError) as exc:
        print(json.dumps({"error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        sys.exit(1)
