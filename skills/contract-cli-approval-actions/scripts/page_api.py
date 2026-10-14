#!/usr/bin/env python3
"""Restricted test-environment page API client for approval actions.

The client intentionally exposes only the endpoints needed by this Skill.  It
reads the current contract-cli User credential, never prints the credential,
and makes every mutation opt-in with a task-bound confirmation argument.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import datetime as dt
import hashlib
import hmac
import json
import os
import shutil
import subprocess
import sys
import time
import urllib.parse
from pathlib import Path
from typing import Any, Iterable


BASE_URL = "https://test-contract.qtech.cn"
DEFAULT_PROFILE = "contract-test"
TASK_PATHS = {
    "todo": "/clm/api/workflow/runtime/task/todo",
    "done": "/clm/api/workflow/runtime/task/done",
    "notice": "/clm/api/workflow/runtime/notice/getNotices",
}
# 标品默认任务列表目前不会稳定返回合同组；合同组业务类型为 10。
# 查询时补拉这些业务类型并按任务身份去重，避免默认列表未来恢复后出现重复。
FALLBACK_BUSINESS_TYPE_CODES = (10,)
TASK_QUERY_FIELDS = (
    "contractNumber",
    "contractName",
    "businessTypeName",
    "contractCategoryName",
    "contractCategoryNumber",
    "taskNodeDisplayName",
    "taskNodeName",
)
ACTION_PATH = "/clm/api/workflow/runtime/task/run"
NOTICE_PATH = "/clm/api/workflow/runtime/notice/send"
DETAIL_PATH = "/clm/api/composition/contractAndTask"
EMPLOYEE_PATH = "/clm/api/employee/findEmployeesByKeyword"
EMPLOYEE_IDS_PATH = "/clm/api/employee/getEmployeesByIds"
CATEGORY_PATH = "/clm/api/category/findAllWithinAuthority"

ENDORSE_COMMANDS = {
    "parallel": 6,
    "after-serial": 18,
    "after-parallel": 19,
    "before-serial": 20,
    "before-parallel": 21,
}
ENDORSE_PERMISSIONS = {
    "parallel": "endorse",
    "after-serial": "post_serial_endorsement",
    "after-parallel": "post_parallel_endorsement",
    "before-serial": "pre_serial_endorsement",
    "before-parallel": "pre_parallel_endorsement",
}
ENDORSE_MODE_LABELS = {
    "parallel": "并加签",
    "after-serial": "后串签",
    "after-parallel": "后会签",
    "before-serial": "前串签",
    "before-parallel": "前会签",
}
ENDORSE_MODE_BY_COMMAND = {value: key for key, value in ENDORSE_COMMANDS.items()}
ACTION_LABELS = {
    "approve": "审批通过",
    "agree": "审批通过",
    "endorse": "加签",
    "transfer": "转办",
    "notice": "抄送",
}

TASK_TABLE_COLUMNS = ("序号", "对象", "名称", "编号", "当前节点", "当前处理人")
TASK_TABLE_FULL_LIMIT = 20
TASK_TABLE_SUMMARY_LIMIT = 10

HUMAN_NODE_TYPES = {"approve", "submit", "archive"}
NODE_TYPE_PREFIXES = {
    "UserTask_approve": "approve",
    "UserTask_submit": "submit",
    "UserTask_archive": "archive",
}


class SkillError(RuntimeError):
    def __init__(self, kind: str, message: str, details: Any = None):
        super().__init__(message)
        self.kind = kind
        self.message = message
        self.details = details


def emit(payload: Any) -> None:
    print(json.dumps(sanitize(payload), ensure_ascii=False, indent=2))


def sanitize(value: Any) -> Any:
    """Prevent credentials accidentally returned by an upstream payload."""
    secret_markers = ("token", "secret", "password", "ticket", "authorization")
    if isinstance(value, dict):
        result = {}
        for key, item in value.items():
            if any(marker in str(key).lower() for marker in secret_markers):
                result[key] = "<redacted>"
            else:
                result[key] = sanitize(item)
        return result
    if isinstance(value, list):
        return [sanitize(item) for item in value]
    return value


def config_path() -> Path:
    configured = os.environ.get("CONTRACT_CLI_CONFIG_DIR")
    base = Path(configured).expanduser() if configured else Path.home() / ".contract-cli"
    return base / "config.json"


def parse_expiry(value: str) -> dt.datetime:
    normalized = value.strip().replace("Z", "+00:00")
    parsed = dt.datetime.fromisoformat(normalized)
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=dt.timezone.utc)
    return parsed


def load_auth(profile_name: str) -> dict[str, Any]:
    path = config_path()
    if not path.exists():
        raise SkillError("auth", "当前设备尚未配置合同登录，请先完成 User 登录", {"configPath": str(path)})
    try:
        config = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SkillError("auth", "无法读取当前合同登录配置，请重新登录或检查本机配置", str(exc)) from exc

    profile = (config.get("profiles") or {}).get(profile_name)
    if not isinstance(profile, dict):
        raise SkillError("auth", "未找到可用的合同登录配置", {"profile": profile_name})
    if profile.get("environment") != "test":
        raise SkillError("boundary", "当前登录环境与本 Skill 不匹配，已停止操作")
    if profile.get("default_identity") != "user":
        raise SkillError("boundary", "当前不是 User 登录，无法处理本人审批")

    identity = ((profile.get("identities") or {}).get("user") or {})
    token = identity.get("token") or {}
    access_token = token.get("access_token")
    if not access_token:
        raise SkillError("auth", "尚未完成 User 登录，请先登录后再试")

    expiry = token.get("expiry")
    if expiry:
        try:
            if parse_expiry(expiry) <= dt.datetime.now(dt.timezone.utc):
                raise SkillError("auth", "当前 User 登录已过期，请重新登录")
        except ValueError as exc:
            raise SkillError("auth", "无法确认当前 User 登录是否有效，请重新登录", {"expiry": expiry}) from exc

    return {
        "profile": profile_name,
        "environment": "test",
        "access_token": access_token,
        "expiry": expiry,
    }


def request_json(
    auth: dict[str, Any],
    method: str,
    path: str,
    *,
    query: dict[str, Any] | None = None,
    body: dict[str, Any] | None = None,
) -> dict[str, Any]:
    query_items = []
    for key, value in (query or {}).items():
        if value is not None:
            query_items.append((key, str(value).lower() if isinstance(value, bool) else value))
    url = BASE_URL + path
    if query_items:
        url += "?" + urllib.parse.urlencode(query_items)
    access_token = str(auth["access_token"])
    if any(character in access_token for character in ('\n', '\r', '"')):
        raise SkillError("auth", "User 凭证格式异常")
    command = [
        "curl",
        "--config",
        "-",
        "--silent",
        "--show-error",
        "--connect-timeout",
        "10",
        "--max-time",
        "30",
        "--request",
        method,
        "--url",
        url,
        "--write-out",
        "\n%{http_code}",
    ]
    if body is not None:
        command.extend(
            [
                "--header",
                "Content-Type: application/json",
                "--data-binary",
                json.dumps(body, ensure_ascii=False, separators=(",", ":")),
            ]
        )
    curl_config = (
        'header = "Accept: application/json"\n'
        f'header = "Authorization: Bearer {access_token}"\n'
        'header = "User-Agent: contract-cli-approval-actions/1.0"\n'
    )
    try:
        completed = subprocess.run(
            command,
            input=curl_config,
            text=True,
            capture_output=True,
            check=False,
            timeout=35,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise SkillError("network", "暂时无法连接合同页面服务，请稍后重试", str(exc)) from exc
    raw_with_status = completed.stdout
    if "\n" not in raw_with_status:
        message = completed.stderr.strip() or "未收到 HTTP 响应"
        raise SkillError("network", "暂时无法访问合同页面服务，请稍后重试", message)
    raw, status_text = raw_with_status.rsplit("\n", 1)
    try:
        status = int(status_text)
    except ValueError as exc:
        raise SkillError("response", "合同页面服务返回异常，暂时无法继续", status_text) from exc
    if completed.returncode != 0 and status == 0:
        message = completed.stderr.strip() or "网络请求失败"
        raise SkillError("network", "暂时无法访问合同页面服务，请稍后重试", message)
    if status >= 400:
        try:
            details = json.loads(raw)
        except json.JSONDecodeError:
            details = raw[:2000]
        raise SkillError("http", "合同页面服务返回异常，暂时无法继续", {"status": status, "response": details})

    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise SkillError("response", "合同页面服务返回了无法识别的结果，暂时无法继续", {"status": status}) from exc
    if not isinstance(payload, dict):
        raise SkillError("response", "合同页面服务返回了无法识别的结果，暂时无法继续", payload)
    if payload.get("success") is False or int(payload.get("code") or status) >= 400:
        raise SkillError(
            "business",
            "这次操作未通过页面业务校验，没有继续执行",
            {
                "message": payload.get("message") or payload.get("msg"),
                "code": payload.get("code"),
                "requestId": payload.get("requestId"),
            },
        )
    return payload


def page_items(payload: dict[str, Any]) -> list[dict[str, Any]]:
    data = payload.get("data") or {}
    if isinstance(data, list):
        return [item for item in data if isinstance(item, dict)]
    for key in ("content", "records", "items", "list"):
        value = data.get(key) if isinstance(data, dict) else None
        if isinstance(value, list):
            return [item for item in value if isinstance(item, dict)]
    return []


def task_page(
    auth: dict[str, Any],
    scope: str,
    page_index: int,
    page_size: int,
    query_text: str | None = None,
    business_type_code: int | None = None,
) -> dict[str, Any]:
    return request_json(
        auth,
        "GET",
        TASK_PATHS[scope],
        query={
            "pageIndex": page_index,
            "pageSize": page_size,
            "desc": True,
            "query": query_text,
            "businessTypeCode": business_type_code,
        },
    )


def collect_task_pages(
    auth: dict[str, Any],
    scope: str,
    query_text: str | None = None,
    business_type_code: int | None = None,
    max_pages: int = 100,
) -> list[dict[str, Any]]:
    collected: list[dict[str, Any]] = []
    for page_index in range(1, max_pages + 1):
        payload = task_page(auth, scope, page_index, 100, query_text, business_type_code)
        items = page_items(payload)
        collected.extend(items)
        page = payload.get("data") or {}
        has_next = bool(page.get("hasNext")) if isinstance(page, dict) else False
        if not has_next and len(items) < 100:
            break
    else:
        raise SkillError("limit", f"任务列表超过 {max_pages * 100} 条，请增加筛选条件")
    return collected


def task_matches_query(item: dict[str, Any], query_text: str) -> bool:
    keyword = query_text.strip().casefold()
    if not keyword:
        return True
    searchable = " ".join(str(item.get(field) or "") for field in TASK_QUERY_FIELDS).casefold()
    return keyword in searchable


def collect_tasks(
    auth: dict[str, Any],
    scope: str,
    query_text: str | None = None,
    business_type_code: int | None = None,
    max_pages: int = 100,
) -> list[dict[str, Any]]:
    """Collect the complete visible task set, including contract groups.

    Page-side free-text search is not consistent for contract names, so the
    restricted runner applies the user keyword locally to already-authorized
    task rows. This never expands the current User's visibility.
    """
    business_types: list[int | None]
    if business_type_code is not None:
        business_types = [business_type_code]
    else:
        business_types = [None, *FALLBACK_BUSINESS_TYPE_CODES]

    unique: dict[str, dict[str, Any]] = {}
    anonymous: list[dict[str, Any]] = []
    for current_type in business_types:
        rows = collect_task_pages(
            auth,
            scope,
            query_text=None,
            business_type_code=current_type,
            max_pages=max_pages,
        )
        for item in rows:
            identity = task_identity(item)
            if identity:
                unique[str(identity)] = item
            elif item not in anonymous:
                anonymous.append(item)

    collected = [*unique.values(), *anonymous]
    if query_text and query_text.strip():
        collected = [item for item in collected if task_matches_query(item, query_text)]
    return collected


def task_identity(item: dict[str, Any]) -> str | None:
    return item.get("processTaskInstanceId") or item.get("processNoticeTaskInstanceId")


def task_node_type(item: dict[str, Any]) -> str | None:
    value = item.get("taskNodeTypeName")
    if value:
        return str(value).lower()
    node_id = str(item.get("taskNodeId") or "")
    for prefix, node_type in NODE_TYPE_PREFIXES.items():
        if node_id.startswith(prefix):
            return node_type
    return None


def require_human_task(item: dict[str, Any]) -> None:
    node_type = task_node_type(item)
    node_id = str(item.get("taskNodeId") or "")
    if node_type in HUMAN_NODE_TYPES or node_id.startswith("UserTask_"):
        return
    raise SkillError("validation", "目标不是标品页面中的人工任务节点")


def find_task_by_detail(
    auth: dict[str, Any], task_id: str, scopes: Iterable[str]
) -> tuple[str, dict[str, Any]] | None:
    """Resolve one known task without scanning an unbounded history list.

    Accounts with many years of completed approvals can exceed the list scan
    guard. The page detail endpoint is already permission checked and accepts
    the exact task instance, so it is the safe fallback for an ID previously
    resolved from the current conversation.
    """
    payload = request_json(
        auth,
        "GET",
        DETAIL_PATH,
        query={
            "taskInstanceId": task_id,
            "batchContractChildrenPageIndex": 0,
            "batchContractChildrenPageSize": 10,
            "tianyancha": False,
            "smart": False,
        },
    )
    detail = payload.get("data") or {}
    task_meta = (((detail.get("task") or {}).get("taskMeta")) or {})
    if str(task_meta.get("processTaskInstanceId") or "") != str(task_id):
        return None
    task_type_name = str(task_meta.get("taskTypeName") or "").lower()
    if task_type_name in TASK_PATHS:
        scope = task_type_name
    else:
        scope = {0: "todo", 1: "done", 2: "notice"}.get(task_meta.get("taskTypeCode"), "done")
    allowed = set(scopes)
    if scope not in allowed:
        return None
    return scope, task_meta


def find_task_by_id(
    auth: dict[str, Any], task_id: str, scopes: Iterable[str]
) -> tuple[str, dict[str, Any]]:
    scope_values = tuple(scopes)
    if len(scope_values) > 1:
        direct = find_task_by_detail(auth, task_id, scope_values)
        if direct:
            return direct
    matches: list[tuple[str, dict[str, Any]]] = []
    history_scan_limited = False
    for scope in scope_values:
        try:
            rows = collect_tasks(auth, scope)
        except SkillError as exc:
            if exc.kind != "limit":
                raise
            history_scan_limited = True
            break
        for item in rows:
            ids = {item.get("processTaskInstanceId"), item.get("processNoticeTaskInstanceId")}
            if task_id in ids:
                matches.append((scope, item))
    unique = {(scope, task_identity(item)): (scope, item) for scope, item in matches}
    if not unique:
        if history_scan_limited or len(scope_values) > 1:
            direct = find_task_by_detail(auth, task_id, scope_values)
            if direct:
                return direct
        raise SkillError(
            "target",
            "该审批已不在当前可操作范围内，可能已经处理完成或被转交",
            {"taskId": task_id},
        )
    if len(unique) > 1:
        raise SkillError(
            "target",
            "这条审批同时存在多个可见记录，请先明确要处理当前待办还是历史记录",
            {"taskId": task_id, "scopes": sorted(scope for scope, _item in unique.values())},
        )
    return next(iter(unique.values()))


def task_detail(auth: dict[str, Any], scope: str, item: dict[str, Any]) -> dict[str, Any]:
    business_id = item.get("businessId")
    task_id = item.get("processTaskInstanceId")
    notice_id = item.get("processNoticeTaskInstanceId")
    # 与任务中心当前前端保持一致：有任务时只传 taskInstanceId/noticeId。
    # 同时传 contractId 会让合同组被按普通合同查询并返回“未查询到该合同”。
    query: dict[str, Any] = {
        "batchContractChildrenPageIndex": 0,
        "batchContractChildrenPageSize": 10,
        "tianyancha": False,
        "smart": False,
    }
    if scope == "notice" and not task_id and notice_id:
        query["noticeId"] = notice_id
    elif task_id:
        query["taskInstanceId"] = task_id
    elif business_id:
        query["contractId"] = business_id
    else:
        raise SkillError("target", "任务缺少可用于读取详情的业务标识")
    payload = request_json(auth, "GET", DETAIL_PATH, query=query)
    return payload.get("data") or {}


def permission_values(detail: dict[str, Any]) -> set[str]:
    permissions = (((detail.get("task") or {}).get("permissions")) or {})
    values = set()
    for item in permissions.get("permittedFunctions") or []:
        if isinstance(item, dict) and item.get("value"):
            values.add(str(item["value"]).lower())
    return values


def summarize_task(scope: str, item: dict[str, Any]) -> dict[str, Any]:
    keys = (
        "businessId",
        "businessTypeCode",
        "businessTypeName",
        "contractNumber",
        "contractName",
        "contractCategoryId",
        "contractCategoryName",
        "contractCategoryNumber",
        "processInstanceId",
        "processTaskInstanceId",
        "processNoticeTaskInstanceId",
        "taskNodeId",
        "taskNodeTypeCode",
        "taskNodeTypeName",
        "taskNodeDisplayName",
        "taskNodeName",
        "taskTypeCode",
        "taskTypeName",
        "timestamp",
    )
    business_type_name = str(item.get("businessTypeName") or "")
    object_type = (
        "合同组"
        if item.get("businessTypeCode") == 10 or "合同组" in business_type_name
        else "合同"
    )
    return {
        "scope": scope,
        "objectType": object_type,
        **{key: item.get(key) for key in keys if item.get(key) is not None},
    }


def process_records(detail: dict[str, Any]) -> dict[str, Any]:
    """Return the page's approval records without exposing the full detail payload."""
    task_records = ((detail.get("task") or {}).get("processRecords") or {})
    if isinstance(task_records, dict) and task_records:
        return task_records
    contract_info = ((detail.get("contract") or {}).get("contractInfo") or {})
    contract_records = contract_info.get("approveRecord") or {}
    return contract_records if isinstance(contract_records, dict) else {}


