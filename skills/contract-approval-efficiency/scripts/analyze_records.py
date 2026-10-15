#!/usr/bin/env python3
"""Analyze original CLM process/records responses without making network calls.

Input: {meta: {start, end, timezone, timezone_confirmed: true, coverage},
instances: [{businessId, businessTypeCode, processInstanceId, processName?,
response: {success: true, code: 200, data: {done: [...], todo: [...]}}}]}.
Dates use a half-open completion window [start, end). Naive API timestamps
are interpreted only in the explicitly confirmed meta.timezone.
"""
import argparse
from collections import Counter
from datetime import datetime
import html
import json
from pathlib import Path
import sys
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError


NODE_TYPES = {1: "审批", 2: "盖章", 3: "归档"}
HANDLED_COMMANDS = {1, 2, 10}
ANALYSIS_SUBJECTS = {"people", "department", "other"}


def identity(value):
    """Never accept floating-point IDs, which may already have lost precision."""
    if isinstance(value, bool) or isinstance(value, float):
        raise ValueError("ID 必须为字符串或无精度损失的整数，不能为浮点数/布尔值")
    if value is None:
        return ""
    if not isinstance(value, (str, int)):
        raise ValueError("ID 必须为字符串或整数")
    return str(value)


def integer_code(value):
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, str) and value.strip().lstrip("-").isdigit():
        return int(value)
    return None


def timestamp(value, timezone):
    if not isinstance(value, str) or not value.strip():
        raise ValueError("时间必须为非空日期/时间字符串")
    parsed = datetime.fromisoformat(value.strip().replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone)
    return parsed.astimezone(timezone)


def display_name(obj, fallback):
    return str(obj.get("displayName") or obj.get("name") or fallback)


def optional_array(obj, key):
    value = obj.get(key)
    if value is None:
        return []
    if not isinstance(value, list):
        raise ValueError(f"{key} 必须为数组或 null")
    return value


def metric_fingerprint(record, node_type, node_id, process_id):
    """Compare metrics, not volatile UI fields such as elapsed-since-now spans."""
    assignees = optional_array(record, "assigneeEmployees")
    people = set()
    for employee in assignees:
        if not isinstance(employee, dict):
            raise ValueError("assigneeEmployees 必须为员工对象数组")
        department_ids = set()
        for department in optional_array(employee, "departments"):
            if not isinstance(department, dict):
                raise ValueError("员工 departments 必须为部门对象数组")
            department_ids.add(identity(department.get("departmentId")))
        people.add((identity(employee.get("employeeId")), tuple(sorted(department_ids))))
    return (process_id, identity(record.get("processTaskInstanceId")),
            identity(record.get("processDefinitionId")), node_id, node_type,
            integer_code(record.get("taskStatusCode")),
            integer_code(record.get("commandTypeCode")),
            record.get("createTime"), record.get("endTime"), tuple(sorted(people)))


def empty_bucket(details):
    return dict(details, completed_count=0, approved_count=0, rejected_count=0,
                in_progress_count=0, unassigned_count=0, total_seconds=0.0,
                min_seconds=None, max_seconds=None, _completed_contracts=set(),
                _pending_contracts=set())


def add_sample(bucket, duration, rejected, pending, unattributed, business_key=None):
    if pending:
        bucket["in_progress_count"] += 1
        if business_key:
            bucket["_pending_contracts"].add(business_key)
        return
    bucket["completed_count"] += 1
    bucket["rejected_count"] += int(rejected)
    bucket["approved_count"] += int(not rejected)
    bucket["unassigned_count"] += int(unattributed)
    if business_key:
        bucket["_completed_contracts"].add(business_key)
    bucket["total_seconds"] += duration
    bucket["min_seconds"] = duration if bucket["min_seconds"] is None else min(bucket["min_seconds"], duration)
    bucket["max_seconds"] = duration if bucket["max_seconds"] is None else max(bucket["max_seconds"], duration)


def finalize(buckets):
    rows = []
    for original in buckets.values():
        bucket = dict(original)
        count = bucket["completed_count"]
        bucket["average_seconds"] = bucket["total_seconds"] / count if count else 0.0
        bucket["min_seconds"] = bucket["min_seconds"] if count else 0.0
        bucket["max_seconds"] = bucket["max_seconds"] if count else 0.0
        bucket["processed_contract_count"] = len(bucket.pop("_completed_contracts"))
        bucket["in_progress_contract_count"] = len(bucket.pop("_pending_contracts"))
        rows.append(bucket)
    return sorted(rows, key=lambda row: (
        row["node_type_code"], -(row["average_seconds"] or 0),
        row.get("employee_id", ""), tuple(row.get("department_ids", [])),
        row.get("process_definition_id", ""), row.get("node_id", "")))


def employee_map(record):
    people = {}
    for person in optional_array(record, "assigneeEmployees"):
        if not isinstance(person, dict):
            raise ValueError("assigneeEmployees 必须为员工对象数组")
        eid = identity(person.get("employeeId"))
        if eid:
            people[eid] = person
    return people


def department_map(person):
    result = {}
    for dep in optional_array(person, "departments"):
        if not isinstance(dep, dict):
            raise ValueError("员工 departments 必须为部门对象数组")
        did = identity(dep.get("departmentId"))
        if did:
            result[did] = display_name(dep, did)
    return result


