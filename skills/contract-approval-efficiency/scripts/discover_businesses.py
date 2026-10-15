#!/usr/bin/env python3
"""Resolve a current employee roster and discover visible contract candidates.

Only calls the read-only Open Web recipes documented in references/discovery.md.
The output is a candidate inventory, never proof of completed approval.
"""
import argparse
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import sys
from datetime import datetime
from zoneinfo import ZoneInfo

PREFIX = '/open-web/web/clm/'
VERSION = 2
VISIBLE_SEARCH_KEY = '__visible_contracts__'
TASK_PATH = '/open-web/web/clm-openapi/v1/tasks/getTasks'


class DiscoveryError(Exception):
    pass


def identifier(value):
    if isinstance(value, bool) or not isinstance(value, (str, int)) or not str(value).strip():
        raise DiscoveryError('missing_or_invalid_internal_id')
    return str(value)


def employee(raw):
    if not isinstance(raw, dict):
        raise DiscoveryError('invalid_employee')
    result = {'employeeId': identifier(raw.get('employeeId'))}
    for key in ('name', 'displayName', 'status'):
        if key in raw:
            result[key] = raw[key]
    if raw.get('larkId') is not None:
        result['larkId'] = identifier(raw['larkId'])
    result['departments'] = [department(d) for d in raw.get('departments') or []]
    return result


def department(raw):
    if not isinstance(raw, dict):
        raise DiscoveryError('invalid_department')
    result = {'departmentId': identifier(raw.get('departmentId'))}
    for key in ('name', 'displayName', 'status'):
        if key in raw:
            result[key] = raw[key]
    if raw.get('parentDepartmentId') is not None:
        result['parentDepartmentId'] = identifier(raw['parentDepartmentId'])
    return result


def trees(nodes):
    """Flatten tree responses without mistaking every descendant for a name match."""
    if not isinstance(nodes, list):
        raise DiscoveryError('invalid_department_tree')
    found = {}
    queue = list(nodes)
    while queue:
        raw = queue.pop(0)
        if not isinstance(raw, dict):
            raise DiscoveryError('invalid_department')
        key = identifier(raw.get('departmentId'))
        if key not in found:
            found[key] = raw
            queue.extend(raw.get('childDepartments') or [])
    return found


def select(candidates, id_key, selected):
    if selected is not None:
        matches = [c for c in candidates if c[id_key] == selected]
        if not matches:
            raise DiscoveryError('selected_id_not_in_lookup_candidates')
        return matches[0]
    return candidates[0] if len(candidates) == 1 else None


def cli_caller(binary, profile):
    def call(method, path, query=None, body=None):
        command = [binary, 'api', 'call', '--channel', 'web', '--profile', profile,
                   '--as', 'user', '--output', 'json', method, path if path.startswith('/') else PREFIX + path,
                   '--header', 'Timezone-Offset: -480']
        for key, value in (query or {}).items():
            command.extend(['--query', f'{key}={value}'])
        if body is not None:
            command.extend(['--data', json.dumps(body, ensure_ascii=False)])
        try:
            result = subprocess.run(command, capture_output=True, text=True, timeout=120)
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise DiscoveryError(type(exc).__name__) from None
        if result.returncode:
            # Never echo raw stderr/stdout: the CLI owns authentication and transport.
            raise DiscoveryError(f'cli_failed_exit_{result.returncode}')
        try:
            return json.loads(result.stdout)
        except ValueError:
            raise DiscoveryError('invalid_cli_json') from None
    return call