def current_node_employees(
    detail: dict[str, Any], item: dict[str, Any]
) -> list[dict[str, Any]]:
    """Resolve active handlers for the task's exact current node.

    A process can have multiple active nodes. The broad
    ``allTodoAssigneeEmployees`` list therefore cannot be used for one row in
    the todo table. Match the row's node (or, as a fallback, the exact task)
    and collect only unfinished records at that node.
    """
    todo_nodes = process_records(detail).get("todo") or []
    target_node_id = str(item.get("taskNodeId") or "")
    target_task_id = str(item.get("processTaskInstanceId") or "")
    matched_nodes = [
        node
        for node in todo_nodes
        if isinstance(node, dict) and str(node.get("nodeId") or "") == target_node_id
    ]
    if not matched_nodes and target_task_id:
        matched_nodes = [
            node
            for node in todo_nodes
            if isinstance(node, dict)
            and any(
                str(record.get("processTaskInstanceId") or "") == target_task_id
                for record in (node.get("records") or [])
                if isinstance(record, dict)
            )
        ]

    employees: list[dict[str, Any]] = []
    seen: set[str] = set()
    for node in matched_nodes:
        for record in node.get("records") or []:
            if not isinstance(record, dict) or record.get("commandTypeCode") is not None:
                continue
            for employee in record.get("assigneeEmployees") or []:
                if not isinstance(employee, dict):
                    continue
                identity = str(
                    employee.get("employeeId")
                    or employee.get("userId")
                    or employee.get("email")
                    or employee.get("displayName")
                    or employee.get("name")
                    or ""
                )
                if not identity or identity in seen:
                    continue
                seen.add(identity)
                employees.append(employee)
    return employees


