import importlib.util
from pathlib import Path
import unittest


MODULE_PATH = Path(__file__).resolve().parents[1] / "scripts" / "analyze_records.py"
SPEC = importlib.util.spec_from_file_location("analyze_records", MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def task(task_id="t1", **changes):
    value = {
        "processDefinitionId": "definition1", "processInstanceId": "instance1",
        "processTaskInstanceId": task_id, "nodeId": "node1", "nodeName": "审核",
        "createTime": "2026-09-01 09:00:00", "endTime": "2026-09-01 10:00:00",
        "taskStatusCode": 1, "commandTypeCode": 1,
        "assigneeEmployees": [{"employeeId": "e1", "displayName": "张三",
            "departments": [{"departmentId": "d1", "displayName": "法务部"}]}],
    }
    value.update(changes)
    return value


def bundle(*records, node_type=1, todo=False):
    return {"meta": {"start": "2026-09-01", "end": "2026-10-01",
        "timezone": "Asia/Shanghai", "timezone_confirmed": True,
        "timezone_source": "合成测试夹具，时间明确按 Asia/Shanghai 构造", "coverage": "测试样本"},
        "node_participant_evidence": [{"processInstanceId": r.get("processInstanceId"),
            "processDefinitionId": r.get("processDefinitionId"), "nodeId": r.get("nodeId"),
            "employee_ids": sorted({p["employeeId"] for x in records if x.get("processInstanceId") == r.get("processInstanceId") and x.get("nodeId") == r.get("nodeId") for p in x.get("assigneeEmployees", []) if p.get("employeeId")}),
            "complete": True, "source": "合成测试的原始完整分配清单"}
            for r in records if r.get("processInstanceId") and r.get("processDefinitionId") and r.get("nodeId") and r.get("assigneeEmployees")],
        "instances": [{"processInstanceId": "instance1", "businessId": "contract1", "businessTypeCode": 1,
            "response": {"success": True, "code": 200, "data": {
                "todo" if todo else "done": [{"nodeId": "node1", "nodeTypeCode": node_type, "records": list(records)}]}}}]}


class AnalyzeRecordsTests(unittest.TestCase):
    def test_weighted_mean_not_mean_of_people_means(self):
        rows = [task("t1"), task("t2"), task("t3", endTime="2026-09-01 18:00:00",
                assigneeEmployees=[{"employeeId": "e2", "departments": [{"departmentId": "d1"}]}])]
        report = MODULE.analyze(bundle(*rows))
        self.assertEqual(report["by_department"][0]["completed_count"], 3)
        self.assertAlmostEqual(report["by_department"][0]["average_seconds"], 11 * 3600 / 3)

    def test_archive_and_completed_inside_todo(self):
        report = MODULE.analyze(bundle(task(), node_type=3, todo=True))
        self.assertEqual(report["by_process_node"][0]["node_type"], "归档")
        self.assertEqual(next(r for r in report["by_approver"] if r["node_type_code"] == 3)["average_seconds"], 3600)

    def test_revisited_node_counts_distinct_tasks(self):
        report = MODULE.analyze(bundle(task("first"), task("retry")))
        self.assertEqual(report["by_process_node"][0]["completed_count"], 2)

    def test_identical_duplicates_count_once(self):
        report = MODULE.analyze(bundle(task(), task()))
        self.assertEqual(report["by_approver"][0]["completed_count"], 1)
        self.assertEqual(report["diagnostics"]["duplicate_copies_ignored"], 1)

    def test_response_and_unique_business_instance_counts_are_distinct(self):
        value = bundle(task("new"), task("old", processInstanceId="old-instance"))
        value["instances"].append(bundle(task("new"), task("old", processInstanceId="old-instance"))["instances"][0])
        report = MODULE.analyze(value)
        self.assertEqual(report["meta"]["input_response_count"], 2)
        self.assertEqual(report["meta"]["unique_business_count"], 1)
        self.assertEqual(report["meta"]["unique_process_instance_count"], 2)
        self.assertEqual(report["meta"]["period_contract_count"], 1)
        text = MODULE.render_markdown(report)
        self.assertIn("统计周期内涉及合同数：1份", text)
        self.assertNotIn("输入响应数", text)
        self.assertNotIn("已识别业务数", text)
        self.assertNotIn("记录涉及流程实例数", text)
        self.assertNotIn("输入实例数", text)

    def test_missing_business_identity_is_diagnosed(self):
        value = bundle(task())
        del value["instances"][0]["businessId"]
        report = MODULE.analyze(value)
        self.assertEqual(report["meta"]["input_response_count"], 1)
        self.assertEqual(report["meta"]["unique_business_count"], 0)
        self.assertEqual(report["meta"]["unique_process_instance_count"], 1)
        self.assertEqual(report["diagnostics"]["responses_missing_business_identity"], 1)

    def test_ui_only_changes_do_not_conflict(self):
        first = task(createTimeSpan={"days": 1}, endTimeSpan={"hours": 2})
        second = task(createTimeSpan={"days": 2}, endTimeSpan={"hours": 26}, taskComment="更新展示")
        second["assigneeEmployees"][0]["avatarUrl"] = "https://example.invalid/new.png"
        second["assigneeEmployees"][0]["displayName"] = "更新姓名"
        second["assigneeEmployees"][0]["departments"][0]["displayName"] = "更新部门名"
        report = MODULE.analyze(bundle(first, second))
        self.assertEqual(report["by_approver"][0]["completed_count"], 1)
        self.assertEqual(report["diagnostics"]["duplicate_copies_ignored"], 1)
        self.assertEqual(report["diagnostics"]["conflicting_tasks_excluded"], 0)

    def test_missing_record_instance_id_not_inferred_from_parent(self):
        value = bundle(task(processInstanceId=None))
        value["instances"][0]["response"]["data"]["done"][0]["processInstanceId"] = "new-instance"
        report = MODULE.analyze(value)
        self.assertEqual(sum(r["completed_count"] for r in report["by_approver"]), 0)
        self.assertEqual(report["diagnostics"]["missing_dedup_id"], 1)

    def test_conflicting_duplicate_excludes_all_versions(self):
        report = MODULE.analyze(bundle(task(), task(endTime="2026-09-01 11:00:00")))
        self.assertEqual(sum(r["completed_count"] for r in report["by_approver"]), 0)
        self.assertEqual(report["diagnostics"]["conflicting_tasks_excluded"], 1)

    def test_failed_response_not_empty_data(self):
        value = bundle()
        value["instances"][0]["response"] = {"success": False, "code": 403, "data": {"done": []}}
        with self.assertRaisesRegex(ValueError, "禁止把权限"):
            MODULE.analyze(value)

    def test_missing_negative_and_cancelled_not_zero(self):
        report = MODULE.analyze(bundle(task("good"), task("missing", endTime=None),
            task("negative", endTime="2026-09-01 08:00:00"), task("cancelled", taskStatusCode=3)))
        self.assertEqual(report["by_approver"][0]["completed_count"], 1)
        self.assertEqual(report["by_approver"][0]["average_seconds"], 3600)
        self.assertEqual(report["diagnostics"]["invalid_or_missing_time"], 1)
        self.assertEqual(report["diagnostics"]["negative_duration_excluded"], 1)

    def test_pending_separate_and_forecast_ignored(self):
        value = bundle(task("done"), task("pending", endTime=None, taskStatusCode=0))
        value["instances"][0]["response"]["data"]["forecast"] = [{"nodeTypeCode": 1, "records": [task("future")]}]
        report = MODULE.analyze(value)
        self.assertEqual(report["by_approver"][0]["completed_count"], 1)
        self.assertEqual(report["by_approver"][0]["in_progress_count"], 1)
        self.assertEqual(report["by_approver"][0]["average_seconds"], 3600)

    def test_pending_only_has_no_zero_duration(self):
        report = MODULE.analyze(bundle(task(endTime=None, taskStatusCode=0)))
        self.assertEqual(report["by_approver"][0]["in_progress_count"], 1)
        self.assertEqual(report["by_approver"][0]["average_seconds"], 0)
        self.assertEqual(report["by_approver"][0]["total_seconds"], 0)

    def test_same_name_in_different_definitions_not_merged(self):
        report = MODULE.analyze(bundle(task("a"), task("b", processDefinitionId="definition2")))
        self.assertEqual(len(report["by_process_node"]), 2)

    def test_multiple_assignees_not_replicated(self):
        report = MODULE.analyze(bundle(task(assigneeEmployees=[{"employeeId": "a"}, {"employeeId": "b"}])))
        self.assertEqual(sum(r["completed_count"] for r in report["by_approver"]), 0)
        self.assertEqual(report["by_department"], [])
        self.assertEqual(report["by_process_node"][0]["unassigned_count"], 1)

    def test_multiple_departments_form_one_combination(self):
        report = MODULE.analyze(bundle(task(assigneeEmployees=[{"employeeId": "e1", "departments": [
            {"departmentId": "d2", "displayName": "部门二"}, {"departmentId": "d1", "displayName": "部门一"}]}])))
        self.assertEqual(len(report["by_department"]), 1)
        self.assertEqual(report["by_department"][0]["department_ids"], ["d1", "d2"])
        self.assertEqual(report["by_department"][0]["completed_count"], 1)

    def test_conflicting_current_department_snapshots_do_not_split_person(self):
        value = bundle(task("a"), task("b", assigneeEmployees=[{"employeeId": "e1", "departments": [{"departmentId": "d2"}]}]))
        report = MODULE.analyze(value)
        self.assertEqual(report["by_approver"][0]["completed_count"], 2)
        self.assertEqual(report["by_process_node"][0]["completed_count"], 2)
        self.assertEqual(report["by_department"], [])
        self.assertEqual(report["diagnostics"]["inconsistent_department_employees"], 1)
        self.assertEqual(report["diagnostics"]["inconsistent_department_tasks_excluded"], 2)
        filtered = MODULE.analyze(value, department_ids=["d1"])
        self.assertEqual(filtered["by_department"], [])
        self.assertEqual(filtered["diagnostics"]["inconsistent_department_employees"], 1)

    def test_half_open_end_and_start(self):
        report = MODULE.analyze(bundle(task("start", createTime="2026-08-31 23:00:00", endTime="2026-09-01 00:00:00"),
            task("end", endTime="2026-10-01 00:00:00")))
        self.assertEqual(report["by_approver"][0]["completed_count"], 1)

    def test_timezone_must_be_confirmed(self):
        value = bundle(task())
        del value["meta"]["timezone_confirmed"]
        with self.assertRaisesRegex(ValueError, "时区未确认"):
            MODULE.analyze(value)

    def test_timezone_source_must_be_nonempty(self):
        for source in (None, "", "   "):
            value = bundle(task())
            value["meta"]["timezone_source"] = source
            with self.assertRaisesRegex(ValueError, "时区来源缺失"):
                MODULE.analyze(value)

    def test_source_metadata_preserved_and_rendered(self):
        value = bundle(task())
        extra = {"environment": "test", "collected_at": "2026-09-23T11:00:00+08:00", "source": "synthetic fixture"}
        value["meta"].update(extra)
        report = MODULE.analyze(value)
        for key, expected in extra.items():
            self.assertEqual(report["meta"][key], expected)
            self.assertIn(expected, MODULE.render_markdown(report))
        self.assertIn("节点表平均=节点任务平均用时", MODULE.render_markdown(report))

    def test_false_or_object_is_not_empty_array(self):
        for invalid in (False, {}, 0, ""):
            with self.subTest(invalid=invalid):
                for field in ("done", "todo", "forecast"):
                    value = bundle(task())
                    value["instances"][0]["response"]["data"][field] = invalid
                    with self.assertRaisesRegex(ValueError, "必须为数组"):
                        MODULE.analyze(value)
                value = bundle(task())
                value["instances"][0]["response"]["data"]["done"][0]["records"] = invalid
                with self.assertRaisesRegex(ValueError, "必须为数组"):
                    MODULE.analyze(value)
                with self.assertRaisesRegex(ValueError, "必须为数组"):
                    MODULE.analyze(bundle(task(assigneeEmployees=invalid)))

    def test_null_arrays_are_allowed(self):
        value = bundle()
        value["instances"][0]["response"]["data"] = {"done": None, "todo": [], "forecast": None}
        self.assertEqual(MODULE.analyze(value)["by_approver"], [])

    def test_only_human_handling_commands(self):
        report = MODULE.analyze(bundle(task("transfer", commandTypeCode=3),
            task("endorse", commandTypeCode=10), task("reject", commandTypeCode=2, taskStatusCode=2)))
        self.assertEqual(report["by_approver"][0]["completed_count"], 2)
        self.assertEqual(report["by_approver"][0]["rejected_count"], 1)

    def test_person_separate_node_types(self):
        value = bundle(task())
        value["instances"][0]["response"]["data"]["done"].append({"nodeTypeCode": 2, "records": [task("seal")]})
        self.assertEqual(len(MODULE.analyze(value)["by_approver"]), 3)

    def test_id_precision_and_float_rejection(self):
        value = bundle(task(assigneeEmployees=[{"employeeId": 9999999999999999999}]))
        self.assertEqual(MODULE.analyze(value)["by_approver"][0]["employee_id"], "9999999999999999999")
        with self.assertRaisesRegex(ValueError, "浮点数"):
            MODULE.analyze(bundle(task(processTaskInstanceId=1.1)))

    def test_markdown_escapes_names(self):
        report = MODULE.analyze(bundle(task(assigneeEmployees=[{"employeeId": "e1", "displayName": "A|B\n<script>[link]"}])))
        text = MODULE.render_markdown(report)
        self.assertIn("A\\|B / &lt;script&gt;\\[link\\]", text)
        self.assertNotIn("<script>", text)

    def test_local_filters_intersect(self):
        value = bundle(task())
        self.assertEqual(sum(r["completed_count"] for r in MODULE.analyze(value, ["not-found"])["by_approver"]), 0)
        self.assertEqual(MODULE.analyze(value, ["e1"], ["d1"])["by_approver"][0]["completed_count"], 1)

    def test_definition_filter_uses_each_historical_record(self):
        value = bundle(task("old", processDefinitionId="old"), task("new", processDefinitionId="new"))
        value["instances"][0]["processDefinitionId"] = "new"
        report = MODULE.analyze(value, ["e1"], ["d1"], [1], ["old", "other"])
        self.assertEqual(report["by_approver"][0]["completed_count"], 1)
        self.assertEqual(report["by_process_node"][0]["process_definition_id"], "old")
        self.assertEqual(report["meta"]["filters"]["process_definition_ids"], ["old", "other"])


if __name__ == "__main__":
    unittest.main()