class Discovery:
    def __init__(self, call):
        self.call = call

    def request(self, method, path, query=None, body=None):
        envelope = self.call(method, path, query, body)
        success_code = 0 if path == TASK_PATH else 200
        if not isinstance(envelope, dict) or envelope.get('success') is not True or envelope.get('code') != success_code:
            code = envelope.get('code') if isinstance(envelope, dict) else None
            safe_code = code if isinstance(code, int) else 'unknown'
            raise DiscoveryError(f'business_error_{safe_code}')
        return envelope.get('data')

    def resolve(self, args):
        out = {'version': VERSION, 'profile': args.profile, 'status': 'partial',
               'candidate_source': args.candidate_source,
               'source_metadata': {'endpoint': TASK_PATH if args.candidate_source == 'raw-tasks' else PREFIX + 'es/search/contractList',
                                   'identity': 'current_authorized_user', 'search_tab_code': 0,
                                   'page_size': args.page_size,
                                   'completed_window': {'start': args.start, 'end': args.end},
                                   'person_filter_applied': args.candidate_source == 'participant-index'},
               'scope': {'kind': 'employee' if args.employee_name else 'department',
                         'query': args.employee_name or args.department_name,
                         'include_descendants': not args.direct_department_only,
                         'department_basis': 'current_roster',
                         'source': {'visible-contracts': 'current_user_visible_contract_list',
                                    'participant-index': 'participant_index_non_exhaustive',
                                    'raw-tasks': 'bpm_assignee_origin_tasks'}[args.candidate_source],
                         'task_person_filter': 'apply_target_employee_ids_to_original_records'},
               'employees': [], 'approver_ids': [], 'businesses': [], 'diagnostics': [],
               'coverage': {'roster_complete': False, 'contract_search_complete': False},
               'continuation': {'searches': {}}}
        if args.employee_name:
            raw = self.request('GET', 'employee/findEmployeesByKeyword', {
                'keywords': args.employee_name, 'filterCurrentEmployee': 'false',
                'noFilterDeletedEmployee': 'true'})
            if not isinstance(raw, list):
                raise DiscoveryError('invalid_employee_list')
            candidates = list({e['employeeId']: e for e in map(employee, raw)}.values())
            chosen = select(candidates, 'employeeId', args.selected_employee_id)
        else:
            raw = self.request('GET', 'department/findDepartmentsByKeyword', {
                'keyword': args.department_name, 'requireAll': 'false', 'showAllStatus': 'true'})
            if not isinstance(raw, list):
                raise DiscoveryError('invalid_department_list')
            nodes = trees(raw)
            candidates = [department(d) for d in nodes.values() if any(
                args.department_name.casefold() in str(d.get(k) or '').casefold()
                for k in ('name', 'displayName'))]
            chosen = select(candidates, 'departmentId', args.selected_department_id)
        if chosen is None:
            out['status'] = 'needs_selection' if candidates else 'not_found'
            out['candidates'] = candidates
            return out
        out['scope']['resolved'] = chosen
        if args.employee_name:
            out['employees'] = [chosen]
            out['coverage']['roster_complete'] = True
        else:
            self.members(args, out, chosen['departmentId'], nodes)
        out['approver_ids'] = [e['employeeId'] for e in out['employees']]
        return out

    def members(self, args, out, root_id, known):
        queue, visited, people = [root_id], set(), {}
        complete = True
        while queue:
            dep_id = queue.pop(0)
            if dep_id in visited:
                continue
            if args.max_departments > 0 and len(visited) >= args.max_departments:
                out['diagnostics'].append({'reason': 'department_limit', 'remaining_department_ids': [dep_id] + queue})
                complete = False
                break
            visited.add(dep_id)
            if not args.direct_department_only:
                queue.extend(identifier(d['departmentId']) for d in known.get(dep_id, {}).get('childDepartments') or [])
            signatures = set()
            page = 1
            while True:
                try:
                    data = self.request('GET', 'department/organization/list', {
                        'parentDepartmentId': dep_id, 'employeePageIndex': page, 'employeePageSize': 50})
                    if not isinstance(data, dict) or not isinstance(data.get('employees'), list):
                        raise DiscoveryError('invalid_organization_page')
                    if not args.direct_department_only:
                        children = trees(data.get('subDepartments') or [])
                        known.update(children)
                        queue.extend(children)
                    rows = [employee(r) for r in data['employees']]
                    signature = tuple(sorted(e['employeeId'] for e in rows))
                    if rows and signature in signatures:
                        raise DiscoveryError('repeated_member_page')
                    signatures.add(signature)
                    for e in rows:
                        eid = e['employeeId']
                        if eid in people:
                            deps = {d['departmentId']: d for d in people[eid]['departments'] + e['departments']}
                            people[eid]['departments'] = list(deps.values())
                        else:
                            people[eid] = e
                    if not rows or data.get('hasMore') is False:
                        break  # Service can over-report hasMore on an empty terminal page.
                    if data.get('hasMore') is not True:
                        raise DiscoveryError('missing_member_has_more')
                    if args.max_member_pages > 0 and page >= args.max_member_pages:
                        raise DiscoveryError('member_page_limit')
                    page += 1
                except DiscoveryError as exc:
                    complete = False
                    out['diagnostics'].append({'departmentId': dep_id, 'page': page, 'reason': str(exc)})
                    break
        out['employees'] = list(people.values())
        out['coverage'].update(roster_complete=complete, department_ids=sorted(visited))

    def search_candidates(self, eid, state, inventory, args, diagnostics):
        state.pop('stop_reason', None)
        keys = set(state['business_keys'])
        groups = set(state['group_ids'])
        while not state['done']:
            pending = state['pending']
            while pending:
                item = pending[0]
                key = f"{item['businessTypeCode']}:{item['businessId']}"
                if key not in keys and args.max_businesses > 0 and len(keys) >= args.max_businesses:
                    state['stop_reason'] = 'business_limit'
                    break
                if key not in inventory:
                    inventory[key] = dict(item, matched_employee_ids=[], discovery_source=args.candidate_source)
                if eid is not None and eid not in inventory[key]['matched_employee_ids']:
                    inventory[key]['matched_employee_ids'].append(eid)
                keys.add(key)
                pending.pop(0)
            if pending:
                break
            if state.get('blocked'):
                state['stop_reason'] = state['blocked']
                break
            if state['exhausted']:
                state['done'] = True
                break
            business_limit = args.max_businesses > 0 and len(keys) >= args.max_businesses
            page_limit = args.max_pages > 0 and state['pages_read'] >= args.max_pages
            if business_limit or page_limit:
                state['stop_reason'] = 'business_limit' if business_limit else 'page_limit'
                break
            body = {'searchTabCode': 0, 'pageIndex': state['page_index'], 'pageSize': args.page_size,
                    'withStatistics': False, 'enableHighlight': False, 'viewTypeEnum': 'PLAIN_LIST_VIEW',
                    'conditionUnits': [], 'filterUnits': []}
            if eid is not None:
                body['filterUnits'] = [
                    {'searchField': 'APPROVE_AUTHORITY', 'searchValue': eid, 'unionType': 'MUST'}]
            if state['cursor'] is not None:
                body['searchAfter'] = state['cursor']
            try:
                data = self.request('POST', 'es/search/contractList', body=body)
                if not isinstance(data, dict) or not isinstance(data.get('content'), list):
                    raise DiscoveryError('invalid_contract_page')
                rows = data['content']
                total = data.get('filteredTotalElements')
                if isinstance(total, bool) or not isinstance(total, int) or total < 0:
                    raise DiscoveryError('missing_group_total')
                if state['expected_groups'] is not None and state['expected_groups'] != total:
                    raise DiscoveryError('changed_group_total_restart_required')
                state['expected_groups'] = total
                if not rows:
                    if len(groups) != total:
                        raise DiscoveryError('empty_page_before_total')
                    state['exhausted'] = True
                    continue
                page_ids, items = [], []
                for group in rows:
                    if not isinstance(group, dict):
                        raise DiscoveryError('invalid_contract_group')
                    page_ids.append(identifier(group.get('groupId')))
                    children = group.get('searchContractListItems')
                    if not isinstance(children, list) or not children:
                        raise DiscoveryError('missing_group_businesses')
                    for raw in children:
                        if not isinstance(raw, dict):
                            raise DiscoveryError('invalid_business_item')
                        btype = raw.get('businessTypeCode')
                        if isinstance(btype, bool) or not isinstance(btype, int):
                            raise DiscoveryError('invalid_business_type')
                        item = {'businessId': identifier(raw.get('contractId')), 'businessTypeCode': btype,
                                'groupId': identifier(group.get('groupId'))}
                        for field in ('assemblyId', 'contractName', 'contractNumber', 'contractStatusCode',
                                      'contractStatus', 'createTime', 'submitTime', 'archivedTime', 'updateTime'):
                            if raw.get(field) is not None:
                                item[field] = identifier(raw[field]) if field == 'assemblyId' else raw[field]
                        # Preserve only actual API metadata; never derive a process ID from a business ID.
                        if raw.get('processInstanceId'):
                            item['processInstanceId'] = identifier(raw['processInstanceId'])
                            item['processInstanceIdSource'] = 'list_item.processInstanceId'
                        progress = raw.get('progressVO')
                        if isinstance(progress, dict) and progress.get('processInstanceId'):
                            item['progressVO'] = {'processInstanceId': identifier(progress['processInstanceId'])}
                            if 'processInstanceId' not in item:
                                item['processInstanceId'] = identifier(progress['processInstanceId'])
                                item['processInstanceIdSource'] = 'list_item.progressVO.processInstanceId'
                        items.append(item)
                signature = hashlib.sha256(json.dumps(sorted(page_ids)).encode()).hexdigest()
                if signature in state['seen_pages'] or not set(page_ids) - groups:
                    raise DiscoveryError('repeated_contract_page')
                next_groups = groups | set(page_ids)
                if len(next_groups) > total:
                    raise DiscoveryError('group_total_inconsistent')
                cursor = data.get('searchAfter')
                # Keep fetched records even when the server omits the next cursor.
                groups = next_groups
                state['pending'] = items
                state['seen_pages'].append(signature)
                state['pages_read'] += 1
                state['page_index'] += 1
                state['exhausted'] = len(groups) == total
                state['cursor'] = cursor
                if not state['exhausted'] and (not isinstance(cursor, list) or not cursor):
                    state['blocked'] = 'missing_search_after_restart_required'
            except DiscoveryError as exc:
                state['stop_reason'] = str(exc)
                diagnostics.append({'candidate_source': args.candidate_source, 'employeeId': eid,
                                    'page': state['page_index'], 'reason': str(exc)})
                break
        state['business_keys'] = sorted(keys)
        state['group_ids'] = sorted(groups)

    def raw_tasks(self, args, out):
        """Use the BPM task cursor, never the asynchronous contract participant filter."""
        records = {row['task_instance_id']: row for row in out.get('task_records', [])}
        processes = {(row['businessId'], row['processInstanceId']): row
                     for row in out.get('process_instances', [])}
        states = out['continuation']['searches']
        for person in out['employees']:
            eid = identifier(person['employeeId'])
            if not person.get('larkId'):
                try:
                    details = self.request('GET', 'employee/getEmployeesByIds', {'employeeIds': eid})
                    matches = [raw for raw in details if isinstance(raw, dict)
                               and identifier(raw.get('employeeId')) == eid] if isinstance(details, list) else []
                    if len(matches) != 1 or matches[0].get('larkId') is None:
                        raise DiscoveryError('target_lark_id_unavailable')
                    person['larkId'] = identifier(matches[0]['larkId'])
                    person['larkIdSource'] = 'employee/getEmployeesByIds'
                except DiscoveryError as exc:
                    out['diagnostics'].append({'employeeId': eid, 'reason': str(exc)})
                    states.setdefault(eid + ':identity', {'done': False, 'pages_read': 0,
                                                       'stop_reason': str(exc), 'task_ids': [], 'pending': []})
                    continue
            states.pop(eid + ':identity', None)
            for status in (1, 0):
                key = f'{eid}:{status}'
                state = states.setdefault(key, {'employeeId': eid, 'task_status': status,
                    'pages_read': 0, 'cursor': None, 'seen_pages': [], 'task_ids': [],
                    'pending': [], 'done': False})
                state.pop('stop_reason', None)
                task_ids = set(state['task_ids'])
                try:
                    while not state['done']:
                        while state['pending']:
                            raw = state['pending'][0]
                            tid = identifier(raw.get('task_instance_id'))
                            bid = identifier(raw.get('contract_id'))
                            pid = identifier(raw.get('process_instance_id'))
                            end_time = raw.get('end_time')
                            in_window = status == 0 or (
                                isinstance(end_time, int) and not isinstance(end_time, bool)
                                and (args.start_ms is None or end_time >= args.start_ms)
                                and (args.end_ms is None or end_time < args.end_ms))
                            # Missing completed timestamps must remain inspectable, not be discarded as outside the window.
                            if status == 1 and not isinstance(end_time, int):
                                in_window = True
                            pkey = (bid, pid)
                            if in_window and pkey not in processes and args.max_businesses > 0 and len(processes) >= args.max_businesses:
                                raise DiscoveryError('business_limit')
                            record = records.setdefault(tid, dict(raw, discovered_for_employee_ids=[]))
                            if eid not in record['discovered_for_employee_ids']:
                                record['discovered_for_employee_ids'].append(eid)
                            record['candidate_in_window_or_pending'] = in_window or record.get('candidate_in_window_or_pending', False)
                            task_ids.add(tid)
                            if in_window:
                                process = processes.setdefault(pkey, {'businessId': bid, 'processInstanceId': pid,
                                    'processInstanceIdSource': 'bpm_task.process_instance_id',
                                    'discovery_source': 'raw-tasks', 'matched_employee_ids': [], 'source_task_ids': []})
                                if eid not in process['matched_employee_ids']:
                                    process['matched_employee_ids'].append(eid)
                                if tid not in process['source_task_ids']:
                                    process['source_task_ids'].append(tid)
                            state['pending'].pop(0)
                        if state.get('blocked'):
                            raise DiscoveryError(state['blocked'])
                        if args.max_pages > 0 and state['pages_read'] >= args.max_pages:
                            raise DiscoveryError('page_limit')
                        query = {'assignee_user_id': person['larkId'], 'task_status': status,
                                 'sort_type': 'DESC', 'cursor_size': args.page_size}
                        if state['cursor'] is not None:
                            query.update(last_cursor=state['cursor'][0], last_sort_value=state['cursor'][1])
                        data = self.request('GET', TASK_PATH, query=query)
                        rows = data.get('task_list') if isinstance(data, dict) else None
                        if not isinstance(rows, list):
                            raise DiscoveryError('invalid_raw_task_page')
                        state['pages_read'] += 1
                        if not rows:
                            state['done'] = True
                            break
                        tids = [identifier(raw.get('task_instance_id')) for raw in rows if isinstance(raw, dict)]
                        if len(tids) != len(rows):
                            raise DiscoveryError('invalid_raw_task_item')
                        signature = hashlib.sha256(json.dumps(tids).encode()).hexdigest()
                        if signature in state['seen_pages'] or not set(tids) - task_ids:
                            raise DiscoveryError('repeated_raw_task_page')
                        last = rows[-1]
                        # BPM selectTime: todo uses create_time; done uses end_time.
                        next_cursor = [last.get('cursor'), last.get('create_time' if status == 0 else 'end_time')]
                        state['pending'] = rows
                        state['seen_pages'].append(signature)
                        if any(isinstance(value, bool) or not isinstance(value, int) for value in next_cursor):
                            state['blocked'] = 'missing_raw_task_cursor_restart_required'
                        elif next_cursor == state['cursor']:
                            state['blocked'] = 'repeated_raw_task_cursor_restart_required'
                        else:
                            state['cursor'] = next_cursor
                        # A short page is not EOF (the first page defaults to 10 in the service).
                except DiscoveryError as exc:
                    state['stop_reason'] = str(exc)
                    out['diagnostics'].append({'employeeId': eid, 'task_status': status,
                                              'pages_read': state['pages_read'], 'reason': str(exc)})
                state['task_ids'] = sorted(task_ids)
        out['task_records'] = list(records.values())
        out['process_instances'] = list(processes.values())
        complete = all(state['done'] for state in states.values())
        out['coverage'].update(contract_search_complete=complete, candidate_pagination_complete=complete,
            candidate_source='raw-tasks', employee_count=len(out['employees']),
            unique_business_count=len({row['businessId'] for row in processes.values()}),
            process_instance_count=len(processes), raw_task_count=len(records),
            searches=[{'search_key': key, 'pages_read': state['pages_read'],
                       'tasks_found': len(state['task_ids']), 'complete': state['done'],
                       'pending_task_rows': len(state['pending']), 'stop_reason': state.get('stop_reason')}
                      for key, state in states.items()],
            limitation='BPM visible origin-task candidates; each contract/process still requires current-user read permission')
        self.finish_checkpoints(args, out, {row['businessId'] for row in processes.values()}, complete)
        out['status'] = 'complete' if out['coverage']['contract_search_complete'] and out['coverage']['roster_complete'] else 'partial'
        return out

    def finish_checkpoints(self, args, out, found_ids, complete):
        checkpoint_ids = sorted(set(out.get('checkpoint_contract_ids', []) + args.checkpoint_contract_id))
        out['checkpoint_contract_ids'] = checkpoint_ids
        missing_ids = [bid for bid in checkpoint_ids if bid not in found_ids]
        out['coverage']['checkpoints'] = {'requested': checkpoint_ids,
            'found': [bid for bid in checkpoint_ids if bid in found_ids], 'missing': missing_ids}
        out['diagnostics'] = [item for item in out['diagnostics'] if item.get('reason') != 'checkpoint_not_discovered']
        if complete and missing_ids:
            out['diagnostics'].append({'reason': 'checkpoint_not_discovered', 'contract_ids': missing_ids})
            out['coverage']['contract_search_complete'] = False

    def run(self, args, previous=None):
        if previous is not None:
            out = copy.deepcopy(previous)
            if out.get('version') != VERSION or out.get('profile') != args.profile or 'searches' not in out.get('continuation', {}):
                raise DiscoveryError('incompatible_resume_file')
            if out.get('candidate_source') != args.candidate_source:
                raise DiscoveryError('resume_candidate_source_mismatch_restart_required')
            if out.get('source_metadata', {}).get('page_size') != args.page_size:
                raise DiscoveryError('resume_page_size_mismatch_use_original_page_size')
            if out.get('source_metadata', {}).get('completed_window') != {'start': args.start, 'end': args.end}:
                raise DiscoveryError('resume_time_window_mismatch_restart_required')
            if out.get('status') in ('needs_selection', 'not_found', 'error'):
                raise DiscoveryError('resume_requires_resolved_people')
        else:
            out = self.resolve(args)
        if out['status'] in ('needs_selection', 'not_found'):
            return out
        if args.resolve_only:
            out['status'] = 'resolved' if out['coverage']['roster_complete'] else 'partial'
            return out
        if args.candidate_source == 'raw-tasks':
            return self.raw_tasks(args, out)
        inventory = {f"{b['businessTypeCode']}:{identifier(b['businessId'])}": b for b in out['businesses']}
        states = out['continuation']['searches']
        search_ids = ([None] if out['employees'] else []) if args.candidate_source == 'visible-contracts' else [
            identifier(person['employeeId']) for person in out['employees']]
        for eid in search_ids:
            state = states.setdefault(eid if eid is not None else VISIBLE_SEARCH_KEY,
                {'page_index': 0, 'pages_read': 0, 'cursor': None,
                'expected_groups': None, 'group_ids': [], 'seen_pages': [], 'business_keys': [],
                'pending': [], 'exhausted': False, 'done': False})
            self.search_candidates(eid, state, inventory, args, out['diagnostics'])
        out['businesses'] = list(inventory.values())
        complete = all(s['done'] for s in states.values())
        search_coverage = [{
                'search_key': eid, 'employeeId': None if eid == VISIBLE_SEARCH_KEY else eid,
                'pages_read': s['pages_read'], 'groups_found': len(s['group_ids']),
                'expected_groups': s['expected_groups'], 'businesses_found': len(s['business_keys']),
                'pending_business_rows': len(s['pending']), 'complete': s['done'],
                'stop_reason': s.get('stop_reason')} for eid, s in states.items()]
        full_source = args.candidate_source == 'visible-contracts'
        out['coverage'].update(contract_search_complete=complete and full_source,
            candidate_pagination_complete=complete, candidate_source=args.candidate_source,
            employee_count=len(out['employees']), unique_business_count=len(inventory), searches=search_coverage,
            per_employee=search_coverage if not full_source else [],
            limitation=('current_authorized_user_contract_list_scope_only' if full_source
                        else 'participant_index_can_omit_real_approval_tasks'))
        checkpoint_ids = sorted(set(out.get('checkpoint_contract_ids', []) + args.checkpoint_contract_id))
        out['checkpoint_contract_ids'] = checkpoint_ids
        found_ids = {b['businessId'] for b in inventory.values()}
        missing_ids = [bid for bid in checkpoint_ids if bid not in found_ids]
        out['coverage']['checkpoints'] = {'requested': checkpoint_ids,
            'found': [bid for bid in checkpoint_ids if bid in found_ids], 'missing': missing_ids}
        out['diagnostics'] = [item for item in out['diagnostics']
                              if item.get('reason') not in ('checkpoint_not_discovered', 'participant_index_non_exhaustive')]
        if complete and missing_ids:
            out['diagnostics'].append({'reason': 'checkpoint_not_discovered', 'contract_ids': missing_ids})
            out['coverage']['contract_search_complete'] = False
        if not full_source:
            out['diagnostics'].append({'reason': 'participant_index_non_exhaustive',
                'detail': 'Explicit fast mode: complete pagination does not establish complete actual participation.'})
        out['status'] = 'complete' if (complete and full_source and not missing_ids
            and out['coverage']['roster_complete']) else 'partial'
        return out