def current_handler_summary(detail: dict[str, Any], item: dict[str, Any]) -> dict[str, Any]:
    current_employee_id = str(
        (((detail.get("task") or {}).get("permissions") or {}).get("employeeId")) or ""
    )
    employees = current_node_employees(detail, item)
    handlers: list[tuple[bool, str]] = []
    for employee in employees:
        employee_id = str(employee.get("employeeId") or "")
        is_current_user = bool(current_employee_id and employee_id == current_employee_id)
        name = "你" if is_current_user else str(
            employee.get("displayName") or employee.get("name") or ""
        ).strip()
        if name:
            handlers.append((is_current_user, name))

    # Put the current user first so a multi-person row reads naturally as
    # “你、王江涛”, while preserving the page order for everyone else.
    handlers.sort(key=lambda value: not value[0])
    names = [name for _, name in handlers]
    if not names:
        return {
            "currentHandlerStatus": "unavailable",
            "currentHandlerCount": 0,
            "currentHandlerNames": [],
            "currentHandlerDisplay": "暂未取得",
        }
    display = "、".join(names) if len(names) <= 3 else f"{'、'.join(names[:2])}等{len(names)}人"
    return {
        "currentHandlerStatus": "available",
        "currentHandlerCount": len(names),
        "currentHandlerNames": names,
        "currentHandlerDisplay": display,
    }


def summarize_task_with_handlers(
    auth: dict[str, Any], scope: str, item: dict[str, Any]
) -> dict[str, Any]:
    summary = summarize_task(scope, item)
    if scope != "todo":
        return summary
    try:
        detail = task_detail(auth, scope, item)
        summary.update(current_handler_summary(detail, item))
    except SkillError:
        # Handler enrichment is display data; a temporary detail failure must
        # not hide an otherwise valid todo row.
        summary.update(
            {
                "currentHandlerStatus": "unavailable",
                "currentHandlerCount": 0,
                "currentHandlerNames": [],
                "currentHandlerDisplay": "暂未取得",
            }
        )
    return summary


def summarize_task_page(
    auth: dict[str, Any], scope: str, items: list[dict[str, Any]]
) -> list[dict[str, Any]]:
    if scope != "todo" or len(items) < 2:
        return [summarize_task_with_handlers(auth, scope, item) for item in items]
    worker_count = min(6, len(items))
    with concurrent.futures.ThreadPoolExecutor(max_workers=worker_count) as executor:
        return list(
            executor.map(
                lambda item: summarize_task_with_handlers(auth, scope, item),
                items,
            )
        )


def task_presentation_row(summary: dict[str, Any], sequence: int) -> dict[str, Any]:
    """Build the fixed six-field business row required by the Skill UI contract."""
    return {
        "序号": sequence,
        "对象": str(summary.get("objectType") or "合同"),
        "名称": str(summary.get("contractName") or "—"),
        "编号": str(summary.get("contractNumber") or "—"),
        "当前节点": str(
            summary.get("taskNodeDisplayName") or summary.get("taskNodeName") or "—"
        ),
        "当前处理人": str(summary.get("currentHandlerDisplay") or "暂未取得"),
    }


def markdown_cell(value: Any) -> str:
    return str(value).replace("|", "\\|").replace("\r", " ").replace("\n", " ")


