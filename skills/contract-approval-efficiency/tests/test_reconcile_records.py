import importlib.util
from pathlib import Path
import unittest


spec = importlib.util.spec_from_file_location("reconciliation", Path(__file__).parents[1] / "scripts/reconcile_records.py")
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)


class ReconcileTests(unittest.TestCase):
    def setup_data(self):
        meta = {"start": "2026-09-01T00:00:00+08:00", "end": "2026-10-01T00:00:00+08:00", "timezone": "Asia/Shanghai"}
        created, finished = "2026-09-23 16:52:55", "2026-09-23 17:39:24"
        source = {"process_instance_id": "p", "task_instance_id": "t", "contract_id": "b", "task_status": 1,
                  "create_time": r.milliseconds(created, r.ZoneInfo(meta["timezone"])),
                  "end_time": r.milliseconds(finished, r.ZoneInfo(meta["timezone"])),
                  "candidate_in_window_or_pending": True}
        record = {"processInstanceId": "p", "processTaskInstanceId": "t", "businessId": "b", "businessTypeCode": 0,
                  "createTime": created, "endTime": finished, "commandTypeCode": 1, "taskStatusCode": 1,
                  "assigneeEmployees": [{"employeeId": "a"}]}
        bundle = {"meta": meta, "instances": [{"response": {"data": {"done": [{"nodeTypeCode": 1, "records": [record]}]}}}]}
        return {"approver_ids": ["a"], "task_records": [source]}, bundle

    def test_recovered_known_task_time_and_id_reconciles(self):
        discovery, bundle = self.setup_data()
        result = r.reconcile(discovery, bundle, {"meta": bundle["meta"], "instances": []})
        self.assertTrue(result["all_source_tasks_reconciled"])
        self.assertEqual(result["timestamp_mismatches"], [])
        self.assertEqual(result["newly_recovered_completed_count"], 1)
        self.assertEqual(result["newly_recovered_completed"][0]["duration_seconds"], 2789)

    def test_missing_source_task_has_failure_reason_instead_of_zero(self):
        discovery, bundle = self.setup_data()
        bundle["instances"] = []
        bundle["collection_errors"] = [{"processInstanceId": "p", "error": "business_failure", "code": 110004}]
        result = r.reconcile(discovery, bundle)
        self.assertFalse(result["all_source_tasks_reconciled"])
        self.assertEqual(result["missing_task_count"], 1)
        self.assertEqual(result["missing_tasks"][0]["reason"], "collection_failed")

    def test_submission_node_exists_but_not_approval_candidate(self):
        discovery, bundle = self.setup_data()
        bundle["instances"][0]["response"]["data"]["done"][0]["nodeTypeCode"] = 0
        result = r.reconcile(discovery, bundle)
        self.assertTrue(result["all_source_tasks_reconciled"])
        self.assertEqual(result["matched_node_type_counts"], {"0": 1})
        self.assertEqual(result["new_completed_candidate_count"], 0)

    def test_window_misclassification_is_detected_independently(self):
        discovery, bundle = self.setup_data()
        discovery["task_records"][0]["candidate_in_window_or_pending"] = False
        result = r.reconcile(discovery, bundle)
        self.assertFalse(result["all_source_tasks_reconciled"])
        self.assertEqual(len(result["source_window_disagreements"]), 1)
        self.assertEqual(result["matched_task_count"], 1)

    def test_later_completion_not_reported_as_old_report_omission(self):
        discovery, bundle = self.setup_data()
        old = {"meta": {**bundle["meta"], "end": "2026-09-23T17:00:00+08:00"}, "instances": []}
        result = r.reconcile(discovery, bundle, old)
        self.assertEqual(result["new_completed_candidate_count"], 1)
        self.assertEqual(result["comparable_new_completed_count"], 0)
        self.assertEqual(result["newly_recovered_completed_count"], 0)


if __name__ == "__main__":
    unittest.main()
