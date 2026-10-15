import csv
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

MODULE = Path(__file__).resolve().parents[1] / "scripts/export_details.py"
spec = importlib.util.spec_from_file_location("export_details", MODULE)
exporter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(exporter)


def report(rows=None):
    return {"meta": {"start": "2026-01-01T00:00:00+08:00", "end": "2026-09-23T00:00:00+08:00",
                     "timezone": "Asia/Shanghai", "source": "合成测试数据", "environment": "test",
                     "scope": "示例部门", "coverage": {"readable_businesses": 1},
                     "period_contract_count": 1},
            "details": rows if rows is not None else [
                {"contract_name": "测试服务合同", "contract_id": 9876543210987654321,
                 "contract_number": "HT-2026-001", "node_name": "部门负责人审批", "operator_name": "示例甲",
                 "result": "审批通过", "created_at": "2026-09-20T08:00:00+08:00",
                 "ended_at": "2026-09-20T09:30:00+08:00", "duration_seconds": 5400,
                 "node_type": 1, "process_instance_id": "1234567890123456789", "task_instance_id": "T-001",
                 "included_overview": True, "included_personal": True, "exclusion_reason": ""}]}


class ExportDetailsTests(unittest.TestCase):
    def test_required_column_order_and_long_identifier(self):
        payload, rows, warnings = exporter.prepare_report(report())
        self.assertEqual(payload["headers"][:9], ["合同名称", "合同ID", "合同编号", "节点名称", "操作人", "操作结果", "进入时间", "完成操作时间", "用时(h)"])
        self.assertEqual(payload["rows"][0][1], "9876543210987654321")
        self.assertEqual(rows[0][1], "'9876543210987654321")
        self.assertEqual(payload["rows"][0][8], 1.5)
        self.assertEqual(warnings, [])
        self.assertEqual(payload["coverage"], "统计周期内涉及合同：1 份")
        self.assertNotIn("readable_businesses", payload["coverage"])

    def test_csv_is_bom_and_preserves_every_row(self):
        source = report()
        source["details"] *= 257
        with tempfile.TemporaryDirectory() as directory:
            result = exporter.export_report(source, Path(directory) / "details.xlsx", csv_only=True)
            output = Path(result["path"])
            self.assertTrue(output.read_bytes().startswith(b"\xef\xbb\xbf"))
            with output.open(encoding="utf-8-sig", newline="") as handle:
                rows = list(csv.reader(handle))
            self.assertEqual(len(rows), 258)
            self.assertEqual(result["row_count"], 257)

    def test_runtime_failure_falls_back_with_reason(self):
        with tempfile.TemporaryDirectory() as directory:
            with mock.patch.object(exporter, "runtime_paths", side_effect=RuntimeError("缺少测试运行环境")):
                result = exporter.export_report(report(), Path(directory) / "details.xlsx")
            self.assertEqual(result["format"], "csv")
            self.assertIn("缺少测试运行环境", result["fallback_reason"])
            self.assertTrue(Path(result["path"]).is_file())

    def test_injection_protection_for_text_and_identifiers(self):
        source = report()
        source["details"][0].update(contract_name="=HYPERLINK(\"https://invalid\")", contract_number="  +1",
                                     contract_id="@SUM(A1)", operator_name="\tDDE", exclusion_reason="-2")
        payload, rows, _ = exporter.prepare_report(source)
        for index in (0, 1, 2, 4, 14):
            self.assertTrue(payload["rows"][0][index].startswith("'"))
            self.assertTrue(rows[0][index].startswith("'"))

    def test_empty_report_is_header_only_csv(self):
        with tempfile.TemporaryDirectory() as directory:
            result = exporter.export_report(report([]), Path(directory) / "empty.xlsx", csv_only=True)
            with Path(result["path"]).open(encoding="utf-8-sig", newline="") as handle:
                self.assertEqual(list(csv.reader(handle)), [exporter.HEADERS])
            self.assertEqual(result["row_count"], 0)

    def test_null_pending_duration_stays_blank(self):
        source = report()
        source["details"][0].update(ended_at=None, duration_seconds=None, result="在途", included_personal=False)
        payload, rows, _ = exporter.prepare_report(source)
        self.assertEqual(payload["rows"][0][7], "")
        self.assertIsNone(payload["rows"][0][8])
        self.assertEqual(payload["rows"][0][13], "否")

    def test_dates_convert_to_reporting_timezone(self):
        source = report()
        source["details"][0]["created_at"] = "2026-09-20T00:00:00Z"
        payload, rows, _ = exporter.prepare_report(source)
        self.assertIsInstance(payload["rows"][0][6], float)
        self.assertEqual(rows[0][6], "2026-09-20 08:00:00+08:00")

    def test_invalid_dates_preserved_and_disclosed(self):
        source = report()
        source["details"][0]["created_at"] = "invalid source time"
        payload, _, warnings = exporter.prepare_report(source)
        self.assertEqual(payload["rows"][0][6], "invalid source time")
        self.assertEqual(len(warnings), 1)

    def test_float_identifiers_are_rejected(self):
        source = report()
        source["details"][0]["contract_id"] = 9.876543210987654e18
        with self.assertRaisesRegex(ValueError, "浮点数"):
            exporter.prepare_report(source)

    def test_missing_details_is_error_not_empty(self):
        with self.assertRaisesRegex(ValueError, "缺失明细"):
            exporter.prepare_report({"meta": {}})

    def test_invalid_duration_is_not_silently_zero(self):
        for value in (-1, float("nan"), "3600", True):
            source = report()
            source["details"][0]["duration_seconds"] = value
            with self.subTest(value=value), self.assertRaisesRegex(ValueError, "非负"):
                exporter.prepare_report(source)

    def test_multiactor_record_is_not_expanded(self):
        source = report()
        source["details"][0].update(operator_name="示例甲、示例乙", included_personal=False,
                                     exclusion_reason="多人节点")
        payload, _, _ = exporter.prepare_report(source)
        self.assertEqual(len(payload["rows"]), 1)
        self.assertEqual(payload["rows"][0][4], "示例甲、示例乙")

    def test_excel_row_limit_falls_back_without_truncation(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(exporter, "EXCEL_MAX_ROWS", 7):
            result = exporter.export_report(report(), Path(directory) / "details.xlsx")
        self.assertEqual(result["format"], "csv")
        self.assertEqual(result["row_count"], 1)
        self.assertIn("不截断", result["fallback_reason"])

    def test_cli_returns_output_metadata(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "input.json"
            source.write_text(json.dumps(report(), ensure_ascii=False), encoding="utf-8")
            with mock.patch("sys.stdout") as stdout:
                self.assertEqual(exporter.main(["--input", str(source), "--output", str(Path(directory) / "data.xlsx"), "--csv-only"]), 0)
            self.assertIn('"row_count": 1', stdout.write.call_args_list[0].args[0])


if __name__ == "__main__":
    unittest.main()