def build_task_presentation(summaries: list[dict[str, Any]]) -> dict[str, Any]:
    """Return a deterministic, business-only table contract for Agent output.

    The display threshold is based on the final actionable candidate set passed
    to this function, never on the broader unfiltered match set.
    """
    candidate_count = len(summaries)
    if candidate_count <= TASK_TABLE_FULL_LIMIT:
        displayed = summaries
        display_mode = "full"
    else:
        displayed = summaries[:TASK_TABLE_SUMMARY_LIMIT]
        display_mode = "summary"
    rows = [
        task_presentation_row(summary, sequence)
        for sequence, summary in enumerate(displayed, start=1)
    ]
    header = "| " + " | ".join(TASK_TABLE_COLUMNS) + " |"
    divider = "|" + "|".join("---" for _ in TASK_TABLE_COLUMNS) + "|"
    body = [
        "| " + " | ".join(markdown_cell(row[column]) for column in TASK_TABLE_COLUMNS) + " |"
        for row in rows
    ]
    return {
        "columns": list(TASK_TABLE_COLUMNS),
        "candidateCount": candidate_count,
        "displayMode": display_mode,
        "displayedCount": len(rows),
        "remainingCount": candidate_count - len(rows),
        "rows": rows,
        "markdown": "\n".join([header, divider, *body]),
    }


def build_task_page_presentation(
    summaries: list[dict[str, Any]], total: int, start: int
) -> dict[str, Any]:
    """Render one stable, globally numbered page of the complete todo list."""
    rows = [
        task_presentation_row(summary, start + offset + 1)
        for offset, summary in enumerate(summaries)
    ]
    header = "| " + " | ".join(TASK_TABLE_COLUMNS) + " |"
    divider = "|" + "|".join("---" for _ in TASK_TABLE_COLUMNS) + "|"
    body = [
        "| " + " | ".join(markdown_cell(row[column]) for column in TASK_TABLE_COLUMNS) + " |"
        for row in rows
    ]
    return {
        "columns": list(TASK_TABLE_COLUMNS),
        "candidateCount": total,
        "displayMode": "full" if total <= TASK_TABLE_FULL_LIMIT else "summary",
        "displayedCount": len(rows),
        "remainingCount": max(total - start - len(rows), 0),
        "fromSequence": rows[0]["序号"] if rows else None,
        "toSequence": rows[-1]["序号"] if rows else None,
        "rows": rows,
        "markdown": "\n".join([header, divider, *body]),
    }


def summarize_detail(detail: dict[str, Any]) -> dict[str, Any]:
    task = detail.get("task") or {}
    permissions = task.get("permissions") or {}
    task_meta = task.get("taskMeta") or {}
    notice_rows: list[dict[str, Any]] = []
    for record in notice_record_items(detail):
        employee = record.get("noticeEmployee") or {}
        notice_rows.append(
            {
                "recipientName": str(
                    employee.get("displayName")
                    or employee.get("name")
                    or record.get("noticeEmployeeName")
                    or "未知人员"
                ),
                "readStatus": record.get("readStatus") is True,
                "createTime": record.get("createTime"),
            }
        )
    notice_rows.sort(key=lambda row: str(row.get("createTime") or ""), reverse=True)
    endorsement_rows: list[dict[str, Any]] = []
    current_employee_id = str(permissions.get("employeeId") or "")
    for record in process_record_items(detail):
        mode = ENDORSE_MODE_BY_COMMAND.get(record.get("commandTypeCode"))
        if not mode:
            continue
        names = []
        for employee in record.get("endorsementEmployees") or []:
            if not isinstance(employee, dict):
                continue
            if current_employee_id and str(employee.get("employeeId") or "") == current_employee_id:
                continue
            name = str(employee.get("displayName") or employee.get("name") or "").strip()
            if name:
                names.append(name)
        endorsement_rows.append(
            {
                "mode": ENDORSE_MODE_LABELS.get(mode, mode),
                "recipientNames": names,
                "remark": str(record.get("taskComment") or ""),
                "createTime": record.get("createTime") or record.get("taskStartTime"),
            }
        )
    endorsement_rows.sort(key=lambda row: str(row.get("createTime") or ""), reverse=True)
    return {
        "taskMeta": summarize_task(task_meta.get("taskTypeName") or "detail", task_meta),
        "permissions": {
            "newProcess": permissions.get("newProcess"),
            "readPermission": permissions.get("readPermission"),
            "writePermission": permissions.get("writePermission"),
            "permittedFunctions": permissions.get("permittedFunctions") or [],
            "endorsePermissionInfoVo": permissions.get("endorsePermissionInfoVo"),
        },
        "noticeSummary": {
            "count": len(notice_rows),
            "latest": notice_rows[0] if notice_rows else None,
            "records": notice_rows,
        },
        "endorsementSummary": {
            "count": len(endorsement_rows),
            "latest": endorsement_rows[0] if endorsement_rows else None,
            "records": endorsement_rows,
        },
    }


def require_action_permission(detail: dict[str, Any], action: str, mode: str | None = None) -> None:
    values = permission_values(detail)
    required = action
    if action == "endorse":
        required = ENDORSE_PERMISSIONS[mode or "parallel"]
        if "endorse" not in values:
            raise SkillError("permission", "标品页面当前任务未开放加签动作")
    if required not in values:
        label = ENDORSE_MODE_LABELS.get(mode or "") or ACTION_LABELS.get(action, "该操作")
        raise SkillError(
            "permission",
            f"当前节点不支持{label}",
            {"requiredPermission": required},
        )


def task_runtime_meta(detail: dict[str, Any], fallback: dict[str, Any]) -> dict[str, Any]:
    task_meta = (((detail.get("task") or {}).get("taskMeta")) or {})
    result = {
        "businessId": task_meta.get("businessId") or fallback.get("businessId"),
        "businessTypeCode": task_meta.get("businessTypeCode"),
        "processInstanceId": task_meta.get("processInstanceId") or fallback.get("processInstanceId"),
        "processTaskInstanceId": task_meta.get("processTaskInstanceId")
        or fallback.get("processTaskInstanceId"),
    }
    if result["businessTypeCode"] is None:
        result["businessTypeCode"] = fallback.get("businessTypeCode")
    missing = [key for key, value in result.items() if value is None]
    if missing:
        raise SkillError(
            "target",
            "任务详情不完整，当前无法安全执行该操作",
            {"missingFields": missing},
        )
    return result


def ensure_unique(values: list[str], label: str) -> None:
    if not values:
        raise SkillError("validation", f"{label}至少需要一人")
    if len(values) != len(set(values)):
        raise SkillError("validation", f"{label}列表存在重复人员")


def validate_employee_ids(
    auth: dict[str, Any], values: list[str], label: str
) -> list[dict[str, Any]]:
    ensure_unique(values, label)
    payload = request_json(
        auth,
        "GET",
        EMPLOYEE_IDS_PATH,
        query={"employeeIds": ",".join(values), "extraDepartmentInfo": True},
    )
    rows = [item for item in (payload.get("data") or []) if isinstance(item, dict)]
    by_id = {str(item.get("employeeId")): item for item in rows if item.get("employeeId")}
    missing = [value for value in values if str(value) not in by_id]
    if missing:
        raise SkillError("validation", f"{label}中有人员不存在或当前不可见")
    result = []
    for value in values:
        item = by_id[str(value)]
        if item.get("expired") is True or item.get("locked") is True or item.get("canSelect") is False:
            raise SkillError("validation", f"{label}中有人员当前不可选择")
        result.append(
            {
                "employeeId": item.get("employeeId"),
                "displayName": item.get("displayName") or item.get("name"),
                "email": item.get("email"),
                "departmentNames": item.get("departmentNames"),
            }
        )
    return result


def plan_payload(
    action: str,
    item: dict[str, Any],
    body: dict[str, Any],
    plan_context: dict[str, Any] | None = None,
) -> dict[str, Any]:
    payload = {
        "action": action,
        "taskId": task_identity(item),
        "taskTimestamp": item.get("timestamp"),
        "nodeId": item.get("taskNodeId"),
        "nodeType": task_node_type(item),
        "businessId": item.get("businessId"),
        "requestBody": body,
    }
    if plan_context:
        payload["planContext"] = plan_context
    return payload