def business_metadata(registry, instance, record):
    """Historical records must not inherit the current contract's identity."""
    record_id = identity(record.get("businessId"))
    record_type = integer_code(record.get("businessTypeCode"))
    same_process = bool(identity(record.get("processInstanceId"))) and identity(record.get("processInstanceId")) == identity(instance.get("processInstanceId"))
    bid = record_id or (identity(instance.get("businessId")) if same_process else None)
    btype = record_type if record_type is not None else (integer_code(instance.get("businessTypeCode")) if same_process else None)
    matches = list(registry.get((btype, bid), []))
    if identity(instance.get("businessId")) == bid and integer_code(instance.get("businessTypeCode")) == btype:
        matches.append(instance)
    return {"business_key": (btype, bid) if bid and btype is not None else None,
            "contract_id": bid or "", "business_type_code": btype,
            "contract_name": next((b.get("contractName") for b in matches if b.get("contractName")), ""),
            "contract_number": next((b.get("contractNumber") for b in matches if b.get("contractNumber")), "")}


def operation_label(record):
    command = integer_code(record.get("commandTypeCode"))
    status = integer_code(record.get("taskStatusCode"))
    statuses = {0: "在途待办", 2: "拒绝", 3: "已撤回", 4: "已撤销", 5: "自动终止"}
    if status in statuses:
        return statuses[status]
    commands = {0: "提交成功", 1: "通过", 2: "拒绝", 3: "转办", 4: "知会",
                5: "加竞签", 6: "加签", 7: "撤回", 8: "作废", 9: "重新提交",
                10: "加签同意", 11: "自动结束", 12: "加串签", 13: "自动完成",
                14: "自动跳过", 15: "干预转办", 16: "干预中止", 17: "干预回退",
                18: "后加串签", 19: "后加并签", 20: "前加串签", 21: "前加并签",
                22: "确认流程中止", 23: "代理转办", 24: "代理转办失效"}
    if status == 1:
        return commands.get(command, f"其他操作（动作码{command}）")
    return f"未识别状态{status}（动作码{command}）"


