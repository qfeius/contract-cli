import copy
import contextlib
import io
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('discovery', Path(__file__).parents[1] / 'scripts/discover_businesses.py')
d = importlib.util.module_from_spec(spec)
spec.loader.exec_module(d)


def ok(data):
    return {'success': True, 'code': 200, 'data': data}


def emp(eid='9000000000000000001', name='合成人员', deps=None):
    return {'employeeId': eid, 'name': name, 'departments': deps or [], 'userId': 'must-not-use', 'phone': 'must-not-save'}


def dep(did, name='合成部门', children=None):
    return {'departmentId': did, 'name': name, 'childDepartments': children or []}


def group(gid, *ids):
    return {'groupId': gid, 'searchContractListItems': [
        {'contractId': bid, 'businessTypeCode': 0, 'contractName': '合成合同'} for bid in ids]}


def page(groups, total, cursor=None):
    return ok({'content': groups, 'filteredTotalElements': total, 'searchAfter': cursor,
               'totalElements': [{'searchTabCode': 0, 'count': 987654}]})


def args(*extra):
    return d.parse_args(['--employee-name', '合成人员', '--candidate-source', 'visible-contracts',
                         '--output', '/tmp/unused-discovery.json', *extra])


def task(tid, bid='9000000000000000001', pid='process-1', create=100, end=200, cursor=1, status=1):
    return {'task_instance_id': tid, 'contract_id': bid, 'process_instance_id': pid,
            'create_time': create, 'end_time': end, 'cursor': cursor, 'task_status': status,
            'assignee_ids': [777]}


def task_page(*tasks):
    return {'success': True, 'code': 0, 'data': {'task_list': list(tasks)}}


class Fake:
    def __init__(self, responses):
        self.responses = list(responses)
        self.calls = []

    def __call__(self, method, path, query=None, body=None):
        self.calls.append((method, path, copy.deepcopy(query), copy.deepcopy(body)))
        if not self.responses:
            raise AssertionError('unexpected request')
        response = self.responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return copy.deepcopy(response)


