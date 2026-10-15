#!/usr/bin/env python3
"""Export analyzer report.details to XLSX, with a dependency-free CSV fallback.

No network calls or dependency installation. XLSX uses the Codex bundled
@oai/artifact-tool runtime, or caller-supplied --node and --node-modules.
"""
from __future__ import annotations

import argparse
import csv
import json
import math
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from datetime import datetime, timezone, timedelta
import xml.etree.ElementTree as ET
import zipfile

HEADERS = ["合同名称", "合同ID", "合同编号", "节点名称", "操作人", "操作结果",
           "进入时间", "完成操作时间", "用时(h)", "节点类型", "流程实例ID", "任务ID",
           "计入总览", "计入个人统计", "排除原因"]
ID_COLUMNS = (1, 10, 11)
HEADER_ROW = 7
EXCEL_MAX_ROWS = 1048576
NODE_NAMES = {1: "审批", 2: "盖章", 3: "归档", "1": "审批", "2": "盖章", "3": "归档"}


def text_value(value):
    if value is None:
        return ""
    if isinstance(value, (list, dict)):
        return json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    return str(value)


def identifier(value):
    if value is None:
        return ""
    if isinstance(value, bool) or not isinstance(value, (str, int)):
        raise ValueError("ID 必须是字符串或整数，不能使用可能丢失精度的浮点数")
    return str(value)


def safe_text(value):
    """Treat untrusted labels as literal cells, including leading whitespace."""
    value = text_value(value)
    if value and (value.lstrip().startswith(("=", "+", "-", "@")) or value[0] in "\t\r\n"):
        return "'" + value
    return value


def timezone_info(name):
    if not name or name == "Asia/Shanghai":
        # No tzdata installation is needed for the skill's default period.
        return timezone(timedelta(hours=8))
    if name in ("UTC", "Etc/UTC", "Z"):
        return timezone.utc
    from zoneinfo import ZoneInfo
    return ZoneInfo(name)


def local_timestamp(value, zone):
    if value is None or value == "":
        return None, ""
    if not isinstance(value, str):
        raise ValueError("明细时间必须是 ISO 日期时间字符串")
    try:
        stamp = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        # Preserve invalid source text; never turn an unknown time into zero.
        return None, value
    if stamp.tzinfo is None:
        stamp = stamp.replace(tzinfo=zone)
    local = stamp.astimezone(zone)
    naive = local.replace(tzinfo=None)
    serial = (naive - datetime(1899, 12, 30)).total_seconds() / 86400
    return serial, local.isoformat(sep=" ", timespec="seconds")


def yes_no(value):
    if value is True:
        return "是"
    if value is False:
        return "否"
    return "未标记"


def prepare_report(report):
    if not isinstance(report, dict) or not isinstance(report.get("details"), list):
        raise ValueError("输入必须包含 details 明细数组；不能把缺失明细当作空数据")
    meta = report.get("meta") or {}
    zone_name = meta.get("timezone") or "Asia/Shanghai"
    zone = timezone_info(zone_name)
    xlsx_rows, csv_rows, warnings = [], [], []
    invalid_times = 0
    for detail in report["details"]:
        if not isinstance(detail, dict):
            raise ValueError("details 中每条明细必须是对象")
        seconds = detail.get("duration_seconds")
        if seconds is not None and (isinstance(seconds, bool) or not isinstance(seconds, (float, int))
                                    or not math.isfinite(seconds) or seconds < 0):
            raise ValueError("duration_seconds 必须是非负有限数字或 null")
        created, created_text = local_timestamp(detail.get("created_at"), zone)
        ended, ended_text = local_timestamp(detail.get("ended_at"), zone)
        invalid_times += int(bool(created_text) and created is None) + int(bool(ended_text) and ended is None)
        row = [safe_text(detail.get("contract_name")), identifier(detail.get("contract_id")),
               safe_text(detail.get("contract_number")), safe_text(detail.get("node_name")),
               safe_text(detail.get("operator_name")), safe_text(detail.get("result")),
               created if created is not None else safe_text(created_text),
               ended if ended is not None else safe_text(ended_text),
               seconds / 3600 if seconds is not None else None,
               safe_text(NODE_NAMES.get(detail.get("node_type"), detail.get("node_type"))),
               identifier(detail.get("process_instance_id")), identifier(detail.get("task_instance_id")),
               yes_no(detail.get("included_overview")), yes_no(detail.get("included_personal")),
               safe_text(detail.get("exclusion_reason"))]
        for col in ID_COLUMNS:
            row[col] = safe_text(row[col])
        xlsx_rows.append(row)
        csv_row = list(row)
        csv_row[6], csv_row[7] = safe_text(created_text), safe_text(ended_text)
        # CSV has no text type; an apostrophe prevents Excel coercing long IDs.
        for col in ID_COLUMNS:
            if re.fullmatch(r"[0-9]+", csv_row[col] or ""):
                csv_row[col] = "'" + csv_row[col]
        csv_rows.append(csv_row)
    if invalid_times:
        warnings.append(f"{invalid_times} 个时间值无法解析，已保留原始文本")
    scope = meta.get("scope", report.get("scope", "当前授权账号可见、符合筛选条件的流程记录"))
    if isinstance(scope, dict):
        resolved = scope.get("resolved") if isinstance(scope.get("resolved"), dict) else {}
        scope = resolved.get("displayName") or resolved.get("name") or scope.get("query") or "当前授权账号可见、符合筛选条件的流程记录"
    completeness = ("；数据完整性：存在读取缺口，详细诊断见JSON"
                    if meta.get("collection_status") == "partial" else "")
    payload = {
        "headers": HEADERS, "rows": xlsx_rows,
        "period": safe_text(f"统计周期：{meta.get('start', '未提供')} 至 {meta.get('end', '未提供')}（结束时间不含，{zone_name}）"),
        "scope": safe_text("统计范围：" + text_value(scope)),
        "source": safe_text(f"环境：{meta.get('environment', '未提供')}；数据来源：{text_value(meta.get('source', '流程任务记录'))}"),
        "coverage": safe_text(f"统计周期内涉及合同：{meta.get('period_contract_count', 0)} 份{completeness}"),
    }
    return payload, csv_rows, warnings


