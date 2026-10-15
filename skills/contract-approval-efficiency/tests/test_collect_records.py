import contextlib
import copy
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location("collection", Path(__file__).parents[1] / "scripts/collect_records.py")
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)


def ok(data):
    return {"success": True, "code": 200, "data": data}


def business(**values):
    return {"businessId": "9000000000000000001", "businessTypeCode": 0,
            "contractName": "合成合同", **values}


def records():
    return {"done": [{"nodeId": "n", "nodeTypeCode": 1, "taskModeCode": 2,
        "records": [
            {"processInstanceId": "old", "processDefinitionId": "d", "nodeId": "n",
             "processTaskInstanceId": "t1", "businessId": "historical-business", "businessTypeCode": 2,
             "assigneeEmployees": [{"employeeId": "a", "name": "甲", "phone": "private"}],
             "createTime": "2025-01-01 00:00:00", "endTime": "2025-01-02 00:00:00", "commandTypeCode": 1},
            {"processInstanceId": "current", "processDefinitionId": "d", "nodeId": "n",
             "processTaskInstanceId": "t2", "assigneeEmployees": [{"employeeId": "b", "name": "乙"}],
             "commandTypeCode": 11, "parentProcessTaskInstanceId": "t1",
             "agentEmployees": [{"employeeId": "proxy", "phone": "private"}],
             "endorsementEmployees": [{"employeeId": "extra"}]}]}], "todo": [], "forecast": []}


class Fake:
    def __init__(self, *responses):
        self.responses = list(responses)
        self.calls = []

    def __call__(self, path, query):
        self.calls.append((path, copy.deepcopy(query)))
        if not self.responses:
            raise AssertionError("unexpected_request")
        return copy.deepcopy(self.responses.pop(0))


