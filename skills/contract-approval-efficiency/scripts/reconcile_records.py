#!/usr/bin/env python3
"""Reconcile discovered BPM task IDs with collected records, without network IO."""
import argparse
from collections import Counter
from datetime import datetime
import json
from pathlib import Path
from zoneinfo import ZoneInfo


def identity(value):
    if value is None:
        return ""
    if isinstance(value, bool) or not isinstance(value, (str, int)):
        raise ValueError("invalid_task_identity")
    return str(value)


def rows(bundle):
    for instance in bundle.get("instances") or []:
        for section in ("done", "todo"):
            for node in instance.get("response", {}).get("data", {}).get(section) or []:
                for record in node.get("records") or []:
                    yield instance, node, record


def index_records(bundle):
    index = {}
    for instance, node, record in rows(bundle):
        key = (identity(record.get("processInstanceId")), identity(record.get("processTaskInstanceId")))
        if all(key):
            index.setdefault(key, []).append((instance, node, record))
    return index


def milliseconds(value, zone):
    if not value:
        return None
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=zone)
    return round(parsed.timestamp() * 1000)


def completed_candidates(bundle, target_ids, start, end, zone):
    result = {}
    for instance, node, record in rows(bundle):
        if node.get("nodeTypeCode") not in (1, 2, 3):
            continue
        if record.get("taskStatusCode") not in (1, 2) or record.get("commandTypeCode") not in (1, 2, 10):
            continue
        assignees = {identity(person.get("employeeId")) for person in record.get("assigneeEmployees") or []}
        if target_ids and not target_ids.intersection(assignees):
            continue
        try:
            finished = milliseconds(record.get("endTime"), zone)
            created = milliseconds(record.get("createTime"), zone)
        except (ValueError, TypeError):
            continue
        if finished is None or created is None or not start <= finished < end or created > finished:
            continue
        key = (identity(record.get("processInstanceId")), identity(record.get("processTaskInstanceId")))
        if not all(key):
            continue
        result[key] = {"processInstanceId": key[0], "processTaskInstanceId": key[1],
            "businessId": identity(record.get("businessId")), "businessTypeCode": record.get("businessTypeCode"),
            "nodeTypeCode": node.get("nodeTypeCode"), "commandTypeCode": record.get("commandTypeCode"),
            "endTime": record.get("endTime"), "duration_seconds": (finished - created) / 1000}
    return result


def reconcile(discovery, bundle, old_bundle=None):
    zone = ZoneInfo(bundle["meta"]["timezone"])
    start, end = (milliseconds(bundle["meta"][key], zone) for key in ("start", "end"))
    index = index_records(bundle)
    source = {}
    window_disagreements = []
    for task in discovery.get("task_records") or []:
        key = (identity(task.get("process_instance_id")), identity(task.get("task_instance_id")))
        finished = task.get("end_time")
        status = task.get("task_status")
        timestamp_valid = isinstance(finished, int) and not isinstance(finished, bool)
        expected = status == 0 or status == 1 and (not timestamp_valid or start <= finished < end)
        if bool(task.get("candidate_in_window_or_pending")) != expected:
            window_disagreements.append({"processInstanceId": key[0], "processTaskInstanceId": key[1],
                "computed_in_window_or_pending": expected,
                "discovery_in_window_or_pending": task.get("candidate_in_window_or_pending")})
        if expected:
            source[key] = task
    missing, mismatches = [], []
    classified = Counter()
    source_status_counts = Counter()
    for key, task in source.items():
        source_status_counts[str(task.get("task_status"))] += 1
        matches = index.get(key) or []
        if not matches:
            failures = [error for error in bundle.get("collection_errors") or []
                        if error.get("processInstanceId") == key[0]
                        or (not error.get("processInstanceId") and identity(error.get("businessId")) == identity(task.get("contract_id")))]
            missing.append({"processInstanceId": key[0], "processTaskInstanceId": key[1],
                "businessId": identity(task.get("contract_id")), "source_task_status": task.get("task_status"),
                "source_create_time": task.get("create_time"), "source_end_time": task.get("end_time"),
                "reason": "collection_failed" if failures else "source_task_absent_from_process_records",
                "collection_errors": failures})
            continue
        _, node, record = matches[0]
        classified[str(node.get("nodeTypeCode"))] += 1
        differences = []
        for source_field, record_field in (("create_time", "createTime"), ("end_time", "endTime")):
            raw = task.get(source_field)
            try:
                actual = milliseconds(record.get(record_field), zone)
            except (ValueError, TypeError):
                actual = None
            if isinstance(raw, int) and not isinstance(raw, bool) and raw > 0 and actual is not None and raw != actual:
                differences.append({"field": record_field, "source_ms": raw, "record_ms": actual})
        if differences:
            mismatches.append({"processInstanceId": key[0], "processTaskInstanceId": key[1], "differences": differences})
    targets = {identity(value) for value in discovery.get("approver_ids") or []}
    current = completed_candidates(bundle, targets, start, end, zone)
    result = {"completed_window": {"start": bundle["meta"]["start"], "end": bundle["meta"]["end"]},
        "source_candidate_task_count": len(source), "source_status_counts": dict(source_status_counts),
        "matched_task_count": len(source) - len(missing), "missing_task_count": len(missing),
        "matched_node_type_counts": dict(classified), "missing_tasks": missing,
        "timestamp_mismatches": mismatches, "source_window_disagreements": window_disagreements,
        "all_source_tasks_reconciled": not missing and not window_disagreements,
        "new_completed_candidate_count": len(current),
        "note": "完成候选比较尚未应用单人证明排除；提交等非审批节点仅核对是否存在，不计入审批效率。"}
    if old_bundle is not None:
        # A later collection cutoff must never turn later approvals into claims
        # that the earlier report omitted them. Compare only the shared window.
        old_start = milliseconds(old_bundle["meta"]["start"], zone)
        old_end = milliseconds(old_bundle["meta"]["end"], zone)
        comparison_start, comparison_end = max(start, old_start), min(end, old_end)
        current_comparable = completed_candidates(bundle, targets, comparison_start, comparison_end, zone)
        previous = completed_candidates(old_bundle, targets, comparison_start, comparison_end, zone)
        added = sorted(set(current_comparable) - set(previous))
        absent = sorted(set(previous) - set(current_comparable))
        result.update(old_comparison_window={"start_ms": comparison_start, "end_ms": comparison_end},
            comparable_new_completed_count=len(current_comparable), old_completed_candidate_count=len(previous),
            newly_recovered_completed_count=len(added),
            newly_recovered_completed=[current_comparable[key] for key in added],
            previous_completed_absent_count=len(absent),
            previous_completed_absent=[previous[key] for key in absent])
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--discovery", type=Path, required=True)
    parser.add_argument("--bundle", type=Path, required=True)
    parser.add_argument("--old-bundle", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    read = lambda path: json.loads(path.read_text(encoding="utf-8"))
    result = reconcile(read(args.discovery), read(args.bundle), read(args.old_bundle) if args.old_bundle else None)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({key: result[key] for key in ("source_candidate_task_count", "matched_task_count", "missing_task_count",
        "all_source_tasks_reconciled", "new_completed_candidate_count")}, ensure_ascii=False))


if __name__ == "__main__":
    main()
