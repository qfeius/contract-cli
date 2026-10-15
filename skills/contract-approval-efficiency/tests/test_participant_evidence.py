import copy
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
from collect_participant_evidence import (enrich_bundle, minimize_participants,
                                         minimize_process, prove_node, index_bundle,
                                         PARTICIPANT_PATH, PROCESS_PATH, FORM_PATH)
from analyze_records import analyze


def record(task='approve', node='approval', command=1, employee='a'):
    return {'processInstanceId': 'p', 'processDefinitionId': 'd', 'nodeId': node,
            'processTaskInstanceId': task, 'businessId': '100', 'businessTypeCode': 0,
            'assigneeEmployees': [{'employeeId': employee}], 'commandTypeCode': command,
            'taskStatusCode': 1, 'createTime': '2026-09-01 12:00:00', 'endTime': '2026-09-01 13:00:00'}


def bundle():
    return {'meta': {'start': '2026-09-01', 'end': '2026-10-01', 'timezone': 'Asia/Shanghai'},
            'approver_ids': ['a'], 'instances': [{
                'processInstanceId': 'p', 'businessId': '100', 'businessTypeCode': 0,
                'definition_metadata': {'processInstanceId': 'p', 'formId': 'f',
                    'nodeConfigMap': {'approval': {'dynamicApproverConfig': None}}},
                'response': {'code': 200, 'success': True, 'data': {'done': [
                    {'nodeTypeCode': 0, 'nodeId': 'submit', 'records': [record('submit', 'submit', 0)]},
                    {'nodeTypeCode': 1, 'nodeId': 'approval', 'records': [record()]}], 'todo': []}}}]}


def proof_inputs():
    source = bundle()
    groups, _, bindings = index_bundle(source)
    participants = {'employees': [{'employeeId': 'a', 'larkId': 'la', 'node_ids': ['approval', 'submit']}]}
    process = {'processInstanceId': 'p', 'processDefinitionId': 'd',
               'tasks': [{'node_id': 'approval', 'task_instance_id': 'approve', 'assignee_ids': ['la'], 'command_type': 'general'}]}
    forms = {'f': {'formId': 'f', 'published': True, 'processDefinitionId': 'd', 'nodes': [
        {'node_id': 'approval', 'personnel_type_code': 4, 'approvers_bo': 'contract_submitter', 'employee_ids': []}]}}
    return ('p', 'd', 'approval'), groups[('p', 'd', 'approval')], participants, process, forms, bindings


