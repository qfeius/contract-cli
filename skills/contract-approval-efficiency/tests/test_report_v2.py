import copy
import unittest
from test_analyze_records import MODULE as m, task, bundle


def person(report, eid='e1', kind=1):
    return next(r for r in report['by_approver'] if r['employee_id'] == eid and r['node_type_code'] == kind)


class ReportV2Tests(unittest.TestCase):
    def test_named_people_render_only_personal_efficiency_without_numbered_sections(self):
        data = bundle(task())
        data['meta']['analysis_subject'] = 'people'
        report = m.analyze(data)
        text = m.render_markdown(report)
        self.assertEqual(report['meta']['analysis_subject'], 'people')
        self.assertIn('# 个人效率统计', text)
        self.assertEqual(text.count('个人效率统计'), 1)
        self.assertEqual(text.count('| 姓名 |'), 1)
        self.assertIn('| 姓名 | 节点类型 |', text)
        for hidden in ('审批效率总览', '效率洞察', '第一部分', '第二部分', '第三部分'):
            self.assertNotIn(hidden, text)

    def test_analysis_subject_override_supports_multiple_named_people(self):
        data = bundle(task())
        data['employees'] = [{'employeeId': 'e1', 'name': '张三'}, {'employeeId': 'e2', 'name': '李四'}]
        report = m.analyze(data, analysis_subject='people')
        text = m.render_markdown(report)
        self.assertEqual(text.count('张三'), 3)
        self.assertEqual(text.count('李四'), 3)
        self.assertNotIn('审批效率总览', text)

    def test_task_center_attribution_includes_actual_signer_on_multi_person_node(self):
        first = task('actual')
        other = task('other', assigneeEmployees=[{'employeeId':'e2','name':'其他人'}])
        data = bundle(first, other)
        data['employees'] = [{'employeeId': 'e1', 'name': '张三'}]
        data['task_records'] = [{
            'process_instance_id': 'instance1', 'task_instance_id': 'actual',
            'discovered_for_employee_ids': ['e1'], 'task_status': 1,
        }]
        result = m.analyze(data, approver_ids=['e1'], analysis_subject='people')
        row = person(result)
        self.assertEqual(row['approved_count'], 1)
        detail = next(d for d in result['details'] if d['task_instance_id'] == 'actual')
        self.assertTrue(detail['included_personal'])
        self.assertTrue(detail['multi_person_node'])
        self.assertEqual(detail['personal_employee_ids'], ['e1'])
        self.assertEqual(result['personal_exclusions'][0]['attributed_multi_task_count'], 1)

    def test_invalid_analysis_subject_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'analysis_subject'):
            m.analyze(bundle(task()), analysis_subject='person')

    def test_unique_contracts_are_not_handled_task_counts(self):
        data = bundle(task('a'), task('b', commandTypeCode=2, taskStatusCode=2))
        result = m.analyze(data)
        row = result['overview'][0]
        self.assertEqual((row['processed_contract_count'], row['approved_count'], row['rejected_count']), (1, 1, 1))
        self.assertEqual(row['total_seconds'], 7200)
        self.assertEqual(row['average_seconds'], 3600)
        self.assertEqual(person(result)['processed_contract_count'], 1)
        self.assertEqual(result['meta']['period_contract_count'], 1)

    def test_people_period_contract_count_uses_only_personally_attributed_completed_tasks(self):
        other = task('other', assigneeEmployees=[{'employeeId': 'e2', 'name': '其他人'}])
        data = bundle(task('mine'), other)
        data['employees'] = [{'employeeId': 'e1', 'name': '张三'}]
        data['task_records'] = [{'process_instance_id': 'instance1', 'task_instance_id': 'mine',
                                 'discovered_for_employee_ids': ['e1'], 'task_status': 1}]
        result = m.analyze(data, approver_ids=['e1'], analysis_subject='people')
        self.assertEqual(result['meta']['period_contract_count'], 1)
        self.assertIn('统计周期内涉及合同数：1份', m.render_markdown(result))

    def test_pending_contract_count_differs_from_person_pending_tasks(self):
        result = m.analyze(bundle(task('a', taskStatusCode=0, endTime=None), task('b', taskStatusCode=0, endTime=None)))
        self.assertEqual(result['overview'][0]['in_progress_contract_count'], 1)
        self.assertEqual(person(result)['in_progress_count'], 2)
        self.assertEqual(person(result)['average_seconds'], 0)
        self.assertIsNone(result['details'][0]['duration_seconds'])

    def test_every_roster_person_has_three_rows_including_no_records(self):
        data = bundle(task())
        data['employees'] = [{'employeeId': 'e1', 'name': '张三'}, {'employeeId': 'e2', 'name': '合成零记录人员'}]
        result = m.analyze(data)
        self.assertEqual(len(result['by_approver']), 6)
        for kind in (1,2,3):
            row = person(result, 'e2', kind)
            for field in ('processed_contract_count','approved_count','rejected_count','in_progress_count','total_seconds','average_seconds','max_seconds','min_seconds'):
                self.assertEqual(row[field], 0)
        text = m.render_markdown(result)
        self.assertEqual(text.count('| 姓名 |'), 3)
        self.assertNotIn('人员ID', text)
        self.assertEqual(text.count('合成零记录人员'), 3)
        self.assertLess(text.index('统计周期'),text.index('第一部分'))
        self.assertLess(text.index('效率洞察'),text.index('第二部分'))
        self.assertLess(text.index('第二部分'),text.index('第三部分'))

    def test_each_record_single_assignee_but_entire_node_multiple_is_excluded(self):
        other = task('b', assigneeEmployees=[{'employeeId':'e2','name':'其他人'}])
        data = bundle(task('a'),other)
        data['employees']=[{'employeeId':'e1','name':'张三'}]
        result=m.analyze(data,approver_ids=['e1'])
        self.assertEqual(result['overview'][0]['approved_count'],1)
        self.assertEqual(person(result)['approved_count'],0)
        self.assertEqual(result['personal_exclusions'][0]['task_count'],1)
        self.assertTrue(result['details'][0]['included_overview'])
        self.assertFalse(result['details'][0]['included_personal'])

    def test_other_person_outside_time_window_still_proves_multi_person_node(self):
        other=task('b',endTime='2026-08-31 10:00:00',createTime='2026-08-31 09:00:00',assigneeEmployees=[{'employeeId':'e2'}])
        result=m.analyze(bundle(task('a'),other),approver_ids=['e1'])
        self.assertEqual(person(result)['completed_count'],0)
        self.assertEqual(result['overview'][0]['approved_count'],1)

    def test_no_cross_instance_multi_person_leak(self):
        data=bundle(task())
        second=bundle(task('b',processInstanceId='other-instance',assigneeEmployees=[{'employeeId':'e2'}]))
        second['instances'][0]['processInstanceId']='other-instance'
        data['instances']+=second['instances']
        data['node_participant_evidence']+=second['node_participant_evidence']
        result=m.analyze(data)
        self.assertEqual(person(result)['approved_count'],1)
        self.assertEqual(person(result,'e2')['approved_count'],1)

    def test_mode_does_not_prove_single_or_multiple(self):
        for mode in (0,1,2,3,4):
            data=bundle(task());data['instances'][0]['response']['data']['done'][0]['taskModeCode']=mode
            data.pop('node_participant_evidence')
            result=m.analyze(data)
            self.assertEqual(person(result)['approved_count'],0)
            self.assertEqual(result['personal_exclusions'][0]['unverified_task_count'],1)
            self.assertEqual(result['overview'][0]['approved_count'],1)

    def test_proven_multi_person_even_if_losing_or_sign_records_hidden(self):
        data=bundle(task())
        data['node_participant_evidence'][0]['employee_ids']=['e1','e2']
        result=m.analyze(data)
        self.assertEqual(person(result)['approved_count'],0)
        self.assertTrue(result['details'][0]['multi_person_node'])

    def test_incomplete_person_evidence_never_assumed_single(self):
        data=bundle(task());data['node_participant_evidence'][0]['complete']=False
        self.assertEqual(person(m.analyze(data))['approved_count'],0)
        data['node_participant_evidence'][0]['complete']=True
        data['node_participant_evidence'][0]['source']=''
        with self.assertRaisesRegex(ValueError,'核验来源'):m.analyze(data)

    def test_historical_business_identity_and_names_resolve_from_registry(self):
        data=bundle(task(processInstanceId='old-instance',businessId='9000000000000000009',businessTypeCode=2))
        data['businesses']=[{'businessId':'9000000000000000009','businessTypeCode':2,'contractName':'历史合同','contractNumber':'OLD-1'}]
        detail=m.analyze(data)['details'][0]
        self.assertEqual((detail['contract_id'],detail['contract_name'],detail['contract_number']),('9000000000000000009','历史合同','OLD-1'))

    def test_missing_historical_business_is_not_replaced_by_current_contract(self):
        data=bundle(task(processInstanceId='old-instance'))
        result=m.analyze(data)
        self.assertEqual(result['details'][0]['contract_id'],'')
        self.assertEqual(result['overview'][0]['processed_contract_count'],0)
        self.assertEqual(result['overview'][0]['approved_count'],1)
        self.assertIn('合同归属缺失',result['details'][0]['exclusion_reason'])

    def test_automatic_and_invalid_time_retained_but_not_aggregated(self):
        data=bundle(task('good'),task('skip',commandTypeCode=14),task('bad',createTime=None))
        result=m.analyze(data)
        self.assertEqual(len(result['details']),3)
        self.assertEqual(result['overview'][0]['approved_count'],1)
        self.assertEqual(sum(d['included_overview'] for d in result['details']),1)
        self.assertIsNone(next(d for d in result['details'] if d['task_instance_id']=='bad')['duration_seconds'])

    def test_missing_task_id_is_retained_in_detail_without_fabricated_id(self):
        result=m.analyze(bundle(task(processTaskInstanceId=None)))
        self.assertEqual(result['details'][0]['task_instance_id'],'')
        self.assertFalse(result['details'][0]['included_overview'])
        self.assertEqual(result['overview'][0]['approved_count'],0)

    def test_duplicate_task_across_contracts_counts_once_in_overview_and_export(self):
        data=bundle(task())
        data['instances'].append(copy.deepcopy(data['instances'][0]))
        result=m.analyze(data)
        self.assertEqual(len(result['details']),1)
        self.assertEqual(result['overview'][0]['approved_count'],1)

    def test_cancelled_status_never_displays_passed_command(self):
        result=m.analyze(bundle(task(taskStatusCode=3,commandTypeCode=1)))
        self.assertEqual(result['details'][0]['result'],'已撤回')
        self.assertFalse(result['details'][0]['included_overview'])
        self.assertEqual(m.operation_label(task(commandTypeCode=3)),'转办')
        self.assertEqual(m.operation_label(task(taskStatusCode=5)),'自动终止')

    def test_department_filter_excludes_cross_scope_ambiguous_completed_task(self):
        data=bundle(task(assigneeEmployees=[{'employeeId':'e1','departments':[{'departmentId':'d1'}]}, {'employeeId':'e2','departments':[{'departmentId':'d2'}]}]))
        data['employees']=[{'employeeId':'e1','departments':[{'departmentId':'d1'}]}, {'employeeId':'e2','departments':[{'departmentId':'d2'}]}]
        data['approver_ids']=['e1','e2']
        result=m.analyze(data,department_ids=['d1'])
        self.assertEqual(result['overview'][0]['approved_count'],0)
        self.assertFalse(result['details'][0]['included_overview'])

    def test_unknown_candidate_id_not_assumed_within_target_scope(self):
        data=bundle(task(assigneeEmployees=[{'employeeId':'e1'},{}]))
        result=m.analyze(data,approver_ids=['e1'])
        self.assertEqual(result['overview'][0]['approved_count'],0)
        self.assertFalse(result['details'][0]['included_overview'])

    def test_conflict_detail_keeps_in_window_variant_independent_of_input_order(self):
        old=task(endTime='2025-09-01 10:00:00',createTime='2025-09-01 09:00:00')
        current=task()
        outputs=[]
        for records in ((old,current),(current,old)):
            result=m.analyze(bundle(*records))
            self.assertEqual(len(result['details']),1)
            self.assertFalse(result['details'][0]['included_overview'])
            self.assertIn('2026-09-01',result['details'][0]['ended_at'])
            outputs.append(result['details'])
        self.assertEqual(*outputs)

    def test_zero_roster_never_expands_to_everyone_in_candidate_contracts(self):
        data=bundle(task());data['employees']=[]
        result=m.analyze(data)
        self.assertEqual(result['by_approver'],[])
        self.assertEqual(result['details'],[])


if __name__=='__main__':unittest.main()