def parse_args(argv=None):
    p = argparse.ArgumentParser(description=__doc__)
    source = p.add_mutually_exclusive_group(required=True)
    source.add_argument('--employee-name')
    source.add_argument('--department-name')
    source.add_argument('--resume-file', type=Path)
    p.add_argument('--selected-employee-id')
    p.add_argument('--selected-department-id')
    p.add_argument('--direct-department-only', action='store_true')
    p.add_argument('--resolve-only', action='store_true')
    p.add_argument('--profile', default='contract-test', choices=['contract-test'])
    p.add_argument('--cli', default='contract-cli')
    p.add_argument('--candidate-source', choices=['raw-tasks', 'visible-contracts', 'participant-index'],
                   default='raw-tasks',
                   help='default enumerates actual BPM tasks; visible-contracts is an explicit list fallback; participant-index is non-exhaustive')
    p.add_argument('--start', help='completed task window start, inclusive ISO timestamp; naive timestamps use Asia/Shanghai')
    p.add_argument('--end', help='completed task window end, exclusive ISO timestamp; todo candidates are always preserved')
    p.add_argument('--page-size', type=int, default=100,
                   help='contract groups per page (1..500, default 100); preserve this value on resume')
    p.add_argument('--checkpoint-contract-id', action='append', default=[],
                   help='verify a known contract is discovered automatically; repeatable, never inserts missing records')
    p.add_argument('--max-businesses', type=int, default=0,
                   help='per-search cumulative business limit; 0 means unlimited (default)')
    p.add_argument('--max-pages', type=int, default=0,
                   help='per-search cumulative contract page limit; 0 means unlimited (default)')
    p.add_argument('--max-member-pages', type=int, default=0,
                   help='per-department member page limit; 0 means unlimited (default)')
    p.add_argument('--max-departments', type=int, default=0,
                   help='department traversal limit; 0 means unlimited (default)')
    p.add_argument('--output', type=Path, required=True)
    args = p.parse_args(argv)
    for field in ('employee_name', 'department_name'):
        if getattr(args, field) is not None and not getattr(args, field).strip():
            p.error('name must be non-empty')
    if any(getattr(args, k) < 0 for k in ('max_businesses', 'max_pages', 'max_member_pages', 'max_departments')):
        p.error('limits must be non-negative; 0 means unlimited')
    if not 1 <= args.page_size <= 500:
        p.error('--page-size must be between 1 and 500')
    if any(not value.strip() for value in args.checkpoint_contract_id):
        p.error('--checkpoint-contract-id must be non-empty')
    for field in ('start', 'end'):
        value = getattr(args, field)
        try:
            parsed = datetime.fromisoformat(value.replace('Z', '+00:00')) if value else None
            if parsed is not None and parsed.tzinfo is None:
                parsed = parsed.replace(tzinfo=ZoneInfo('Asia/Shanghai'))
            setattr(args, field + '_ms', int(parsed.timestamp() * 1000) if parsed is not None else None)
        except ValueError:
            p.error('--start/--end must be ISO timestamps')
    if args.start_ms is not None and args.end_ms is not None and args.start_ms >= args.end_ms:
        p.error('--start must precede --end')
    if args.selected_employee_id and not args.employee_name:
        p.error('--selected-employee-id requires --employee-name')
    if args.selected_department_id and not args.department_name:
        p.error('--selected-department-id requires --department-name')
    return args


def main():
    args = parse_args()
    try:
        previous = json.loads(args.resume_file.read_text()) if args.resume_file else None
        result = Discovery(cli_caller(args.cli, args.profile)).run(args, previous)
    except (DiscoveryError, ValueError, OSError) as exc:
        reason = str(exc) if isinstance(exc, DiscoveryError) else type(exc).__name__
        result = {'version': VERSION, 'profile': args.profile, 'candidate_source': args.candidate_source,
                  'status': 'error', 'diagnostics': [{'reason': reason}]}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'status': result['status'], 'output': str(args.output)}, ensure_ascii=False))
    return 0 if result['status'] in ('complete', 'resolved') else 2


if __name__ == '__main__':
    sys.exit(main())