class ParticipantEvidenceTests(unittest.TestCase):
    def test_exact_historical_fixed_singleton_can_be_counted(self):
        item, reason = prove_node(*proof_inputs())
        self.assertTrue(item['complete'])
        self.assertEqual(item['employee_ids'], ['a'])
        self.assertEqual(reason, 'proven_fixed_singleton')

    def test_finished_or_single_visible_operator_is_not_singleton_proof(self):
        args = list(proof_inputs())
        args[4]['f']['nodes'][0].update(personnel_type_code=7, approvers_bo='approval_matrix')
        item, reason = prove_node(*args)
        self.assertFalse(item['complete'])
        self.assertEqual(reason, 'fixed_singleton_or_override_not_proven')

    def test_configured_multiple_candidates_are_not_claimed_as_observed_people(self):
        args = list(proof_inputs())
        args[4]['f']['nodes'][0].update(personnel_type_code=0, employee_ids=['a', 'b'], approvers_bo='user:a,b')
        item, reason = prove_node(*args)
        self.assertFalse(item['complete'])
        self.assertEqual(item['employee_ids'], ['a'])
        self.assertEqual(item['configured_employee_ids'], ['a', 'b'])
        self.assertEqual(reason, 'configured_multiple_runtime_unverified')

        source = bundle()
        source['meta']['timezone_confirmed'] = True
        source['meta']['timezone_source'] = 'synthetic Asia/Shanghai fixture'
        source['node_participant_evidence'] = [item]
        report = analyze(source)
        approval = next(row for row in report['personal_exclusions'] if row['node_type_code'] == 1)
        self.assertEqual(approval['node_count'], 0)
        self.assertEqual(approval['unverified_node_count'], 1)
        self.assertFalse(report['details'][0]['multi_person_node'])
        self.assertFalse(report['details'][0]['included_personal'])
        self.assertIn('历史配置含多人，实际分配人数尚未核实', report['details'][0]['exclusion_reason'])
        self.assertEqual(report['overview'][0]['completed_count'], 1)
        self.assertEqual(sum(row['completed_count'] for row in report['by_approver']), 0)

    def test_fixed_person_display_config_does_not_override_actual_expression(self):
        args = list(proof_inputs())
        args[4]['f']['nodes'][0].update(personnel_type_code=0, employee_ids=['a'], approvers_bo='some_dynamic_expression')
        self.assertFalse(prove_node(*args)[0]['complete'])

    def test_current_definition_never_applied_to_historical_task(self):
        args = list(proof_inputs())
        args[4]['f']['processDefinitionId'] = 'new-definition'
        self.assertFalse(prove_node(*args)[0]['complete'])

    def test_hidden_agent_task_prevents_false_singleton(self):
        args = list(proof_inputs())
        args[3]['tasks'].append({'node_id': 'approval', 'task_instance_id': 'hidden', 'assignee_ids': ['la'], 'command_type': 'doAgent'})
        item, reason = prove_node(*args)
        self.assertFalse(item['complete'])
        self.assertEqual(reason, 'raw_reassignment_or_unknown_command')

    def test_parent_command_without_parent_id_still_prevents_singleton(self):
        args = list(proof_inputs())
        args[1]['records'][0]['parentCommandTypeCode'] = 23
        self.assertEqual(prove_node(*args)[1], 'reassignment_or_endorsement_requires_full_history')

    def test_endorsement_approval_requires_reassignment_history(self):
        args = list(proof_inputs())
        args[1]['records'][0]['commandTypeCode'] = 10
        self.assertEqual(prove_node(*args)[1], 'reassignment_or_endorsement_requires_full_history')

    def test_unmapped_openapi_lark_id_is_not_an_employee_id(self):
        args = list(proof_inputs())
        args[3]['tasks'][0]['assignee_ids'] = ['other-id']
        self.assertEqual(prove_node(*args)[1], 'raw_process_assignee_mapping_incomplete')

    def test_missing_raw_task_fails_closed(self):
        args = list(proof_inputs())
        args[3]['tasks'] = []
        self.assertEqual(prove_node(*args)[1], 'raw_process_missing_visible_tasks')

    def test_conflicting_task_copy_on_another_node_cannot_hide_mutation(self):
        args = list(proof_inputs())
        args[3]['tasks'].append({'node_id': 'another', 'task_instance_id': 'approve', 'assignee_ids': ['la'], 'command_type': 'transfer'})
        item, reason = prove_node(*args)
        self.assertFalse(item['complete'])
        self.assertEqual(reason, 'conflicting_raw_process_task')

    def test_identical_raw_task_copies_are_deduplicated(self):
        args = list(proof_inputs())
        args[3]['tasks'].append(copy.deepcopy(args[3]['tasks'][0]))
        self.assertTrue(prove_node(*args)[0]['complete'])

    def test_other_node_null_assignee_does_not_invalidate_proven_node(self):
        args = list(proof_inputs())
        args[3]['tasks'].append({'node_id': 'another', 'task_instance_id': 'other-task', 'assignee_ids': [None], 'command_type': 'general'})
        self.assertTrue(prove_node(*args)[0]['complete'])

    def test_target_node_null_assignee_remains_unverified(self):
        args = list(proof_inputs())
        args[3]['tasks'][0]['assignee_ids'] = [None]
        item, reason = prove_node(*args)
        self.assertFalse(item['complete'])
        self.assertEqual(reason, 'invalid_raw_process_assignee')

    def test_null_mapping_still_participates_in_whole_instance_task_conflicts(self):
        args = list(proof_inputs())
        args[3]['tasks'].append({'node_id': 'another', 'task_instance_id': 'other-task', 'assignee_ids': [None], 'command_type': 'general'})
        args[3]['tasks'].append({'node_id': 'another', 'task_instance_id': 'other-task', 'assignee_ids': ['la'], 'command_type': 'general'})
        self.assertEqual(prove_node(*args)[1], 'conflicting_raw_process_task')

    def test_dynamic_override_not_treated_as_null(self):
        args = list(proof_inputs())
        args[5]['p'][0]['nodeConfigMap']['approval']['dynamicApproverConfig'] = {'configured': True}
        self.assertFalse(prove_node(*args)[0]['complete'])

    def test_index_does_not_bind_current_metadata_to_historical_instance(self):
        data = bundle()
        data['instances'][0]['definition_metadata']['processInstanceId'] = 'current-p'
        self.assertEqual(dict(index_bundle(data)[2]), {})

    def test_all_wrappers_and_nodes_share_one_request_per_instance(self):
        data = bundle()
        data['instances'].append(copy.deepcopy(data['instances'][0]))
        calls = []
        def call(method, path, query):
            calls.append((method, path))
            if path == PARTICIPANT_PATH:
                return {'success': True, 'code': 200, 'data': [{'employeeId': 'a', 'larkId': 'la', 'nodeId2NameMap': {'approval': '审批', 'submit': '提交'}}]}
            if path == PROCESS_PATH + 'p':
                return {'success': True, 'code': 0, 'data': {'process_instance': {'process_instance_id': 'p', 'process_definition_id': 'd', 'task_instance_list': [{'task_instance_id': 'approve', 'node_id': 'approval', 'assignee_ids': ['la'], 'command_type': 'general'}]}}}
            if path == FORM_PATH:
                return {'success': True, 'code': 200, 'data': {'formId': 'f', 'published': True, 'processDefinitionId': 'd', 'processDefinitionJson': {'node_models': [{'node_id': 'approval', 'approvers_bo': 'contract_submitter', 'component_props': {'clm_form': {'approver': {'personnel_type_code': 4}}}}]}}}
            raise AssertionError(path)
        result = enrich_bundle(data, call, workers=2)
        self.assertEqual(len(calls), 3)
        self.assertEqual(len(result['node_participant_evidence']), 1)
        self.assertTrue(result['node_participant_evidence'][0]['complete'])

    def test_minimal_response_drops_pii_and_preserves_all_node_ids(self):
        result = minimize_participants({'success': True, 'code': 200, 'data': [{'employeeId': 'a', 'larkId': 'la', 'email': 'private', 'nodeId': 'first', 'nodeId2NameMap': {'first': '一', 'second': '二'}}]})
        self.assertEqual(result['employees'][0]['node_ids'], ['first', 'second'])
        self.assertNotIn('email', result['employees'][0])


if __name__ == '__main__':
    unittest.main()