def plan_fingerprint(payload: dict[str, Any]) -> str:
    canonical = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def confirm_execute(args: argparse.Namespace, expected_fingerprint: str) -> bool:
    if not args.execute:
        return False
    if os.environ.get("CONTRACT_APPROVAL_EVAL_BLOCK_WRITES") == "1":
        raise SkillError(
            "evaluation_safety",
            "当前测评层级只允许查询和生成方案，未执行审批写入",
        )
    if args.confirm_task_id and args.confirm_task_id != args.task_id:
        raise SkillError("confirmation", "确认对象与当前待执行对象不一致，请重新预览")
    actual_fingerprint = args.confirm_plan_fingerprint or ""
    if not hmac.compare_digest(actual_fingerprint, expected_fingerprint):
        raise SkillError("confirmation", "确认内容与当前动作、人员或参数不一致，请重新预览")
    return True


def process_record_payload(detail: dict[str, Any]) -> dict[str, Any]:
    return (((detail.get("task") or {}).get("processRecords")) or {})


def process_record_items(detail: dict[str, Any]) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    process_records = process_record_payload(detail)
    for scope in ("todo", "done", "forecast"):
        for node in process_records.get(scope) or []:
            for record in node.get("records") or []:
                if isinstance(record, dict):
                    records.append(record)
    return records


def process_record_ids(detail: dict[str, Any]) -> set[str]:
    return {
        str(record.get("processTaskInstanceId"))
        for record in process_record_items(detail)
        if record.get("processTaskInstanceId")
    }


def notice_record_items(detail: dict[str, Any]) -> list[dict[str, Any]]:
    notice_records = (((detail.get("task") or {}).get("noticeRecords")) or {})
    return [record for record in notice_records.get("notice") or [] if isinstance(record, dict)]


def notice_record_ids(detail: dict[str, Any]) -> set[str]:
    return {
        str(record.get("noticeId"))
        for record in notice_record_items(detail)
        if record.get("noticeId")
    }


def employee_ids(employees: Any) -> list[str]:
    return [
        str(employee.get("employeeId"))
        for employee in employees or []
        if isinstance(employee, dict) and employee.get("employeeId")
    ]


def detail_after_write(auth: dict[str, Any], item: dict[str, Any]) -> dict[str, Any] | None:
    if item.get("processTaskInstanceId"):
        try:
            exact_detail = task_detail(auth, "done", item)
        except SkillError:
            exact_detail = None
        if exact_detail and (process_record_payload(exact_detail) or notice_record_items(exact_detail)):
            return exact_detail
    process_id = str(item.get("processInstanceId") or "")
    business_id = str(item.get("businessId") or "")
    candidates: list[tuple[str, dict[str, Any]]] = []
    for scope in ("todo", "done"):
        for candidate in collect_tasks(auth, scope):
            same_process = process_id and str(candidate.get("processInstanceId") or "") == process_id
            same_business = business_id and str(candidate.get("businessId") or "") == business_id
            if same_process or (not process_id and same_business):
                candidates.append((scope, candidate))
    candidates.sort(
        key=lambda pair: (
            pair[0] != "todo",
            str(pair[1].get("timestamp") or ""),
        )
    )
    for scope, candidate in candidates:
        try:
            detail = task_detail(auth, scope, candidate)
        except SkillError:
            continue
        if process_record_payload(detail) or notice_record_items(detail):
            return detail
    return None


def endorse_postcheck(
    before_detail: dict[str, Any],
    after_detail: dict[str, Any] | None,
    body: dict[str, Any],
    recipients: list[dict[str, Any]],
) -> dict[str, Any]:
    mode = ENDORSE_MODE_BY_COMMAND.get(body.get("commandTypeCode"))
    expected_ids = [str(value) for value in body.get("endorseEmployeeIds") or []]
    expected_set = set(expected_ids)
    def action_signature(record: dict[str, Any]) -> tuple[Any, ...]:
        return (
            str(record.get("processTaskInstanceId") or ""),
            record.get("commandTypeCode"),
            tuple(employee_ids(record.get("endorsementEmployees"))),
            str(record.get("taskComment") or "").strip(),
        )

    before_signatures = {
        action_signature(record) for record in process_record_items(before_detail)
    }
    after_records = process_record_items(after_detail or {})
    action_candidates = [
        record
        for record in after_records
        if record.get("commandTypeCode") == body.get("commandTypeCode")
        and action_signature(record) not in before_signatures
        and expected_set.issubset(set(employee_ids(record.get("endorsementEmployees"))))
        and str(record.get("taskComment") or "").strip() == str(body.get("remark") or "").strip()
    ]
    if not mode or not action_candidates:
        return {
            "status": "not_verified",
            "businessCheck": "页面已接受加签，但暂未在审批记录中核验到对应的加签结果",
        }

    action_record = action_candidates[-1]
    action_record_id = str(action_record.get("processTaskInstanceId") or "")
    children = [
        record
        for record in after_records
        if str(record.get("parentProcessTaskInstanceId") or "") == action_record_id
        and record.get("parentCommandTypeCode") == body.get("commandTypeCode")
    ]
    child_assignees = [employee_ids(record.get("assigneeEmployees")) for record in children]
    child_first_ids = [values[0] for values in child_assignees if values]
    child_by_employee = {
        employee_id: record
        for record, values in zip(children, child_assignees)
        for employee_id in values
        if employee_id in expected_set
    }
    recipients_present = expected_set.issubset(set(child_by_employee))
    order_verified: bool | None = None
    state_verified = False
    if mode in {"before-serial", "after-serial"}:
        requested_order = [value for value in child_first_ids if value in expected_set]
        order_verified = requested_order[: len(expected_ids)] == expected_ids
        state_verified = bool(expected_ids) and (
            str(child_by_employee.get(expected_ids[0], {}).get("taskStatusName") or "").lower()
            == "running"
        )
    else:
        state_verified = recipients_present and all(
            str(child_by_employee[employee_id].get("taskStatusName") or "").lower() == "running"
            for employee_id in expected_ids
        )

    verified = recipients_present and state_verified and order_verified is not False
    recipient_names = [str(recipient.get("displayName") or "") for recipient in recipients]
    result: dict[str, Any] = {
        "status": "verified" if verified else "not_verified",
        "businessCheck": (
            "已核验加签模式、人员和顺序"
            if verified and order_verified is True
            else "已核验加签模式和人员"
            if verified
            else "页面已接受加签，但审批记录与预期模式或人员不完全一致"
        ),
        "endorsement": {
            "mode": ENDORSE_MODE_LABELS.get(mode, mode),
            "recipients": recipient_names,
            "orderVerified": order_verified,
        },
    }
    return result


def notice_postcheck(
    before_detail: dict[str, Any],
    after_detail: dict[str, Any] | None,
    recipients: list[dict[str, Any]],
) -> dict[str, Any]:
    before_ids = notice_record_ids(before_detail)
    expected_ids = {
        str(recipient.get("employeeId"))
        for recipient in recipients
        if recipient.get("employeeId")
    }
    new_records = [
        record
        for record in notice_record_items(after_detail or {})
        if str(record.get("noticeId") or "") not in before_ids
    ]
    by_recipient = {
        str(record.get("noticeEmployeeId")): record
        for record in new_records
        if record.get("noticeEmployeeId")
    }
    verified = bool(expected_ids) and expected_ids.issubset(set(by_recipient))
    return {
        "status": "verified" if verified else "not_verified",
        "businessCheck": (
            "已核验抄送记录和接收人；是否已阅读以接收人侧状态为准"
            if verified
            else "页面已接受抄送，但暂未在发送记录中核验到全部接收人"
        ),
        "notice": {
            "recipients": [str(recipient.get("displayName") or "") for recipient in recipients],
            "allRead": (
                all(by_recipient[employee_id].get("readStatus") is True for employee_id in expected_ids)
                if verified
                else None
            ),
        },
    }