def analyze(bundle, approver_ids=(), department_ids=(), node_types=(), process_definition_ids=(), analysis_subject=None):
    if not isinstance(bundle, dict) or not isinstance(bundle.get("meta"), dict):
        raise ValueError("输入必须包含 meta 对象")
    meta = bundle["meta"]
    analysis_subject = analysis_subject or meta.get("analysis_subject") or "other"
    if analysis_subject not in ANALYSIS_SUBJECTS:
        raise ValueError("analysis_subject 只支持 people、department、other")
    if meta.get("timezone_confirmed") is not True:
        raise ValueError("时间源时区未确认：meta.timezone_confirmed 必须为 true")
    if not isinstance(meta.get("timezone_source"), str) or not meta["timezone_source"].strip():
        raise ValueError("时区来源缺失：meta.timezone_source 必须提供非空核验来源")
    try:
        timezone = ZoneInfo(meta["timezone"])
    except (KeyError, TypeError, ZoneInfoNotFoundError) as exc:
        raise ValueError("meta.timezone 必须是有效 IANA 时区") from exc
    start = timestamp(meta.get("start"), timezone)
    end = timestamp(meta.get("end"), timezone)
    if start >= end:
        raise ValueError("统计窗口必须满足 start < end")
    instances = bundle.get("instances")
    if not isinstance(instances, list):
        raise ValueError("instances 必须是数组")
    approver_ids = {identity(value) for value in approver_ids}
    department_ids = {identity(value) for value in department_ids}
    process_definition_ids = {identity(value) for value in process_definition_ids}
    selected_types = {integer_code(value) for value in node_types} or set(NODE_TYPES)
    if not selected_types <= set(NODE_TYPES):
        raise ValueError("node-type 只支持 1=审批、2=盖章、3=归档")

    diagnostics = Counter()
    roster_provided = "employees" in bundle
    roster = {}
    for person in optional_array(bundle, "employees"):
        if not isinstance(person, dict):
            raise ValueError("employees 必须为员工对象数组")
        eid = identity(person.get("employeeId"))
        if not eid:
            raise ValueError("人员清单缺少 employeeId，不能构造零行")
        if approver_ids and eid not in approver_ids:
            continue
        if department_ids and not department_ids.intersection(department_map(person)):
            continue
        roster[eid] = person
    requested_ids = approver_ids or ({identity(v) for v in bundle.get("approver_ids", [])} if "approver_ids" in bundle else set())
    scope_ids = requested_ids or (set(roster) if roster_provided else None)
    participant_evidence = {}
    for evidence in optional_array(bundle, "node_participant_evidence"):
        if not isinstance(evidence, dict):
            raise ValueError("node_participant_evidence 必须为对象数组")
        evidence_key = tuple(identity(evidence.get(k)) for k in ("processInstanceId", "processDefinitionId", "nodeId"))
        ids = {identity(eid) for eid in optional_array(evidence, "employee_ids")}
        if not all(evidence_key) or not all(ids) or not ids:
            raise ValueError("节点人数证据必须含真实实例/定义/节点及人员ID")
        if evidence.get("complete") is True and (not isinstance(evidence.get("source"), str) or not evidence["source"].strip()):
            raise ValueError("完整节点人数证据必须记录核验来源，不能从一个可见操作人推算")
        if evidence_key in participant_evidence and participant_evidence[evidence_key] != evidence:
            raise ValueError("同节点人数证据冲突")
        participant_evidence[evidence_key] = evidence
    source_task_people = {}
    for source_task in optional_array(bundle, "task_records"):
        if not isinstance(source_task, dict):
            raise ValueError("task_records 必须为对象数组")
        source_key = (identity(source_task.get("process_instance_id")),
                      identity(source_task.get("task_instance_id")))
        if not all(source_key):
            continue
        owner_ids = {identity(eid) for eid in optional_array(source_task, "discovered_for_employee_ids")
                     if identity(eid)}
        source_task_people.setdefault(source_key, set()).update(owner_ids)
    group_people, group_modes = {}, {}
    observed_people = {}
    unique = {}
    variants = {}
    conflicts = set()
    conflict_details = []
    unique_businesses = set()
    record_process_ids = set()
    business_registry = {}
    for business in optional_array(bundle, "businesses"):
        if not isinstance(business, dict):
            raise ValueError("businesses 必须为对象数组")
        bid = identity(business.get("businessId") or business.get("contractId"))
        btype = integer_code(business.get("businessTypeCode"))
        if bid and btype is not None:
            business_registry.setdefault((btype, bid), []).append(business)
    for instance in instances:
        if not isinstance(instance, dict):
            raise ValueError("instances 元素必须是对象")
        wrapper_id = identity(instance.get("processInstanceId"))
        response = instance.get("response")
        if not isinstance(response, dict) or response.get("success") is not True or integer_code(response.get("code")) != 200:
            raise ValueError(f"流程实例 {wrapper_id or '(缺ID)'} API 响应失败或不是原始成功响应；禁止把权限/接口失败当成零数据")
        data = response.get("data")
        if not isinstance(data, dict):
            raise ValueError(f"流程实例 {wrapper_id} response.data 不是对象")
        if not any(key in data for key in ("done", "todo")):
            raise ValueError(f"流程实例 {wrapper_id} 缺少 done/todo，不能证明是审批记录响应")
        business_id = identity(instance.get("businessId"))
        business_type = integer_code(instance.get("businessTypeCode"))
        if business_id and business_type is not None:
            unique_businesses.add((business_type, business_id))
        else:
            diagnostics["responses_missing_business_identity"] += 1
        forecasts = optional_array(data, "forecast")
        diagnostics["forecast_nodes_ignored"] += len(forecasts)
        for section in ("done", "todo"):
            nodes = optional_array(data, section)
            for node in nodes:
                if not isinstance(node, dict):
                    raise ValueError("审批节点必须是对象")
                records = optional_array(node, "records")
                for record in records:
                    if not isinstance(record, dict):
                        raise ValueError("records 元素必须是对象")
                    diagnostics["raw_record_count"] += 1
                    observed_people.update(employee_map(record))
                    task_id = identity(record.get("processTaskInstanceId"))
                    # Parent nodes and wrappers can describe a newer instance than
                    # historical records; only the record itself proves this ID.
                    process_id = identity(record.get("processInstanceId"))
                    if process_id:
                        record_process_ids.add(process_id)
                    missing_identity = not task_id or not process_id
                    if missing_identity:
                        diagnostics["missing_dedup_id"] += 1
                    node_id = identity(record.get("nodeId") or node.get("nodeId"))
                    group_key = (process_id, identity(record.get("processDefinitionId")), node_id,
                                 integer_code(node.get("nodeTypeCode")))
                    group_people.setdefault(group_key, set()).update(employee_map(record))
                    group_people[group_key].update(employee_map(node))
                    group_modes.setdefault(group_key, set()).add(integer_code(node.get("taskModeCode")))
                    candidate = {
                        "group_key": group_key, "missing_identity": missing_identity,
                        "business": business_metadata(business_registry, instance, record),
                        "record": record,
                        "node_type": integer_code(node.get("nodeTypeCode")),
                        "node_id": identity(record.get("nodeId") or node.get("nodeId")),
                        "node_name": str(record.get("nodeDisplayName") or record.get("nodeName") or node.get("nodeDisplayName") or node.get("nodeName") or ""),
                        "process_id": process_id,
                    }
                    candidate["fingerprint"] = metric_fingerprint(record, candidate["node_type"], candidate["node_id"], process_id)
                    key = (process_id, task_id) if not missing_identity else ("__unidentified_row__", str(diagnostics["raw_record_count"]))
                    variants.setdefault(key, []).append(candidate)
                    if key in unique:
                        if unique[key]["fingerprint"] == candidate["fingerprint"]:
                            diagnostics["duplicate_copies_ignored"] += 1
                        else:
                            conflicts.add(key)
                        # The same task may be reached through current and historical wrappers.
                        existing = unique[key]["business"]
                        incoming = candidate["business"]
                        if not existing["business_key"] and incoming["business_key"]:
                            unique[key]["business"] = incoming
                        elif incoming["business_key"] and existing["business_key"] != incoming["business_key"]:
                            conflicts.add(key)
                        elif existing["business_key"] == incoming["business_key"]:
                            for field in ("contract_name", "contract_number"):
                                if not existing[field]:
                                    existing[field] = incoming[field]
                        continue
                    unique[key] = candidate
    def representative_priority(candidate):
        record = candidate["record"]
        people = employee_map(record)
        matches = scope_ids is None or bool(set(people).intersection(scope_ids))
        if department_ids:
            matches = matches and any(department_ids.intersection(department_map((roster.get(eid) if roster_provided else None) or person))
                                      for eid, person in people.items())
        try:
            if integer_code(record.get("taskStatusCode")) == 0:
                relevant = timestamp(record.get("createTime"), timezone) < end
            else:
                relevant = start <= timestamp(record.get("endTime"), timezone) < end
            time_priority = 0 if relevant else 2
        except (ValueError, TypeError, OverflowError):
            time_priority = 1
        return (not matches, time_priority, repr(candidate["fingerprint"]), repr(candidate["business"]["business_key"]))

    for conflict_key in conflicts:
        unique[conflict_key] = min(variants[conflict_key], key=representative_priority)
    for process_id, task_id in sorted(conflicts):
        conflict_details.append({"process_instance_id": process_id, "task_instance_id": task_id})
    diagnostics["conflicting_tasks_excluded"] = len(conflicts)
    diagnostics["unique_task_count"] = sum(not c["missing_identity"] for c in unique.values())

    if not roster_provided:
        roster = {eid: person for eid, person in observed_people.items()
                  if (scope_ids is None or eid in scope_ids)
                  and (not department_ids or department_ids.intersection(department_map(person)))}
    # Explicit IDs that were not found must still be visible, identified as unresolved.
    for eid in requested_ids:
        if eid not in roster and not department_ids:
            roster[eid] = observed_people.get(eid) or {"employeeId": eid, "displayName": "姓名未解析"}
    people, departments, process_nodes = {}, {}, {}
    overview = {kind: empty_bucket({"node_type_code": kind, "node_type": NODE_TYPES[kind]}) for kind in sorted(selected_types)}
    for eid, person in roster.items():
        for kind in sorted(selected_types):
            people[(eid, kind)] = empty_bucket({"employee_id": eid, "employee_name": display_name(person, "姓名未解析"),
                                                "node_type_code": kind, "node_type": NODE_TYPES[kind]})
    details = []
    department_samples = []
    employee_department_snapshots = {}
    exclusion_nodes = {kind: set() for kind in selected_types}
    exclusion_tasks = {kind: set() for kind in selected_types}
    exclusion_contracts = {kind: set() for kind in selected_types}
    unknown_nodes = {kind: set() for kind in selected_types}
    unknown_tasks = {kind: set() for kind in selected_types}
    unknown_contracts = {kind: set() for kind in selected_types}
    attributed_multi_nodes = {kind: set() for kind in selected_types}
    attributed_multi_tasks = {kind: set() for kind in selected_types}
    attributed_multi_contracts = {kind: set() for kind in selected_types}
    period_overview_contracts = set()
    period_personal_contracts = set()
    for key, candidate in unique.items():
        record = candidate["record"]
        definition_id = identity(record.get("processDefinitionId"))
        node_type = candidate["node_type"]
        if process_definition_ids and definition_id not in process_definition_ids:
            diagnostics["local_filter_excluded"] += 1
            continue
        if node_type not in selected_types:
            diagnostics["node_type_excluded"] += 1
            continue
        employees = employee_map(record)
        employee_id = next(iter(employees)) if len(employees) == 1 else None
        # An unnamed additional candidate also makes attribution ambiguous.
        unknown_assignee = any(not identity(p.get("employeeId")) for p in optional_array(record, "assigneeEmployees"))
        if unknown_assignee:
            employee_id = None
        employee = employees.get(employee_id)
        raw_source_people = source_task_people.get(key, set())
        source_people = raw_source_people.intersection(roster)
        # Use the selected current roster consistently where available.
        current = (roster.get(employee_id) if roster_provided else None) or employee or {}
        deps = department_map(current)
        observed_or_source_people = set(employees) | raw_source_people
        matching_people = observed_or_source_people if scope_ids is None else observed_or_source_people.intersection(scope_ids)
        if scope_ids is not None and not matching_people:
            diagnostics["local_filter_excluded"] += 1
            continue
        # Observe current department contradictions before filtering them away.
        if employee_id and deps:
            employee_department_snapshots.setdefault(employee_id, set()).add(tuple(sorted(deps)))
        department_candidates = dict(employees)
        for eid in raw_source_people:
            if eid in roster:
                department_candidates[eid] = roster[eid]
        if department_ids and not any(department_ids.intersection(department_map((roster.get(eid) if roster_provided else None) or person))
                                      for eid, person in department_candidates.items()):
            diagnostics["local_filter_excluded"] += 1
            continue
        status = integer_code(record.get("taskStatusCode"))
        pending = status == 0
        duration, created, finished = None, None, None
        reasons = []
        try:
            created = timestamp(record.get("createTime"), timezone)
        except (ValueError, TypeError, OverflowError):
            pass
        try:
            if record.get("endTime"):
                finished = timestamp(record.get("endTime"), timezone)
        except (ValueError, TypeError, OverflowError):
            pass
        if finished is not None and created is not None and finished >= created:
            duration = finished.timestamp() - created.timestamp()
        if pending:
            if finished is not None:
                reasons.append("在途状态存在结束时间")
                diagnostics["pending_with_end_time_excluded"] += 1
            if created is not None and created >= end:
                diagnostics["pending_created_after_window_excluded"] += 1
                continue
            if created is None:
                reasons.append("进入时间缺失或无效")
                diagnostics["invalid_or_missing_time"] += 1
        else:
            if finished is not None and not start <= finished < end:
                diagnostics["outside_completion_window"] += 1
                continue
            if duration is None:
                reasons.append("时间缺失、无效或负时长")
                diagnostics["negative_duration_excluded" if finished and created and finished < created else "invalid_or_missing_time"] += 1
            if status in (3, 4, 5):
                reasons.append("取消或终止任务")
                diagnostics["cancelled_tasks_excluded"] += 1
            elif status not in (1, 2):
                reasons.append("未知或非完成状态")
                diagnostics["unknown_status_excluded"] += 1
            elif integer_code(record.get("commandTypeCode")) not in HANDLED_COMMANDS:
                reasons.append("非有效人工处理动作")
                diagnostics["other_actions_excluded"] += 1
        if candidate["missing_identity"]:
            reasons.append("缺少任务/流程标识，无法去重；明细保留原始行")
        if key in conflicts:
            reasons.append("同一任务的统计字段存在冲突")
        business = candidate["business"]
        business_key = business["business_key"]
        if not business_key:
            diagnostics["tasks_missing_business_identity"] += 1
        observed = group_people[candidate["group_key"]]
        evidence = participant_evidence.get(candidate["group_key"][:3], {})
        proven_ids = {identity(eid) for eid in evidence.get("employee_ids", [])}
        multi = len(observed | proven_ids) > 1
        proven_single = (evidence.get("complete") is True and len(proven_ids) == 1
                         and observed <= proven_ids and employee_id in proven_ids)
        personal_ids = set()
        personal_reason = ""
        if raw_source_people:
            # A per-person task-center row identifies whose concrete task was
            # handled or remains pending, even when the workflow node has
            # several participants.
            source_record_people = source_people.intersection(employees)
            candidates = source_record_people or source_people
            if pending:
                personal_ids = candidates
            elif len(candidates) == 1:
                personal_ids = candidates
            elif len(candidates) > 1:
                personal_reason = "原始任务中心仍归因到多人，无法唯一确定实际处理人"
            else:
                personal_reason = "原始任务中心归因与目标人员清单不一致"
        elif proven_single and employee_id in roster:
            personal_ids = {employee_id}
        else:
            if multi:
                personal_reason = "多人节点缺少可核对的个人任务归因"
            elif employee is None:
                personal_reason = "操作人无法唯一归因"
            else:
                configured_ids = {identity(eid) for eid in optional_array(evidence, "configured_employee_ids")}
                personal_reason = ("单人条件待确认（历史配置含多人，实际分配人数尚未核实）" if len(configured_ids) > 1
                                   else "单人条件待确认（缺原始个人任务或完整历史单人证据）")
        # A completed multi-candidate record cannot be attributed to a selected person.
        # A selected department can count it once only when every candidate is in scope.
        eligible_ids = {eid for eid, person in employees.items()
                        if (scope_ids is None or eid in scope_ids)
                        and (not department_ids or department_ids.intersection(department_map((roster.get(eid) if roster_provided else None) or person)))}
        scope_ambiguous = (not raw_source_people and not pending and employee is None and (scope_ids is not None or department_ids)
                           and (unknown_assignee or not employees or not set(employees).issubset(eligible_ids)))
        if scope_ambiguous:
            reasons.append("操作人跨统计范围且无法唯一归因")
        included = not reasons
        included_personal = included and not personal_reason and bool(personal_ids)
        if multi and not included_personal:
            exclusion_nodes[node_type].add(candidate["group_key"])
            exclusion_tasks[node_type].add(key)
            if business_key:
                exclusion_contracts[node_type].add(business_key)
        if multi and included_personal:
            attributed_multi_nodes[node_type].add(candidate["group_key"])
            attributed_multi_tasks[node_type].add(key)
            if business_key:
                attributed_multi_contracts[node_type].add(business_key)
        if not multi and not personal_ids:
            unknown_nodes[node_type].add(candidate["group_key"])
            unknown_tasks[node_type].add(key)
            if business_key:
                unknown_contracts[node_type].add(business_key)
        detail_reasons = list(reasons)
        if personal_reason:
            detail_reasons.append(personal_reason)
        if not business_key:
            detail_reasons.append("合同归属缺失；不计入合同数")
        if personal_ids:
            operator_name = "、".join(display_name(roster.get(eid) or employees.get(eid) or {}, "姓名未解析")
                                       for eid in sorted(personal_ids))
        else:
            operator_name = "、".join([display_name(roster.get(eid) or person, "姓名未解析")
                                      for eid, person in employees.items()]
                                     + (["身份缺失人员"] if unknown_assignee else [])) or "操作人缺失"
        details.append({
            "contract_name": business["contract_name"], "contract_id": business["contract_id"],
            "contract_number": business["contract_number"], "business_type_code": business["business_type_code"],
            "node_name": candidate["node_name"] or candidate["node_id"] or "未识别节点",
            "node_type": NODE_TYPES[node_type], "node_type_code": node_type,
            "operator_name": operator_name,
            "operator_ids": sorted(employees), "result": operation_label(record),
            "created_at": created.isoformat() if created else (record.get("createTime") or ""),
            "ended_at": finished.isoformat() if finished else (record.get("endTime") or ""),
            "duration_seconds": duration if not pending else None,
            "process_instance_id": candidate["process_id"] or "", "task_instance_id": identity(record.get("processTaskInstanceId")) or "",
            "included_overview": included, "included_personal": included_personal,
            "personal_employee_ids": sorted(personal_ids),
            "exclusion_reason": "；".join(detail_reasons), "multi_person_node": multi,
        })
        if not included:
            continue
        rejected = status == 2 or integer_code(record.get("commandTypeCode")) == 2
        unattributed = employee is None
        diagnostics["in_progress_task_count" if pending else "included_completed_task_count"] += 1
        if not pending and business_key:
            period_overview_contracts.add(business_key)
            if included_personal:
                period_personal_contracts.add(business_key)
        if unattributed:
            diagnostics["unattributed_in_progress" if pending else "unattributed_completed"] += 1
        add_sample(overview[node_type], duration, rejected, pending, unattributed, business_key)
        node_id = candidate["node_id"]
        if definition_id and node_id:
            node_key = (definition_id, node_id, node_type)
            if node_key not in process_nodes:
                process_nodes[node_key] = empty_bucket({"process_definition_id": definition_id,
                    "process_name": definition_id, "node_id": node_id,
                    "node_name": candidate["node_name"] or node_id,
                    "node_type_code": node_type, "node_type": NODE_TYPES[node_type]})
            add_sample(process_nodes[node_key], duration, rejected, pending, unattributed, business_key)
        else:
            diagnostics["missing_definition_or_node_id"] += 1
        if included_personal:
            for personal_id in sorted(personal_ids):
                add_sample(people[(personal_id, node_type)], duration, rejected, pending, False, business_key)
        department_person_ids = personal_ids if included_personal else ({employee_id} if employee_id else set())
        for department_person_id in sorted(department_person_ids):
            personal_deps = department_map(roster.get(department_person_id) or employees.get(department_person_id) or {})
            if personal_deps:
                employee_department_snapshots.setdefault(department_person_id, set()).add(tuple(sorted(personal_deps)))
                department_samples.append((department_person_id, personal_deps, node_type, duration, rejected, pending, business_key))
            else:
                diagnostics["missing_department_attribution"] += 1
    inconsistent_employees = {eid for eid, snapshots in employee_department_snapshots.items() if len(snapshots) > 1}
    diagnostics["inconsistent_department_employees"] = len(inconsistent_employees)
    for eid, deps, kind, duration, rejected, pending, business_key in department_samples:
        if eid in inconsistent_employees:
            diagnostics["inconsistent_department_tasks_excluded"] += 1
            continue
        dep_key = (tuple(sorted(deps)), kind)
        if dep_key not in departments:
            departments[dep_key] = empty_bucket({"department_ids": sorted(deps),
                "department_name": " + ".join(deps[v] for v in sorted(deps)),
                "department_basis": "当前部门组合" if len(deps) > 1 else "当前部门",
                "node_type_code": kind, "node_type": NODE_TYPES[kind]})
        add_sample(departments[dep_key], duration, rejected, pending, False, business_key)
    exclusions = [{"node_type": NODE_TYPES[kind], "node_type_code": kind,
                   "node_count": len(exclusion_nodes[kind]), "task_count": len(exclusion_tasks[kind]),
                   "contract_count": len(exclusion_contracts[kind]),
                   "unverified_node_count": len(unknown_nodes[kind]),
                   "unverified_task_count": len(unknown_tasks[kind]),
                   "unverified_contract_count": len(unknown_contracts[kind]),
                   "attributed_multi_node_count": len(attributed_multi_nodes[kind]),
                   "attributed_multi_task_count": len(attributed_multi_tasks[kind]),
                   "attributed_multi_contract_count": len(attributed_multi_contracts[kind])} for kind in sorted(selected_types)]
    diagnostics["multi_person_nodes_excluded_from_personal"] = sum(r["node_count"] for r in exclusions)
    diagnostics["multi_person_tasks_excluded_from_personal"] = sum(r["task_count"] for r in exclusions)
    diagnostics["single_person_unverified_tasks_excluded"] = sum(r["unverified_task_count"] for r in exclusions)

    return {
        "schema_version": "2.2.1",
        "meta": {"start": start.isoformat(), "end": end.isoformat(),
                 "timezone": str(timezone), "timezone_confirmed": True,
                 "timezone_source": meta["timezone_source"],
                 "analysis_subject": analysis_subject,
                 **{key: meta[key] for key in ("environment", "collected_at", "source", "scope", "collection_status") if key in meta},
                 "coverage": meta.get("coverage") or "仅所提供实例；未证明全量",
                 "period_contract_count": len(period_personal_contracts if analysis_subject == "people" else period_overview_contracts),
                 "period_contract_count_basis": "统计周期内有效完成任务涉及的业务类型+合同ID跨审批、盖章、归档去重；具体人员模式仅计入可归因到所选人员的任务",
                 "instance_count": len(instances),
                 "input_response_count": len(instances),
                 "unique_business_count": len(unique_businesses),
                 "unique_process_instance_count": len(record_process_ids),
                 "time_basis": "自然历时；已处理按 endTime 半开区间过滤；含等待和节假日",
                 "sample_unit": "唯一流程任务实例；节点表不是并行节点整体墙钟时长",
                 "department_basis": "当前组织快照；多部门采用组合桶，非历史归属",
                 "roster_source": "输入完整人员清单" if roster_provided else "输入任务中出现的人员；无法保证零任务人员齐全",
                 "zero_display": "无有效已处理样本时显示0，不代表不存在数据缺口",
                 "multi_person_basis": "优先按原始任务中心的流程实例ID+任务ID归因到具体员工；多人节点中可明确归因的个人任务纳入个人统计。只有任务中心仍无法唯一归因时才排除；无原始任务时回退完整历史单人证据",
                 "pending_basis": "抓取时仍在途且创建早于窗口结束；不是历史时点积压快照",
                 "filters": {"approver_ids": sorted(approver_ids), "department_ids": sorted(department_ids), "node_types": sorted(selected_types), "process_definition_ids": sorted(process_definition_ids)}},
        "overview": finalize(overview), "details": details, "personal_exclusions": exclusions,
        "roster": list(roster.values()),
        "by_approver": finalize(people), "by_department": finalize(departments),
        "by_process_node": finalize(process_nodes),
        "diagnostics": dict(sorted(diagnostics.items())), "conflicting_tasks": conflict_details,
    }