class CollectionTests(unittest.TestCase):
    def test_records_keep_all_people_automatic_old_and_historical_identity(self):
        fake = Fake(ok({"contractId": business()["businessId"], "businessTypeCode": 0,
                        "processInstanceId": "current", "approveRecord": records(),
                        "contractName": "详情名称", "body": "private"}))
        result = c.Collector(fake).collect_business(business())
        self.assertEqual(result["status"], "complete")
        self.assertEqual(len(fake.calls), 1)
        instance = result["instances"][0]
        self.assertEqual(instance["represented_process_instance_ids"], ["current", "old"])
        rows = list(c.record_rows(instance["response"]["data"]))
        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0]["businessId"], "historical-business")
        self.assertEqual(rows[1]["commandTypeCode"], 11)
        self.assertEqual(rows[1]["parentProcessTaskInstanceId"], "t1")
        self.assertEqual(rows[1]["agentEmployees"], [{"employeeId": "proxy"}])
        self.assertEqual(rows[1]["endorsementEmployees"], [{"employeeId": "extra"}])
        self.assertEqual(instance["contractName"], "详情名称")
        self.assertNotIn("private", json.dumps(result))

    def test_missing_current_instance_does_not_discard_historical_records(self):
        result = c.Collector(Fake(ok({"approveRecord": records()}))).collect_business(business())
        self.assertEqual(result["status"], "complete")
        self.assertEqual(result["instances"][0]["processInstanceId"], "")
        self.assertEqual(len(list(c.record_rows(result["instances"][0]["response"]["data"]))), 2)

    def test_missing_instance_without_record_evidence_is_gap_not_empty_success(self):
        for data in ({}, {"approveRecord": {}}, {"approveRecord": None}):
            result = c.Collector(Fake(ok(data))).collect_business(business())
            self.assertEqual(result["instances"], [])
            self.assertEqual(result["errors"][0]["error"], "missing_workflow_evidence")

    def test_explicit_empty_records_is_distinguished_from_missing_records(self):
        result = c.Collector(Fake(ok({"approveRecord": {"done": [], "todo": []}}))).collect_business(business())
        self.assertEqual(result["errors"], [])
        self.assertEqual(len(result["instances"]), 1)

    def test_permission_denied_is_not_bypassed(self):
        fake = Fake({"success": False, "code": 110004})
        result = c.Collector(fake).collect_business(business())
        self.assertEqual(len(fake.calls), 1)
        self.assertEqual(result["errors"][0]["code"], 110004)

    def test_documented_brief_fallback_uses_real_process_id(self):
        fake = Fake({"success": False, "code": 110002}, ok({"processInstanceId": "real"}), ok(records()))
        result = c.Collector(fake).collect_business(business())
        self.assertEqual(result["status"], "complete")
        self.assertEqual([call[0] for call in fake.calls], ["contract/info", "contract/briefInfo", "workflow/runtime/process/records"])
        self.assertEqual(fake.calls[-1][1]["processInstanceId"], "real")

    def test_records_business_failure_and_malformed_success_remain_gaps(self):
        for reply in ({"success": False, "code": 110002}, ok({}), ok({"done": {"not": "list"}})):
            result = c.Collector(Fake(reply)).collect_business(business(processInstanceId="p"))
            self.assertEqual(result["instances"], [])
            self.assertEqual(len(result["errors"]), 1)

    def test_all_explicit_history_instances_are_read_and_partial_failures_retained(self):
        fake = Fake(ok(records()), {"success": False, "code": 110004}, ok({"done": [], "todo": []}))
        result = c.Collector(fake).collect_business(business(processInstanceIds=["p1", "p2", "p3"]))
        self.assertEqual([call[1]["processInstanceId"] for call in fake.calls], ["p1", "p2", "p3"])
        self.assertEqual(len(result["instances"]), 2)
        self.assertEqual(result["status"], "partial")
        self.assertEqual(result["errors"][0]["processInstanceId"], "p2")

    def test_failed_cache_is_retried_and_success_cache_requires_opt_in_ttl(self):
        with tempfile.TemporaryDirectory() as directory:
            first = c.Collector(Fake({"success": False, "code": 110004}), directory, 60).collect_business(business())
            self.assertEqual(first["status"], "partial")
            fake = Fake(ok({"approveRecord": records()}))
            second = c.Collector(fake, directory, 60).collect_business(business())
            self.assertEqual(second["status"], "complete")
            self.assertEqual(len(fake.calls), 1)
            cached = c.Collector(Fake(), directory, 60).collect_business(business())
            self.assertTrue(cached["cache_reused"])
            fake = Fake(ok({"approveRecord": records()}))
            refreshed = c.Collector(fake, directory).collect_business(business())
            self.assertFalse(refreshed["cache_reused"])
            self.assertEqual(len(fake.calls), 1)

    def test_explicit_process_identity_change_invalidates_cache(self):
        with tempfile.TemporaryDirectory() as directory:
            c.Collector(Fake(ok(records())), directory, 60).collect_business(business(processInstanceId="old"))
            fake = Fake(ok(records()))
            result = c.Collector(fake, directory, 60).collect_business(business(processInstanceId="new"))
            self.assertFalse(result["cache_reused"])
            self.assertEqual(fake.calls[0][1]["processInstanceId"], "new")

    def test_business_mismatch_is_not_attributed_to_request(self):
        for data in ({"businessTypeCode": 2}, {"contractId": "different"}):
            result = c.Collector(Fake(ok(data))).collect_business(business())
            self.assertEqual(result["instances"], [])
            self.assertIn("mismatch", result["errors"][0]["error"])

    def test_raw_task_process_without_type_is_read_by_process_permission(self):
        raw_records = records()
        raw_records["done"][0]["records"][1].update(businessId="9000000000000000001", businessTypeCode=2)
        fake = Fake(ok(raw_records))
        result = c.Collector(fake).collect_business({"contract_id": "9000000000000000001", "process_instance_id": "current"})
        self.assertEqual(fake.calls[0][1], {"processInstanceId": "current"})
        self.assertEqual(result["instances"][0]["businessTypeCode"], 2)
        self.assertEqual(result["instances"][0]["businessId"], "9000000000000000001")

    def test_raw_task_unknown_wrapper_type_keeps_real_task_identity(self):
        fake = Fake(ok(records()))
        result = c.Collector(fake).collect_business({"contract_id": "9000000000000000001", "process_instance_id": "current"})
        self.assertIsNone(result["instances"][0]["businessTypeCode"])
        rows = list(c.record_rows(result["instances"][0]["response"]["data"]))
        self.assertEqual(rows[0]["businessTypeCode"], 2)
        self.assertEqual(rows[0]["businessId"], "historical-business")

    def test_definition_metadata_bound_to_current_instance_and_no_private_fields(self):
        context = {"processInstanceId": "newer", "formId": "form", "processDefinitionId": "definition",
            "submitterEmployee": {"employeeId": "a", "phone": "private"},
            "nodeConfigMap": {"n:1": {"dynamicApproverConfig": None, "body": "private"},
                              "n:2": {"dynamicApproverConfig": {"unknown_private_value": "private"}}}}
        fake = Fake(ok(records()), ok(context))
        result = c.Collector(fake, with_definition_metadata=True).collect_business(business(processInstanceId="historical"))
        instance = result["instances"][0]
        self.assertEqual(instance["processInstanceId"], "historical")
        self.assertEqual(instance["definition_metadata"]["processInstanceId"], "newer")
        self.assertEqual(instance["definition_metadata"]["formId"], "form")
        self.assertEqual(instance["definition_metadata"]["nodeConfigMap"]["n:2"]["dynamicApproverConfig"], {"configured": True})
        self.assertNotIn("private", json.dumps(result))

    def test_metadata_failure_does_not_drop_collected_tasks(self):
        fake = Fake(ok(records()), {"success": False, "code": 110004})
        result = c.Collector(fake, with_definition_metadata=True).collect_business(business(processInstanceId="p"))
        self.assertEqual(len(result["instances"]), 1)
        self.assertEqual(result["errors"][0]["route"], "definition_metadata")
        self.assertEqual(result["status"], "partial")

    def test_unsupported_type_is_not_guessed_as_normal_contract(self):
        fake = Fake()
        result = c.Collector(fake).collect_business(business(businessTypeCode=99))
        self.assertEqual(result["errors"][0]["error"], "unsupported_business_type")
        self.assertEqual(fake.calls, [])

    def test_verified_type_16_uses_contract_info_and_retains_its_type(self):
        fake = Fake(ok({"businessTypeCode": 16, "approveRecord": records()}))
        result = c.Collector(fake).collect_business(business(businessTypeCode=16))
        self.assertEqual(fake.calls[0][0], "contract/info")
        self.assertEqual(result["instances"][0]["businessTypeCode"], 16)

    def test_partial_discovery_is_collected_and_gaps_preserved(self):
        args = c.parse_args(["--discovery", "/tmp/unused", "--output-dir", "/tmp/unused",
                            "--start", "2026-09-01T00:00:00+08:00", "--end", "2026-10-01T00:00:00+08:00"])
        discovery = {"status": "partial", "scope": {"kind": "employee"},
            "employees": [{"employeeId": "a", "name": "甲"}, {"employeeId": "b", "name": "乙"}],
            "approver_ids": ["a", "b"], "businesses": [business(), business()],
            "coverage": {"contract_search_complete": False}, "diagnostics": [{"reason": "page_failed"}]}
        with contextlib.redirect_stdout(io.StringIO()):
            bundle, outcomes = c.Collector(Fake(ok({"approveRecord": records()}))).run(discovery, args)
        self.assertEqual(bundle["meta"]["analysis_subject"], "people")
        self.assertEqual(bundle["meta"]["collection_status"], "partial")
        self.assertEqual(bundle["meta"]["discovery_diagnostics"], discovery["diagnostics"])
        self.assertEqual(len(bundle["employees"]), 2)
        self.assertEqual(len(bundle["businesses"]), 1)
        self.assertEqual(len(outcomes), 1)
        self.assertEqual(bundle["meta"]["start"], args.start)
        self.assertEqual(bundle["node_participant_evidence"], [])

    def test_external_evidence_must_have_source_and_conflicts_fail(self):
        evidence = {"processInstanceId": "p", "processDefinitionId": "d", "nodeId": "n",
                    "employee_ids": ["a"], "complete": True, "source": "verified raw endpoint"}
        self.assertEqual(c.normalize_evidence([evidence, evidence]), [evidence])
        with self.assertRaisesRegex(ValueError, "requires_ids_and_source"):
            c.normalize_evidence([{**evidence, "source": ""}])
        with self.assertRaisesRegex(ValueError, "conflicting"):
            c.normalize_evidence([evidence, {**evidence, "employee_ids": ["b"]}])

    def test_float_ids_are_rejected_before_api_calls(self):
        fake = Fake()
        with self.assertRaises(ValueError):
            c.Collector(fake).collect_business(business(businessId=9e18))
        self.assertEqual(fake.calls, [])

    def test_cli_transport_retains_business_failure_despite_exit_zero(self):
        completed = type("Completed", (), {"returncode": 0, "stdout": json.dumps({"success": False, "code": 110004})})()
        with patch.object(c.subprocess, "run", return_value=completed) as run:
            reply = c.cli_caller("contract-cli", "contract-test")("contract/info", {"contractId": "id"})
        self.assertFalse(c.successful(reply))
        command = run.call_args.args[0]
        self.assertIn("Timezone-Offset: -480", command)
        self.assertEqual(command[command.index("--as") + 1], "user")
        self.assertIn("GET", command)

    def test_cli_malformed_json_read_retries_bounded_and_preserves_safe_diagnostics(self):
        truncated = type("Completed", (), {"returncode": 0, "stdout": '{"success":true,"data":'})()
        successful = type("Completed", (), {"returncode": 0, "stdout": json.dumps(ok({"done": [], "todo": []}))})()
        with patch.object(c.subprocess, "run", side_effect=[truncated, successful]) as run, patch.object(c.time, "sleep"):
            result = c.cli_caller("contract-cli", "contract-test")("contract/info", {})
            self.assertTrue(c.successful(result))
            self.assertEqual(run.call_count, 2)
        with patch.object(c.subprocess, "run", return_value=truncated) as run, patch.object(c.time, "sleep"):
            result = c.cli_caller("contract-cli", "contract-test")("contract/info", {})
            self.assertEqual(run.call_count, 3)
            self.assertEqual(result["collection_error"], "invalid_json")
            error = c.Collector.failure(business(), "definition_metadata", result)
            self.assertEqual(error["stdout_bytes"], len(truncated.stdout))
            self.assertEqual(error["json_error_position"], len(truncated.stdout))
            self.assertNotIn("stdout", error)


if __name__ == "__main__":
    unittest.main()