def postcheck_write(
    auth: dict[str, Any],
    action: str,
    item: dict[str, Any],
    before_detail: dict[str, Any],
    body: dict[str, Any],
    recipients: list[dict[str, Any]],
) -> dict[str, Any]:
    task_id = task_identity(item)
    if action in {"transfer", "approve"}:
        still_todo = any(task_identity(row) == task_id for row in collect_tasks(auth, "todo"))
        if action == "approve":
            after_detail = detail_after_write(auth, item)
            approved = any(
                str(record.get("processTaskInstanceId") or "") == str(task_id)
                and record.get("commandTypeCode") == 1
                for record in process_record_items(after_detail or {})
            )
            verified = not still_todo and approved
            return {
                "status": "verified" if verified else "not_verified",
                "businessCheck": (
                    "原待办已移除，且审批记录显示本节点已通过"
                    if verified
                    else "页面已接受审批，但暂未同时核验到待办移除和通过记录"
                ),
            }
        return {
            "status": "verified" if not still_todo else "not_verified",
            "businessCheck": "任务已从本人待办移除" if not still_todo else "任务仍在本人待办，不能确认转办已生效",
        }

    after_detail = detail_after_write(auth, item)
    if action == "endorse":
        return endorse_postcheck(before_detail, after_detail, body, recipients)
    return notice_postcheck(before_detail, after_detail, recipients)


def action_result(
    auth: dict[str, Any],
    args: argparse.Namespace,
    action: str,
    scope: str,
    item: dict[str, Any],
    detail: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any],
    recipients: list[dict[str, Any]],
    plan_context: dict[str, Any] | None = None,
) -> dict[str, Any]:
    payload = plan_payload(action, item, body, plan_context)
    fingerprint = plan_fingerprint(payload)
    execute = confirm_execute(args, fingerprint)
    result: dict[str, Any] = {
        "ok": True,
        "action": action,
        "executed": execute,
        "target": summarize_task(scope, item),
        "recipients": recipients,
        "preflight": {
            "nodeType": task_node_type(item),
            "nodeName": item.get("taskNodeDisplayName") or item.get("taskNodeName"),
            "actionAvailable": True,
        },
        "planFingerprint": fingerprint,
    }
    if args.diagnostic:
        result["diagnostic"] = {
            "profile": auth["profile"],
            "environment": auth["environment"],
            "permissions": summarize_detail(detail).get("permissions"),
            "request": {"method": method, "path": path, "body": body},
        }
    if execute:
        result["response"] = request_json(auth, method, path, body=body)
        postcheck = postcheck_write(auth, action, item, detail, body, recipients)
        for _attempt in range(2):
            if postcheck.get("status") == "verified":
                break
            time.sleep(1)
            postcheck = postcheck_write(auth, action, item, detail, body, recipients)
        result["postcheck"] = postcheck
    else:
        result["next"] = "预览已生成；按 Skill 执行契约取得用户授权后执行"
    return result


def cmd_auth_check(auth: dict[str, Any], _args: argparse.Namespace) -> dict[str, Any]:
    return {
        "ok": True,
        "profile": auth["profile"],
        "environment": auth["environment"],
        "identity": "user",
        "credentialExpiry": auth["expiry"],
        "baseUrl": BASE_URL,
    }


def cmd_runtime_check(auth: dict[str, Any], _args: argparse.Namespace) -> dict[str, Any]:
    """Verify the self-contained Skill runtime without exposing local paths or tokens."""
    cli = shutil.which("contract-cli")
    curl = shutil.which("curl")
    cli_version = None
    if cli:
        try:
            completed = subprocess.run(
                [cli, "--version"],
                text=True,
                capture_output=True,
                check=False,
                timeout=5,
            )
            first_line = (completed.stdout or completed.stderr).strip().splitlines()
            cli_version = first_line[0] if first_line else None
        except (OSError, subprocess.TimeoutExpired):
            cli_version = None
    return {
        "ok": bool(cli and curl),
        "profile": auth["profile"],
        "environment": auth["environment"],
        "identity": "user",
        "runtime": {
            "contractCliAvailable": bool(cli),
            "contractCliVersion": cli_version,
            "pythonVersion": ".".join(str(value) for value in sys.version_info[:3]),
            "curlAvailable": bool(curl),
            "selfContainedSkillRunner": True,
        },
    }