def cell(value):
    """Keep user-controlled names inside their Markdown table cell."""
    value = html.escape(str(value), quote=False).replace("\\", "\\\\")
    for symbol in ("|", "`", "*", "_", "[", "]"):
        value = value.replace(symbol, "\\" + symbol)
    return value.replace("\r\n", " / ").replace("\r", " / ").replace("\n", " / ")


def hours(value):
    return "—" if value is None else f"{value / 3600:.2f}"


def table(rows, labels, keys):
    output = ["| " + " | ".join(labels) + " |", "| " + " | ".join(["---"] * len(labels)) + " |"]
    for row in rows:
        values = [hours(row.get(k)) if k.endswith("_seconds") else cell(row.get(k, "")) for k in keys]
        output.append("| " + " | ".join(values) + " |")
    if not rows:
        output.append("| " + " | ".join(["未取得人员清单"] + ["—"] * (len(labels) - 1)) + " |")
    return "\n".join(output)


def insights(report):
    rows = report["overview"]
    valid = [r for r in rows if r["completed_count"]]
    if not valid:
        return "当前范围没有可纳入统计的有效完成任务；先核对权限、时间窗口与排除项。表中0表示本次有效样本为0。"
    slow = max(valid, key=lambda r: r["average_seconds"])
    pending = sum(r["in_progress_count"] for r in rows)
    count = sum(r["completed_count"] for r in rows)
    rejected = sum(r["rejected_count"] for r in rows)
    note = f"{slow['node_type']}的任务平均用时最高，为{hours(slow['average_seconds'])}小时（{slow['completed_count']}次有效处理）；全部节点共{count}次有效处理，其中拒绝{rejected}次，占{rejected / count:.1%}。当前在途待办共{pending}项。"
    return note + "自然用时包含等待和非工作时间；可优先核对长耗时任务的业务原因，不能据此直接评价个人表现。"