class DiscoveryTests(unittest.TestCase):
    def test_default_is_raw_tasks(self):
        parsed = d.parse_args(['--employee-name', '甲', '--output', '/tmp/unused'])
        self.assertEqual(parsed.candidate_source, 'raw-tasks')

    def test_raw_tasks_resolve_real_lark_id_and_both_cursor_time_fields(self):
        fake = Fake([ok([emp('a')]), ok([dict(emp('a'), larkId=777)]),
                     task_page(task('t1', create=10, end=20, cursor=101)),
                     task_page(task('t2', bid='old', create=1, end=15, cursor=100)), task_page(),
                     task_page(task('todo', bid='pending', create=5, end=None, cursor=50, status=0)), task_page()])
        result = d.Discovery(fake).run(args('--candidate-source', 'raw-tasks',
            '--checkpoint-contract-id', '9000000000000000001'))
        self.assertEqual(result['status'], 'complete')
        self.assertEqual(result['employees'][0]['larkId'], '777')
        self.assertEqual(fake.calls[1][2], {'employeeIds': 'a'})
        self.assertEqual(fake.calls[2][1], d.TASK_PATH)
        self.assertEqual(fake.calls[2][2]['assignee_user_id'], '777')
        self.assertNotIn('last_cursor', fake.calls[2][2])
        self.assertEqual(fake.calls[3][2]['last_sort_value'], 20)
        self.assertEqual(fake.calls[3][2]['last_cursor'], 101)
        self.assertEqual(fake.calls[6][2]['last_sort_value'], 5)
        self.assertEqual(fake.calls[6][2]['last_cursor'], 50)
        self.assertEqual(len(result['process_instances']), 3)
        self.assertNotIn('businessTypeCode', result['process_instances'][0])
        self.assertEqual(result['coverage']['checkpoints']['found'], ['9000000000000000001'])
        self.assertFalse(any('es/search' in call[1] for call in fake.calls))

    def test_raw_completed_window_retains_all_pending_and_scans_until_empty(self):
        fake = Fake([ok([dict(emp('a'), larkId=777)]),
                     task_page(task('outside-new', 'future', end=300)),
                     task_page(task('inside', 'in', end=150, cursor=2)),
                     task_page(task('outside-old', 'old', end=50, cursor=3)), task_page(),
                     task_page(task('todo', 'pending', create=1, end=None, cursor=4, status=0)), task_page()])
        result = d.Discovery(fake).run(args('--candidate-source', 'raw-tasks',
            '--start', '1970-01-01T00:00:00.100+00:00', '--end', '1970-01-01T00:00:00.200+00:00'))
        self.assertEqual(result['status'], 'complete')
        self.assertEqual({p['businessId'] for p in result['process_instances']}, {'in', 'pending'})
        self.assertEqual(len(result['task_records']), 4)
        self.assertEqual(len(fake.calls), 7)

    def test_raw_missing_identity_is_partial_never_assumes_employee_equals_lark(self):
        fake = Fake([ok([emp('a')]), ok([emp('a')])])
        result = d.Discovery(fake).run(args('--candidate-source', 'raw-tasks'))
        self.assertEqual(result['status'], 'partial')
        self.assertEqual(len(fake.calls), 2)
        self.assertEqual(result['process_instances'], [])

    def test_raw_repeated_page_and_missing_cursor_fail_closed_preserving_records(self):
        first = task_page(task('t1'))
        for response in (first, task_page(task('t2', cursor=None))):
            fake = Fake([ok([dict(emp('a'), larkId=777)]), first, response, task_page()])
            result = d.Discovery(fake).run(args('--candidate-source', 'raw-tasks'))
            self.assertEqual(result['status'], 'partial')
            self.assertIn('t1', {t['task_instance_id'] for t in result['task_records']})

    def test_checkpoint_does_not_inject_missing_contract_and_marks_gap(self):
        result = d.Discovery(Fake([ok([emp('a')]), page([], 0)])).run(
            args('--checkpoint-contract-id', 'missing'))
        self.assertEqual(result['status'], 'partial')
        self.assertEqual(result['businesses'], [])
        self.assertEqual(result['coverage']['checkpoints']['missing'], ['missing'])

    def test_department_visible_fallback_scans_once_without_person_filter(self):
        fake = Fake([ok([dep('r')]), ok({'employees': [emp('a'), emp('b')], 'hasMore': False}),
                     page([group('g', 'b1')], 1)])
        parsed = d.parse_args(['--department-name', '合成部门', '--candidate-source', 'visible-contracts', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(parsed)
        self.assertEqual(result['status'], 'complete')
        self.assertEqual(len(fake.calls), 3)
        self.assertEqual(fake.calls[2][3]['filterUnits'], [])
        self.assertEqual(result['businesses'][0]['matched_employee_ids'], [])

    def test_raw_resume_preserves_cursor_and_does_not_repeat_completed_status(self):
        first = d.Discovery(Fake([ok([dict(emp('a'), larkId=777)]),
            task_page(task('t1', cursor=99, end=777)), task_page()])).run(
                args('--candidate-source', 'raw-tasks', '--max-pages', '1'))
        self.assertEqual(first['status'], 'partial')
        later = Fake([task_page()])
        final = d.Discovery(later).run(args('--candidate-source', 'raw-tasks'), previous=first)
        self.assertEqual(final['status'], 'complete')
        self.assertEqual(later.calls[0][2]['last_cursor'], 99)
        self.assertEqual(later.calls[0][2]['last_sort_value'], 777)
        self.assertEqual(len(later.calls), 1)

    def test_resume_rejects_old_version_changed_source_window_or_page_size(self):
        first = d.Discovery(Fake([ok([emp('a')])])).run(args('--resolve-only'))
        old = copy.deepcopy(first)
        old['version'] = 1
        with self.assertRaisesRegex(d.DiscoveryError, 'incompatible_resume'):
            d.Discovery(Fake([])).run(args(), previous=old)
        for flags, reason in ((['--candidate-source', 'raw-tasks'], 'candidate_source_mismatch'),
                              (['--page-size', '200'], 'page_size_mismatch'),
                              (['--start', '2026-09-01'], 'time_window_mismatch')):
            with self.assertRaisesRegex(d.DiscoveryError, reason):
                d.Discovery(Fake([])).run(args(*flags), previous=first)

    def test_visible_metadata_preserves_actual_process_ids_only(self):
        row = group('g', 'one', 'two', 'three')
        row['searchContractListItems'][0].update(processInstanceId='direct', contractStatusCode=6,
            createTime=123, archivedTime=456, progressVO={'processInstanceId': 'progress'})
        row['searchContractListItems'][1]['progressVO'] = {'processInstanceId': 'fallback'}
        result = d.Discovery(Fake([ok([emp('a')]), page([row], 1)])).run(args('--page-size', '200'))
        self.assertEqual(result['businesses'][0]['processInstanceId'], 'direct')
        self.assertEqual(result['businesses'][1]['processInstanceId'], 'fallback')
        self.assertNotIn('processInstanceId', result['businesses'][2])
        self.assertEqual(result['businesses'][0]['archivedTime'], 456)

    def test_ambiguous_name_never_queries_contracts(self):
        fake = Fake([ok([emp('a'), emp('b')])])
        result = d.Discovery(fake).run(args())
        self.assertEqual(result['status'], 'needs_selection')
        self.assertEqual(len(fake.calls), 1)
        self.assertNotIn('phone', result['candidates'][0])
        self.assertNotIn('userId', result['candidates'][0])

    def test_unique_display_suffix_and_selected_real_id(self):
        for extra in ([], ['--selected-employee-id', 'a']):
            fake = Fake([ok([emp('a', '合成人员(Example)')]), page([], 0)])
            result = d.Discovery(fake).run(args(*extra))
            self.assertEqual(result['status'], 'complete')
            self.assertEqual(result['approver_ids'], ['a'])

    def test_selection_must_come_from_candidate_response(self):
        fake = Fake([ok([emp('a')])])
        with self.assertRaisesRegex(d.DiscoveryError, 'selected_id_not'):
            d.Discovery(fake).run(args('--selected-employee-id', 'invented'))
        self.assertEqual(len(fake.calls), 1)

    def test_not_found_not_complete(self):
        result = d.Discovery(Fake([ok([])])).run(args())
        self.assertEqual(result['status'], 'not_found')

    def test_reject_float_bool_and_missing_internal_id(self):
        for value in (1.5, True, None, ''):
            with self.assertRaises(d.DiscoveryError):
                d.identifier(value)
        self.assertEqual(d.identifier(9876543210123456789), '9876543210123456789')
        with self.assertRaises(d.DiscoveryError):
            d.employee({'userId': 'not-an-employee-id'})

    def test_department_empty_root_child_members_cycles_dedup(self):
        child = dep('c', '合成子组')
        root = dep('r', children=[child])
        fake = Fake([ok([root]), ok({'employees': [], 'hasMore': True, 'subDepartments': [child]}),
                     ok({'employees': [emp('a')], 'hasMore': False, 'subDepartments': [root]}), page([], 0)])
        a = d.parse_args(['--department-name', '合成部门', '--candidate-source', 'visible-contracts', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(a)
        self.assertEqual(result['status'], 'complete')
        self.assertEqual(result['approver_ids'], ['a'])
        self.assertEqual(result['coverage']['department_ids'], ['c', 'r'])
        self.assertEqual(len(fake.calls), 4)

    def test_direct_department_only(self):
        fake = Fake([ok([dep('r', children=[dep('c', '子组')])]),
                     ok({'employees': [], 'hasMore': False, 'subDepartments': [dep('c')]})])
        a = d.parse_args(['--department-name', '合成部门', '--candidate-source', 'visible-contracts', '--direct-department-only', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(a)
        self.assertEqual(result['approver_ids'], [])
        self.assertEqual(len(fake.calls), 2)

    def test_member_second_page_failure_preserves_roster_and_marks_partial(self):
        fake = Fake([ok([dep('r')]), ok({'employees': [emp('a')], 'hasMore': True}),
                     {'success': False, 'code': 110004}, page([], 0)])
        a = d.parse_args(['--department-name', '合成部门', '--candidate-source', 'visible-contracts', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(a)
        self.assertEqual(result['status'], 'partial')
        self.assertEqual(result['approver_ids'], ['a'])
        self.assertFalse(result['coverage']['roster_complete'])
        self.assertEqual(fake.calls[2][2]['employeePageIndex'], 2)

    def test_member_empty_terminal_with_has_more_bug(self):
        fake = Fake([ok([dep('r')]), ok({'employees': [emp('a')], 'hasMore': True}),
                     ok({'employees': [], 'hasMore': True})])
        a = d.parse_args(['--department-name', '合成部门', '--resolve-only', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(a)
        self.assertEqual(result['status'], 'resolved')
        self.assertEqual(len(fake.calls), 3)

    def test_resolve_only_keeps_explicit_contract_scope(self):
        fake = Fake([ok([emp()])])
        result = d.Discovery(fake).run(args('--resolve-only'))
        self.assertEqual(result['status'], 'resolved')
        self.assertEqual(result['businesses'], [])
        self.assertEqual(len(fake.calls), 1)

    def test_group_total_not_contract_total_and_cursor_precision(self):
        cursor = [1.0, 1780000000000, 0, 0, 9876543210123456789]
        fake = Fake([ok([emp('a')]), page([group('g1', 'b1', 'b2')], 2, cursor),
                     page([group('g2', 'b3')], 2)])
        result = d.Discovery(fake).run(args())
        self.assertEqual(result['status'], 'complete')
        self.assertEqual(len(result['businesses']), 3)
        body = fake.calls[2][3]
        self.assertEqual(body['searchAfter'], cursor)
        self.assertEqual(body['filterUnits'], [])
        self.assertEqual(body['conditionUnits'], [])
        self.assertEqual(set(body), {'searchTabCode', 'pageIndex', 'pageSize', 'withStatistics', 'enableHighlight',
                                     'viewTypeEnum', 'conditionUnits', 'filterUnits', 'searchAfter'})

    def test_default_and_zero_limits_fetch_over_200_businesses_and_20_pages(self):
        for extra in ([], ['--max-businesses', '0', '--max-pages', '0']):
            with self.subTest(extra=extra):
                responses = [ok([emp('a')])]
                for index in range(25):
                    rows = [group(f'g{n}', f'b{n}') for n in range(index * 10, (index + 1) * 10)]
                    responses.append(page(rows, 250, [9876543210123456789 + index]))
                fake = Fake(responses)
                result = d.Discovery(fake).run(args(*extra))
                self.assertEqual(result['status'], 'complete')
                self.assertEqual({b['businessId'] for b in result['businesses']}, {f'b{n}' for n in range(250)})
                coverage = result['coverage']['searches'][0]
                self.assertEqual(coverage['pages_read'], 25)
                self.assertEqual(coverage['businesses_found'], 250)
                self.assertTrue(coverage['complete'])
                self.assertEqual(len(fake.calls), 26)
                self.assertEqual(fake.calls[-1][3]['searchAfter'], [9876543210123456789 + 23])
                self.assertEqual(result['diagnostics'], [])

    def test_default_member_pagination_exceeds_20_pages(self):
        responses = [ok([dep('r')])]
        responses.extend(ok({'employees': [emp(f'e{i}')], 'hasMore': i < 24}) for i in range(25))
        fake = Fake(responses)
        a = d.parse_args(['--department-name', '合成部门', '--resolve-only', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(a)
        self.assertEqual(result['status'], 'resolved')
        self.assertEqual(result['approver_ids'], [f'e{i}' for i in range(25)])
        self.assertEqual([c[2]['employeePageIndex'] for c in fake.calls[1:]], list(range(1, 26)))
        self.assertTrue(result['coverage']['roster_complete'])

    def test_default_department_traversal_exceeds_100_departments(self):
        root = dep('r', children=[dep(f'd{i}', '子组') for i in range(100)])
        fake = Fake([ok([root])] + [ok({'employees': [], 'hasMore': False}) for _ in range(101)])
        a = d.parse_args(['--department-name', '合成部门', '--resolve-only', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(a)
        self.assertEqual(result['status'], 'resolved')
        self.assertEqual(len(result['coverage']['department_ids']), 101)
        self.assertEqual(len(fake.calls), 102)
        self.assertTrue(result['coverage']['roster_complete'])

    def test_explicit_roster_limits_preserve_partial_results(self):
        for flag, reason in (('--max-member-pages', 'member_page_limit'),
                             ('--max-departments', 'department_limit')):
            with self.subTest(flag=flag):
                root = dep('r', children=[dep('child', '子组')])
                fake = Fake([ok([root]), ok({'employees': [emp('a')], 'hasMore': flag == '--max-member-pages'})])
                a = d.parse_args(['--department-name', '合成部门', '--direct-department-only',
                                  '--resolve-only', flag, '1', '--output', '/tmp/unused'])
                if flag == '--max-departments':
                    a.direct_department_only = False
                result = d.Discovery(fake).run(a)
                self.assertEqual(result['status'], 'partial')
                self.assertEqual(result['approver_ids'], ['a'])
                self.assertFalse(result['coverage']['roster_complete'])
                self.assertEqual(result['diagnostics'][0]['reason'], reason)
                self.assertEqual(len(fake.calls), 2)

    def test_each_employee_independent_scalar_search_union(self):
        fake = Fake([ok([dep('r')]), ok({'employees': [emp('a'), emp('b')], 'hasMore': False}),
                     page([group('g1', 'b1', 'b2')], 1), page([group('g1', 'b1', 'b2')], 1)])
        a = d.parse_args(['--department-name', '合成部门', '--candidate-source', 'participant-index', '--output', '/tmp/unused'])
        result = d.Discovery(fake).run(a)
        self.assertEqual(result['status'], 'partial')
        self.assertEqual(len(result['businesses']), 2)
        self.assertEqual(result['businesses'][0]['matched_employee_ids'], ['a', 'b'])
        self.assertEqual([c[3]['filterUnits'][0]['searchValue'] for c in fake.calls[2:]], ['a', 'b'])

    def test_repeated_page_early_empty_changed_total_are_partial(self):
        first = page([group('g1', 'b1')], 2, [1234567890123456789])
        for second in (first, page([], 2), page([group('g2', 'b2')], 3)):
            fake = Fake([ok([emp('a')]), first, second])
            result = d.Discovery(fake).run(args())
            self.assertEqual(result['status'], 'partial')
            self.assertEqual(len(result['businesses']), 1)
            self.assertEqual(len(fake.calls), 3)

    def test_missing_cursor_retains_successful_page_not_complete(self):
        result = d.Discovery(Fake([ok([emp('a')]), page([group('g1', 'b1', 'b2')], 2)])).run(args())
        self.assertEqual(result['status'], 'partial')
        self.assertEqual(len(result['businesses']), 2)
        self.assertEqual(result['coverage']['searches'][0]['stop_reason'], 'missing_search_after_restart_required')

    def test_mid_page_limit_resume_no_loss_no_refetch(self):
        fake = Fake([ok([emp('a')]), page([group('g1', 'b1', 'b2', 'b3')], 1)])
        first = d.Discovery(fake).run(args('--max-businesses', '1'))
        self.assertEqual(first['status'], 'partial')
        self.assertEqual(len(first['businesses']), 1)
        self.assertEqual(first['coverage']['searches'][0]['pending_business_rows'], 2)
        no_calls = Fake([])
        final = d.Discovery(no_calls).run(args('--max-businesses', '3'), previous=first)
        self.assertEqual(final['status'], 'complete')
        self.assertEqual(len(final['businesses']), 3)
        self.assertEqual(len(no_calls.calls), 0)

    def test_page_limit_resume_uses_next_cursor(self):
        fake = Fake([ok([emp('a')]), page([group('g1', 'b1')], 2, [9876543210123456789])])
        first = d.Discovery(fake).run(args('--max-pages', '1'))
        later = Fake([page([group('g2', 'b2')], 2)])
        final = d.Discovery(later).run(args('--max-pages', '2'), previous=first)
        self.assertEqual(final['status'], 'complete')
        self.assertEqual(later.calls[0][3]['searchAfter'], [9876543210123456789])

    def test_default_resume_removes_explicit_limits_and_drains_pending_rows(self):
        first = d.Discovery(Fake([ok([emp('a')]), page([group('g1', 'b1', 'b2')], 2, [123])])).run(
            args('--max-businesses', '1', '--max-pages', '1'))
        self.assertEqual(first['status'], 'partial')
        later = Fake([page([group('g2', 'b3')], 2)])
        final = d.Discovery(later).run(args(), previous=first)
        self.assertEqual(final['status'], 'complete')
        self.assertEqual({b['businessId'] for b in final['businesses']}, {'b1', 'b2', 'b3'})
        self.assertEqual(final['coverage']['searches'][0]['pages_read'], 2)
        self.assertEqual(len(later.calls), 1)
        self.assertEqual(later.calls[0][3]['searchAfter'], [123])

    def test_missing_cursor_pending_rows_can_be_drained_on_resume(self):
        first = d.Discovery(Fake([ok([emp('a')]), page([group('g1', 'b1', 'b2')], 2)])).run(args('--max-businesses', '1'))
        final = d.Discovery(Fake([])).run(args('--max-businesses', '2'), previous=first)
        self.assertEqual(final['status'], 'partial')
        self.assertEqual(len(final['businesses']), 2)

    def test_business_failure_exit_zero_is_not_empty(self):
        fake = Fake([ok([emp('a')]), {'success': False, 'code': 110004, 'data': []}])
        result = d.Discovery(fake).run(args())
        self.assertEqual(result['status'], 'partial')
        self.assertEqual(result['coverage']['searches'][0]['stop_reason'], 'business_error_110004')

    def test_lifecycle_items_not_collapsed_to_group(self):
        g = group('g', 'same', 'same')
        g['searchContractListItems'][1]['businessTypeCode'] = 2
        result = d.Discovery(Fake([ok([emp('a')]), page([g], 1)])).run(args())
        self.assertEqual(len(result['businesses']), 2)

    def test_cli_rejects_unverified_profile_and_blank_name(self):
        for argv in (['--employee-name', '合成人员', '--profile', 'prod'],
                     ['--employee-name', '   '], ['--department-name', '']):
            with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                d.parse_args([*argv, '--output', '/tmp/unused'])
            self.assertEqual(error.exception.code, 2)

    def test_limits_default_to_zero_accept_zero_and_reject_negative(self):
        for flag in ('--max-businesses', '--max-pages', '--max-member-pages', '--max-departments'):
            with self.subTest(flag=flag):
                field = flag[2:].replace('-', '_')
                self.assertEqual(getattr(args(), field), 0)
                self.assertEqual(getattr(args(flag, '0'), field), 0)
                self.assertEqual(getattr(args(flag, '1'), field), 1)
                with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as error:
                    args(flag, '-1')
                self.assertEqual(error.exception.code, 2)

    def test_malformed_contract_item_preserves_previous_pages(self):
        fake = Fake([ok([emp('a')]), page([group('g1', 'b1')], 2, [123]),
                     page([{'groupId': 'g2', 'searchContractListItems': [42]}], 2)])
        result = d.Discovery(fake).run(args())
        self.assertEqual(result['status'], 'partial')
        self.assertEqual(len(result['businesses']), 1)
        self.assertEqual(result['coverage']['searches'][0]['stop_reason'], 'invalid_business_item')

    def test_resume_profile_mismatch_rejected(self):
        first = d.Discovery(Fake([ok([emp('a')])])).run(args('--resolve-only'))
        with self.assertRaisesRegex(d.DiscoveryError, 'incompatible_resume'):
            other = args()
            other.profile = 'different-test-profile'
            d.Discovery(Fake([])).run(other, previous=first)


if __name__ == '__main__':
    unittest.main()