def cmd_tasks(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    if args.page_index < 1 or args.page_size < 1:
        raise SkillError("validation", "页码和每页数量必须大于 0")
    rows = collect_tasks(
        auth,
        args.scope,
        args.query,
        args.business_type_code,
    )
    rows.sort(key=lambda item: str(item.get("timestamp") or ""), reverse=True)
    start = (args.page_index - 1) * args.page_size
    end = start + args.page_size
    total = len(rows)
    total_pages = (total + args.page_size - 1) // args.page_size
    selected_rows = rows[start:end]
    summaries = summarize_task_page(auth, args.scope, selected_rows)
    page = {
        "content": [
            {"sequence": start + offset + 1, **summary}
            for offset, summary in enumerate(summaries)
        ],
        "pageIndex": args.page_index,
        "pageSize": args.page_size,
        "numberOfElements": len(selected_rows),
        "totalElements": total,
        "totalPages": total_pages,
        "hasNext": args.page_index < total_pages,
    }
    return {
        "ok": True,
        "profile": auth["profile"],
        "environment": auth["environment"],
        "scope": args.scope,
        "page": page,
    }


def cmd_task_list(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    """Return a deterministic, paged business table for the complete todo list."""
    if args.page_index < 1 or args.page_size < 1 or args.page_size > 20:
        raise SkillError("validation", "页码必须大于 0，每批展示数量必须为 1–20")
    rows = collect_tasks(auth, "todo", args.query, args.business_type_code)
    rows.sort(key=lambda item: str(item.get("timestamp") or ""), reverse=True)
    total = len(rows)
    object_distribution: dict[str, int] = {}
    node_distribution: dict[str, int] = {}
    for item in rows:
        summary = summarize_task("todo", item)
        object_name = str(summary.get("objectType") or "合同")
        node_name = str(
            summary.get("taskNodeDisplayName")
            or summary.get("taskNodeName")
            or "未命名节点"
        )
        object_distribution[object_name] = object_distribution.get(object_name, 0) + 1
        node_distribution[node_name] = node_distribution.get(node_name, 0) + 1
    if total <= TASK_TABLE_FULL_LIMIT:
        start = 0
        selected_rows = rows
    else:
        start = (args.page_index - 1) * args.page_size
        selected_rows = rows[start : start + args.page_size]
    summaries = summarize_task_page(auth, "todo", selected_rows)
    page = {
        "content": [
            {"sequence": start + offset + 1, **summary}
            for offset, summary in enumerate(summaries)
        ],
        "pageIndex": args.page_index,
        "pageSize": args.page_size,
        "numberOfElements": len(selected_rows),
        "totalElements": total,
        "hasNext": start + len(selected_rows) < total,
    }
    return {
        "ok": True,
        "profile": auth["profile"],
        "environment": auth["environment"],
        "scope": "todo",
        "collection": {
            "fullyCollected": True,
            "deduplicated": True,
            "includesFallbackBusinessTypes": list(FALLBACK_BUSINESS_TYPE_CODES),
        },
        "distribution": {
            "byObjectType": object_distribution,
            "byNode": node_distribution,
        },
        "page": page,
        "presentation": build_task_page_presentation(summaries, total, start),
    }


def cmd_task_table(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    """Render an exact current-task set with the fixed six-field display contract."""
    ensure_unique(args.task_id, "任务")
    summaries: list[dict[str, Any]] = []
    for task_id in args.task_id:
        scope, item = find_task_by_id(auth, task_id, ("todo",))
        summaries.append(summarize_task_with_handlers(auth, scope, item))
    return {
        "ok": True,
        "scope": "todo",
        "presentation": build_task_presentation(summaries),
    }


def category_nodes(value: Any) -> Iterable[dict[str, Any]]:
    if isinstance(value, list):
        for item in value:
            yield from category_nodes(item)
    elif isinstance(value, dict):
        if value.get("id"):
            yield value
        yield from category_nodes(value.get("subCategoryNodeList") or [])


def cmd_categories(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    payload = request_json(auth, "GET", CATEGORY_PATH)
    keyword = (args.keyword or "").casefold()
    nodes = list(category_nodes(payload.get("data")))
    by_id = {str(node.get("id")): node for node in nodes if node.get("id")}
    rows = []
    for node in nodes:
        searchable = " ".join(
            str(node.get(key) or "")
            for key in ("id", "name", "categoryDisplayName", "categoryNumber", "abbreviation")
        ).casefold()
        if keyword and keyword not in searchable:
            continue
        rows.append(
            {
                "id": node.get("id"),
                "name": node.get("name"),
                "categoryDisplayName": node.get("categoryDisplayName"),
                "categoryNumber": node.get("categoryNumber"),
                "parentId": node.get("parentId"),
                "parentName": (by_id.get(str(node.get("parentId"))) or {}).get("name"),
                "depth": node.get("depth"),
                "enabled": node.get("enabled"),
            }
        )
    return {"ok": True, "count": len(rows), "categories": rows}


def cmd_employees(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    payload = request_json(
        auth,
        "GET",
        EMPLOYEE_PATH,
        query={"keywords": args.keyword, "filterCurrentEmployee": True},
    )
    employees = []
    for item in payload.get("data") or []:
        if not isinstance(item, dict):
            continue
        employees.append(
            {
                "employeeId": item.get("employeeId"),
                "displayName": item.get("displayName") or item.get("name"),
                "email": item.get("email"),
                "employeeCode": item.get("employeeCode"),
                "departmentNames": item.get("departmentNames"),
                "canSelect": item.get("canSelect"),
                "expired": item.get("expired"),
                "locked": item.get("locked"),
                "status": item.get("status"),
            }
        )
    return {"ok": True, "count": len(employees), "employees": employees}


def cmd_resolve(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    tasks = collect_tasks(auth, args.scope, args.query, args.business_type_code)
    category_ids: set[str] | None = None
    if args.include_descendants:
        if not args.category_id and not args.category_number:
            raise SkillError("validation", "展开子类型前需要先明确一个合同类型")
        category_payload = request_json(auth, "GET", CATEGORY_PATH)
        nodes = list(category_nodes(category_payload.get("data")))
        roots = [
            node
            for node in nodes
            if (args.category_id and str(node.get("id")) == str(args.category_id))
            or (
                args.category_number
                and str(node.get("categoryNumber")) == str(args.category_number)
            )
        ]
        if len(roots) != 1:
            raise SkillError("target", "合同类型无法唯一定位，不能展开子类型")
        children_by_parent: dict[str, list[dict[str, Any]]] = {}
        for node in nodes:
            children_by_parent.setdefault(str(node.get("parentId")), []).append(node)
        category_ids = set()
        pending = [roots[0]]
        while pending:
            node = pending.pop()
            node_id = str(node.get("id"))
            category_ids.add(node_id)
            pending.extend(children_by_parent.get(node_id, []))

    filters = {
        "processTaskInstanceId": args.task_id,
        "processInstanceId": args.process_id,
        "businessId": args.contract_id,
        "contractNumber": args.contract_number,
        "taskNodeId": args.node_id,
        "contractCategoryId": None if category_ids is not None else args.category_id,
        "contractCategoryNumber": None if category_ids is not None else args.category_number,
    }
    for key, expected in filters.items():
        if expected is not None:
            tasks = [item for item in tasks if str(item.get(key)) == str(expected)]
    if category_ids is not None:
        tasks = [item for item in tasks if str(item.get("contractCategoryId")) in category_ids]
    if args.node_type:
        tasks = [item for item in tasks if task_node_type(item) == args.node_type]
    tasks.sort(key=lambda item: str(item.get("timestamp") or ""), reverse=True)
    return {
        "ok": True,
        "scope": args.scope,
        "matchCount": len(tasks),
        "latestTaskId": task_identity(tasks[0]) if tasks else None,
        "latestSelection": (
            {
                "nodeType": task_node_type(tasks[0]),
                "nodeName": tasks[0].get("taskNodeDisplayName") or tasks[0].get("taskNodeName"),
                "timestamp": tasks[0].get("timestamp"),
            }
            if tasks
            else None
        ),
        "tasks": [summarize_task(args.scope, item) for item in tasks],
    }


def cmd_task_detail(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    scopes = TASK_PATHS.keys() if args.scope == "auto" else (args.scope,)
    scope, item = find_task_by_id(auth, args.task_id, scopes)
    detail = task_detail(auth, scope, item)
    return {
        "ok": True,
        "target": summarize_task(scope, item),
        "detail": summarize_detail(detail),
    }


def cmd_endorse(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    if not args.remark.strip():
        raise SkillError("validation", "加签原因不能为空")
    recipients = validate_employee_ids(auth, args.employee_id, "加签人")
    if args.mode in {"before-parallel", "after-parallel"} and len(args.employee_id) < 2:
        raise SkillError("validation", "前/后会签至少需要两名加签人")
    scope, item = find_task_by_id(auth, args.task_id, ("todo",))
    require_human_task(item)
    detail = task_detail(auth, scope, item)
    require_action_permission(detail, "endorse", args.mode)
    meta = task_runtime_meta(detail, item)

    permissions = (((detail.get("task") or {}).get("permissions")) or {})
    current_employee = permissions.get("employeeId")
    if current_employee and str(current_employee) in {str(value) for value in args.employee_id}:
        raise SkillError("validation", "加签人不能包含当前用户自己")
    endorse_permission = permissions.get("endorsePermissionInfoVo") or {}
    configured = endorse_permission.get("attributeEditableConfig")
    editable_map = {"editable": "ENDORSEE_EDITABLE", "read-only": "ENDORSEE_NOT_EDITABLE"}
    if configured == "ENDORSER_SELECT":
        if not args.endorsee_editable:
            raise SkillError("validation", "当前节点要求明确加签人是否可以编辑合同，请补充选择")
        editable = editable_map[args.endorsee_editable]
    elif configured in {"ENDORSEE_EDITABLE", "ENDORSEE_NOT_EDITABLE"}:
        editable = configured
    else:
        editable = None

    endorse_info: dict[str, Any] = {"endorseEmployeeIdList": args.employee_id}
    if editable:
        endorse_info["endorseeAttributeEditable"] = editable
    body = {
        "businessId": meta["businessId"],
        "businessTypeCode": meta["businessTypeCode"],
        "commandTypeCode": ENDORSE_COMMANDS[args.mode],
        "processTaskInstanceId": meta["processTaskInstanceId"],
        "endorseEmployeeIds": args.employee_id,
        "endorseInfo": endorse_info,
    }
    body["remark"] = args.remark.strip()
    return action_result(
        auth, args, "endorse", scope, item, detail, "PUT", ACTION_PATH, body, recipients
    )


def cmd_approve(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    scope, item = find_task_by_id(auth, args.task_id, ("todo",))
    require_human_task(item)
    if task_node_type(item) != "approve":
        raise SkillError("validation", "审批通过只支持当前真实审批节点")
    detail = task_detail(auth, scope, item)
    require_action_permission(detail, "agree")
    permissions = (((detail.get("task") or {}).get("permissions")) or {})
    if permissions.get("writePermission") is True:
        raise SkillError(
            "boundary",
            "该审批节点允许同时修改合同内容，当前无法安全代替页面完成审批",
        )
    meta = task_runtime_meta(detail, item)
    body = {
        "businessId": meta["businessId"],
        "businessTypeCode": meta["businessTypeCode"],
        "commandTypeCode": 1,
        "processTaskInstanceId": meta["processTaskInstanceId"],
    }
    if args.remark and args.remark.strip():
        body["remark"] = args.remark.strip()
    return action_result(
        auth, args, "approve", scope, item, detail, "PUT", ACTION_PATH, body, []
    )


def cmd_transfer(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    recipients = validate_employee_ids(auth, [args.employee_id], "转办接收人")
    scope, item = find_task_by_id(auth, args.task_id, ("todo",))
    require_human_task(item)
    detail = task_detail(auth, scope, item)
    require_action_permission(detail, "transfer")
    meta = task_runtime_meta(detail, item)
    current_employee = ((((detail.get("task") or {}).get("permissions")) or {}).get("employeeId"))
    if current_employee and str(current_employee) == str(args.employee_id):
        raise SkillError("validation", "不能把审批转办给当前用户自己")
    body = {
        "businessId": meta["businessId"],
        "businessTypeCode": meta["businessTypeCode"],
        "commandTypeCode": 3,
        "processTaskInstanceId": meta["processTaskInstanceId"],
        "transferEmployeeId": args.employee_id,
    }
    if args.remark:
        body["remark"] = args.remark
    return action_result(
        auth, args, "transfer", scope, item, detail, "PUT", ACTION_PATH, body, recipients
    )


def cmd_notice(auth: dict[str, Any], args: argparse.Namespace) -> dict[str, Any]:
    recipients = validate_employee_ids(auth, args.employee_id, "抄送人")
    scope, item = find_task_by_id(auth, args.task_id, TASK_PATHS.keys())
    require_human_task(item)
    detail = task_detail(auth, scope, item)
    require_action_permission(detail, "notice")
    meta = task_runtime_meta(detail, item)
    current_employee = ((((detail.get("task") or {}).get("permissions")) or {}).get("employeeId"))
    if current_employee and str(current_employee) in {str(value) for value in args.employee_id}:
        raise SkillError("validation", "抄送人不能包含当前用户自己")
    existing_recipient_ids = {
        str(record.get("noticeEmployeeId"))
        for record in notice_record_items(detail)
        if record.get("noticeEmployeeId")
    }
    repeated_names = [
        str(recipient.get("displayName") or "")
        for recipient in recipients
        if str(recipient.get("employeeId") or "") in existing_recipient_ids
    ]
    if repeated_names and not args.allow_repeat:
        display = "、".join(name for name in repeated_names if name) or "所选人员"
        raise SkillError(
            "validation",
            f"{display}已经收到过这条审批的抄送；如需再次提醒，请明确要求重复抄送",
        )
    body = {
        "noticeEmployeeIds": args.employee_id,
        "processInstanceId": meta["processInstanceId"],
        "processTaskInstanceId": meta["processTaskInstanceId"],
    }
    if args.remark:
        body["remark"] = args.remark
    return action_result(
        auth,
        args,
        "notice",
        scope,
        item,
        detail,
        "POST",
        NOTICE_PATH,
        body,
        recipients,
        {"allowRepeat": bool(args.allow_repeat)},
    )


def add_write_gate(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--execute", action="store_true", help="实际发起写请求；默认仅生成预览")
    parser.add_argument(
        "--confirm-task-id",
        help="兼容性内部参数；如传入必须与 task-id 完全一致",
    )
    parser.add_argument(
        "--confirm-plan-fingerprint",
        help="Skill 内部参数；必须与预览返回的 planFingerprint 完全一致",
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="合同审批动作 test 页面接口运行器")
    parser.add_argument("--profile", default=DEFAULT_PROFILE, help="test 环境 contract-cli profile")
    parser.add_argument(
        "--diagnostic",
        action="store_true",
        help="仅在用户授权排障时返回最少必要技术诊断信息",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    auth = sub.add_parser("auth-check", help="检查 User 登录和 test 环境边界")
    auth.set_defaults(handler=cmd_auth_check)

    runtime = sub.add_parser("runtime-check", help="检查 CLI、User 登录和 Skill 随包运行依赖")
    runtime.set_defaults(handler=cmd_runtime_check)

    tasks = sub.add_parser("tasks", help="查询当前用户待办、已办或抄送列表")
    tasks.add_argument("--scope", choices=TASK_PATHS.keys(), default="todo")
    tasks.add_argument("--query")
    tasks.add_argument("--business-type-code", type=int)
    tasks.add_argument("--page-index", type=int, default=1)
    tasks.add_argument("--page-size", type=int, default=20)
    tasks.set_defaults(handler=cmd_tasks)

    task_list = sub.add_parser(
        "task-list", help="一次生成完整待办总数和固定六字段分页表格"
    )
    task_list.add_argument("--query")
    task_list.add_argument("--business-type-code", type=int)
    task_list.add_argument("--page-index", type=int, default=1)
    task_list.add_argument("--page-size", type=int, default=10)
    task_list.set_defaults(handler=cmd_task_list)

    task_table = sub.add_parser(
        "task-table", help="按最终可执行任务集合生成固定六字段业务表格"
    )
    task_table.add_argument("--task-id", action="append", required=True)
    task_table.set_defaults(handler=cmd_task_table)

    resolve = sub.add_parser("resolve", help="把合同、流程、节点或类型筛选为真实任务")
    resolve.add_argument("--scope", choices=TASK_PATHS.keys(), default="todo")
    resolve.add_argument("--query")
    resolve.add_argument("--business-type-code", type=int)
    resolve.add_argument("--task-id")
    resolve.add_argument("--process-id")
    resolve.add_argument("--contract-id")
    resolve.add_argument("--contract-number")
    resolve.add_argument("--node-id")
    resolve.add_argument("--category-id")
    resolve.add_argument("--category-number")
    resolve.add_argument("--include-descendants", action="store_true")
    resolve.add_argument("--node-type", choices=sorted(HUMAN_NODE_TYPES))
    resolve.set_defaults(handler=cmd_resolve)

    employees = sub.add_parser("employees", help="按姓名、邮箱等关键字查询可选员工 ID")
    employees.add_argument("--keyword", required=True)
    employees.set_defaults(handler=cmd_employees)

    categories = sub.add_parser("categories", help="查询当前用户权限内合同类型")
    categories.add_argument("--keyword")
    categories.set_defaults(handler=cmd_categories)

    detail = sub.add_parser("task-detail", help="读取任务动作权限和稳定执行字段")
    detail.add_argument("--task-id", required=True)
    detail.add_argument("--scope", choices=("auto", *TASK_PATHS.keys()), default="auto")
    detail.set_defaults(handler=cmd_task_detail)

    endorse = sub.add_parser("endorse", help="预览或执行加签")
    endorse.add_argument("--task-id", required=True)
    endorse.add_argument("--employee-id", action="append", required=True)
    endorse.add_argument("--mode", choices=ENDORSE_COMMANDS.keys(), required=True)
    endorse.add_argument("--remark", required=True)
    endorse.add_argument("--endorsee-editable", choices=("editable", "read-only"))
    add_write_gate(endorse)
    endorse.set_defaults(handler=cmd_endorse)

    approve = sub.add_parser("approve", help="预览或执行审批通过")
    approve.add_argument("--task-id", required=True)
    approve.add_argument("--remark")
    add_write_gate(approve)
    approve.set_defaults(handler=cmd_approve)

    transfer = sub.add_parser("transfer", help="预览或执行转办")
    transfer.add_argument("--task-id", required=True)
    transfer.add_argument("--employee-id", required=True)
    transfer.add_argument("--remark")
    add_write_gate(transfer)
    transfer.set_defaults(handler=cmd_transfer)

    notice = sub.add_parser("notice", help="预览或执行抄送")
    notice.add_argument("--task-id", required=True)
    notice.add_argument("--employee-id", action="append", required=True)
    notice.add_argument("--remark")
    notice.add_argument(
        "--allow-repeat",
        action="store_true",
        help="仅在用户明确要求再次抄送给已收件人时使用",
    )
    add_write_gate(notice)
    notice.set_defaults(handler=cmd_notice)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    try:
        auth = load_auth(args.profile)
        emit(args.handler(auth, args))
        return 0
    except SkillError as exc:
        error = {"type": exc.kind, "message": exc.message}
        if getattr(args, "diagnostic", False) and exc.details is not None:
            error["details"] = exc.details
        emit({"ok": False, "error": error})
        return 2
    except KeyboardInterrupt:
        emit({"ok": False, "error": {"type": "cancelled", "message": "操作已取消"}})
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