def render_markdown(report):
    meta = report["meta"]
    people_only = meta.get("analysis_subject") == "people"
    overview_keys = ["node_type", "processed_contract_count", "approved_count", "rejected_count", "in_progress_contract_count", "total_seconds", "average_seconds"]
    personal_keys = ["employee_name", "processed_contract_count", "approved_count", "rejected_count", "in_progress_count", "total_seconds", "average_seconds", "max_seconds", "min_seconds"]
    single_person = people_only and len(report.get("roster", [])) == 1
    scope = meta.get("scope", "当前授权账号可读取、符合筛选条件的目标对象流程")
    if isinstance(scope, dict):
        resolved = scope.get("resolved") if isinstance(scope.get("resolved"), dict) else {}
        scope = resolved.get("displayName") or resolved.get("name") or scope.get("query") or "当前授权账号可读取、符合筛选条件的目标对象流程"
    parts = ["# 个人效率统计" if people_only else "# 审批效率统计", f"统计周期：{cell(meta['start'])} ≤ 完成时间 < {cell(meta['end'])}；时区：{cell(meta['timezone'])}。",
             f"统计数据范围：{cell(scope)}；统计周期内涉及合同数：{meta.get('period_contract_count', 0)}份。",
             "环境：" + cell(meta.get("environment", "未声明")) + "；数据来源：" + cell(meta.get("source", "成功读取的流程记录")) + "；采集时间：" + cell(meta.get("collected_at", "未声明")) + "；人员清单：" + cell(meta["roster_source"]) + "。",
             "口径：处理合同数按业务类型+合同ID去重；通过、拒绝按有效任务次数计。平均用时=有效任务总用时÷有效处理次数；均为自然历时，含等待和节假日。在途为采集时快照，不计入用时。部门采用当前组织归属。节点表平均=节点任务平均用时，不是并行节点整体用时。"]
    if people_only:
        parts.append("个人归因优先使用任务中心的流程实例ID、任务ID和人员任务清单；多人节点中能明确确认实际处理人及起止时间的任务正常计入。仅归因仍不唯一的任务排除。")
    else:
        parts.extend(["## 第一部分：审批效率总览",
                      table(report["overview"], ["节点类型", "处理合同数", "审批通过次数", "审批拒绝次数", "在途合同数", "总用时(h)", "平均用时(h)"], overview_keys),
                      "效率洞察：" + insights(report),
                      "## 第二部分：个人审批效率统计",
                      "个人归因优先使用任务中心的流程实例ID、任务ID和人员任务清单；多人节点中能明确确认实际处理人的任务正常计入。仅归因仍不唯一的任务排除。总览保留符合对象与时间范围的有效任务，因此不一定等于个人表之和。"])
    if single_person:
        combined_keys = ["employee_name", "node_type", *personal_keys[1:]]
        parts.append(table(report["by_approver"],
                           ["姓名", "节点类型", "处理合同数", "审批通过次数", "审批拒绝次数", "在途待办任务数", "总用时(h)", "平均用时(h)", "最长用时(h)", "最短用时(h)"], combined_keys))
        summaries = []
        for excluded in report["personal_exclusions"]:
            summaries.append(f"{excluded['node_type']}：任务中心已归因多人节点任务{excluded['attributed_multi_task_count']}条；仍无法归因而排除{excluded['task_count']}条；其他单人条件待确认{excluded['unverified_task_count']}条")
        parts.append("归因与排除：" + "；".join(summaries) + "。")
    else:
        for stage in report["overview"]:
            kind = stage["node_type_code"]
            parts.append("### " + stage["node_type"])
            parts.append(table([r for r in report["by_approver"] if r["node_type_code"] == kind],
                               ["姓名", "处理合同数", "审批通过次数", "审批拒绝次数", "在途待办任务数", "总用时(h)", "平均用时(h)", "最长用时(h)", "最短用时(h)"], personal_keys))
            excluded = next(r for r in report["personal_exclusions"] if r["node_type_code"] == kind)
            parts.append(f"本类任务中心已明确归因的多人节点任务{excluded['attributed_multi_task_count']}条，已计入对应人员；仍有{excluded['task_count']}条多人节点任务无法唯一归因而排除。另有{excluded['unverified_task_count']}条任务因缺少原始个人任务或完整单人证据而暂不计入。")
    parts.append("无有效处理数据的人员按要求展示0；数据缺失、未授权及被排除任务不因此视为真实零耗时。")
    if meta.get("collection_status") == "partial":
        parts.append("数据完整性：本次读取存在接口、权限或对账缺口；未取得的记录没有按零处理，详细诊断保留在JSON中。")
    if not people_only:
        parts.extend(["## 第三部分：审批明细", f"共{len(report['details'])}条去重后的范围内记录，含个人统计排除项和在途待办。先交付Excel，Excel生成失败则交付CSV并说明原因；缺时间的单条明细留空。"])
    diagnostics = report["diagnostics"]
    parts.append(f"明细共{len(report['details'])}条，包含符合范围的已办、在途及排除记录；"
                 f"其中有效人工已办{diagnostics.get('included_completed_task_count', 0)}条，"
                 "个人表仅进一步排除任务中心无法唯一归因或缺少必要归因证据的记录。")
    conflicts = diagnostics.get("conflicting_tasks_excluded", 0)
    if conflicts:
        parts.append(f"另有{conflicts}条任务出现互相冲突的记录，已排除统计并保留核对信息。")
    if report.get("detail_export"):
        result = report["detail_export"]
        parts.append(f"明细文件：[{cell(Path(result['path']).name)}](<{result['path']}>)")
        if result.get("fallback_reason"):
            parts.append("Excel未生成，已提供CSV：" + cell(result["fallback_reason"]))
    return "\n\n".join(parts) + "\n"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--markdown", type=Path)
    parser.add_argument("--json", dest="json_output", type=Path)
    parser.add_argument("--details", type=Path, help="明细Excel路径；失败自动写同名CSV")
    parser.add_argument("--csv-only", action="store_true", help="显式只导出CSV")
    parser.add_argument("--node", help="Excel导出所用Node绝对路径")
    parser.add_argument("--node-modules", help="包含@oai/artifact-tool的依赖目录")
    parser.add_argument("--approver-id", action="append", default=[])
    parser.add_argument("--department-id", action="append", default=[])
    parser.add_argument("--node-type", action="append", type=int, choices=NODE_TYPES, default=[])
    parser.add_argument("--process-definition-id", action="append", default=[])
    parser.add_argument("--analysis-subject", choices=sorted(ANALYSIS_SUBJECTS),
                        help="输出对象：people=具体一人或多人；department=部门；other=其他范围")
    args = parser.parse_args(argv)
    try:
        bundle = json.loads(args.input.read_text(encoding="utf-8"))
        report = analyze(bundle, args.approver_id, args.department_id, args.node_type,
                         args.process_definition_id, args.analysis_subject)
        from export_details import export_report
        output_base = args.markdown or args.json_output or args.input.with_name("approval-efficiency-report.md")
        details_path = args.details or output_base.with_name(output_base.stem + "-details.xlsx")
        report["detail_export"] = export_report(report, details_path, csv_only=args.csv_only,
                                                node=args.node, node_modules=args.node_modules)
        markdown = render_markdown(report)
        if args.markdown:
            args.markdown.parent.mkdir(parents=True, exist_ok=True)
            args.markdown.write_text(markdown, encoding="utf-8")
        if args.json_output:
            args.json_output.parent.mkdir(parents=True, exist_ok=True)
            args.json_output.write_text(json.dumps(report, ensure_ascii=False, indent=2, allow_nan=False) + "\n", encoding="utf-8")
        if not args.markdown:
            print(markdown, end="")
    except (OSError, ValueError, TypeError, KeyError) as exc:
        print(f"分析失败：{exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