def runtime_paths(node=None, node_modules=None):
    runtime = Path.home() / ".cache/codex-runtimes/codex-primary-runtime/dependencies"
    node = node or os.environ.get("CONTRACT_EFFICIENCY_NODE")
    modules = node_modules or os.environ.get("CONTRACT_EFFICIENCY_NODE_MODULES")
    bundled_node = runtime / "node/bin/node"
    if not node:
        node = str(bundled_node) if bundled_node.is_file() else shutil.which("node")
    if not modules:
        candidates = [Path(p) for p in os.environ.get("NODE_PATH", "").split(os.pathsep) if p]
        candidates.append(runtime / "node/node_modules")
        modules = next((str(p) for p in candidates if (p / "@oai/artifact-tool").is_dir()), None)
    if not node or not (Path(node).is_file() or shutil.which(str(node))):
        raise RuntimeError("未找到 Node.js；可通过 --node 指定运行程序")
    if not modules or not (Path(modules) / "@oai/artifact-tool").is_dir():
        raise RuntimeError("未找到 @oai/artifact-tool；可通过 --node-modules 指定已有依赖目录")
    return str(node), Path(modules).resolve()


def validate_xlsx(path, expected_rows, expected_values=None):
    """Verify saved rows, literal identifiers, filters and frozen header."""
    ns = {"s": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
    with zipfile.ZipFile(path) as archive:
        if archive.testzip() is not None:
            raise RuntimeError("XLSX ZIP 校验失败")
        sheet = ET.fromstring(archive.read("xl/worksheets/sheet1.xml"))
        data_rows = [r for r in sheet.findall("s:sheetData/s:row", ns) if int(r.attrib["r"]) > HEADER_ROW]
        if len(data_rows) != expected_rows:
            raise RuntimeError(f"XLSX 明细行数不一致：期望 {expected_rows}，实际 {len(data_rows)}")
        if sheet.findall(".//s:f", ns):
            raise RuntimeError("明细中出现非预期公式")
        strings = []
        if "xl/sharedStrings.xml" in archive.namelist():
            strings = ["".join(item.itertext()) for item in ET.fromstring(archive.read("xl/sharedStrings.xml"))]
        for row_index, row in enumerate(data_rows):
            for cell in row.findall("s:c", ns):
                column = re.sub(r"[0-9]", "", cell.attrib["r"])
                if column in ("B", "K", "L") and cell.find("s:v", ns) is not None:
                    if cell.attrib.get("t") not in ("s", "inlineStr", "str"):
                        raise RuntimeError("XLSX 标识符未以文本保存")
                    if expected_values is not None:
                        index = {"B": 1, "K": 10, "L": 11}[column]
                        expected = expected_values[row_index][index]
                        actual = cell.find("s:v", ns).text or ""
                        if cell.attrib.get("t") == "s":
                            actual = strings[int(actual)]
                        if re.fullmatch(r"[0-9]+", expected) and actual != expected:
                            raise RuntimeError("XLSX 长 ID 内容发生变化")
        pane = sheet.find("s:sheetViews/s:sheetView/s:pane", ns)
        if pane is None or pane.attrib.get("ySplit") != str(HEADER_ROW):
            raise RuntimeError("XLSX 表头冻结未保存")
        table_files = [n for n in archive.namelist() if n.startswith("xl/tables/table") and n.endswith(".xml")]
        if not any(ET.fromstring(archive.read(n)).find("s:autoFilter", ns) is not None for n in table_files):
            raise RuntimeError("XLSX 筛选功能未保存")


def write_csv(path, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8-sig", newline="", dir=path.parent,
                                         prefix=".details-", suffix=".csv", delete=False) as handle:
            temporary = Path(handle.name)
            writer = csv.writer(handle)
            writer.writerow(HEADERS)
            writer.writerows(rows)
        with temporary.open(encoding="utf-8-sig", newline="") as handle:
            parsed = csv.reader(handle)
            if next(parsed) != HEADERS or sum(1 for _ in parsed) != len(rows):
                raise RuntimeError("CSV 行数校验失败")
        os.replace(temporary, path)
    finally:
        if temporary and temporary.exists():
            temporary.unlink()


def export_report(report, output, *, csv_only=False, node=None, node_modules=None, preview=None):
    payload, csv_rows, warnings = prepare_report(report)
    output = Path(output).expanduser().resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    reason = "用户选择 CSV" if csv_only else None
    if not csv_only:
        try:
            if len(payload["rows"]) + HEADER_ROW > EXCEL_MAX_ROWS:
                raise RuntimeError("明细超过 Excel 单表行数上限，全部记录改为 CSV，不截断")
            node_exec, modules = runtime_paths(node, node_modules)
            with tempfile.TemporaryDirectory(prefix="approval-details-", dir=output.parent) as scratch:
                scratch = Path(scratch)
                shutil.copy2(Path(__file__).with_suffix(".mjs"), scratch / "export_details.mjs")
                if os.name == "nt":
                    subprocess.run(["cmd", "/c", "mklink", "/J", str(scratch / "node_modules"), str(modules)],
                                   check=True, capture_output=True, text=True)
                else:
                    (scratch / "node_modules").symlink_to(modules, target_is_directory=True)
                prepared = scratch / "payload.json"
                prepared.write_text(json.dumps(payload, ensure_ascii=False, allow_nan=False), encoding="utf-8")
                temp_xlsx = scratch / "details.xlsx"
                command = [node_exec, str(scratch / "export_details.mjs"), str(prepared), str(temp_xlsx)]
                if preview:
                    command.append(str(Path(preview).expanduser().resolve()))
                result = subprocess.run(command, capture_output=True, text=True)
                if result.returncode:
                    # Builder errors carry no source rows. Do not echo a raw report or arbitrary runtime output.
                    error = result.stderr.strip().splitlines()
                    raise RuntimeError("Excel 生成失败：" + (error[-1][:300] if error else f"退出码 {result.returncode}"))
                validate_xlsx(temp_xlsx, len(csv_rows), payload["rows"])
                destination = output.with_suffix(".xlsx")
                os.replace(temp_xlsx, destination)
            return {"format": "xlsx", "path": str(destination), "row_count": len(csv_rows),
                    "fallback_reason": None, "warnings": warnings}
        except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, zipfile.BadZipFile, ET.ParseError) as exc:
            reason = str(exc)
    destination = output.with_suffix(".csv")
    write_csv(destination, csv_rows)
    warnings.append("CSV 数字型 ID 前加单引号，以免 Excel 自动转换导致精度丢失；按文本导入时可去掉该前缀")
    return {"format": "csv", "path": str(destination), "row_count": len(csv_rows),
            "fallback_reason": reason, "warnings": warnings}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="包含 details 数组的分析 report.json")
    parser.add_argument("--output", required=True, help="期望的明细 .xlsx 路径；失败写同名 .csv")
    parser.add_argument("--csv-only", action="store_true")
    parser.add_argument("--node", help="已有 Node.js 的路径")
    parser.add_argument("--node-modules", help="包含 @oai/artifact-tool 的已有 node_modules 目录")
    parser.add_argument("--preview", help="可选：生成 Excel 首屏 PNG 供人工核对")
    args = parser.parse_args(argv)
    try:
        report = json.loads(Path(args.input).read_text(encoding="utf-8"))
        result = export_report(report, args.output, csv_only=args.csv_only, node=args.node,
                               node_modules=args.node_modules, preview=args.preview)
    except (OSError, ValueError, RuntimeError) as exc:
        print(json.dumps({"status": "error", "message": str(exc)}, ensure_ascii=False))
        return 1
    print(json.dumps({"status": "ok", **result}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
