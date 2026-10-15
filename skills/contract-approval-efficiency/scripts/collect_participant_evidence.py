#!/usr/bin/env python3
"""Collect conservative historical node participant evidence with the test user.

The participant endpoint alone is NOT complete for finished shared OR tasks.
Single-person proof additionally requires the exact published definition, a
fixed singleton assignment and no reassignment/dynamic-override evidence.
Only the minimal IDs/configuration needed to audit that proof are persisted.
"""
import argparse
from collections import Counter, defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
import hashlib
import json
from pathlib import Path
import subprocess
import sys
from zoneinfo import ZoneInfo

from discover_businesses import DiscoveryError

PARTICIPANT_PATH = '/open-web/web/clm/workflow/runtime/listAllApprovalEmployees'
FORM_PATH = '/open-web/web/clm/form/getFormDefinition'
PROCESS_PATH = '/open-web/web/clm-openapi/v1/process_instances/'
MUTATIONS = {3, 5, 6, 10, 12, 15, 16, 17, 18, 19, 20, 21, 23, 24}
SAFE_COMMANDS = {None, '', 'general', 'agree', 'startandsubmit', 'submit', 'autoEnd',
                 'autoCompleted', 'skipNode', 'reject', 'rejectAndTerminate',
                 'rollBackTaskByExpression', 'recover', 'recoverAndCancel'}


def cli_caller(binary, profile):
    def call(method, path, query=None):
        if method != 'GET' or not (path in (PARTICIPANT_PATH, FORM_PATH) or path.startswith(PROCESS_PATH)):
            raise DiscoveryError('unsupported_evidence_recipe')
        command = [binary, 'api', 'call', '--channel', 'web', '--profile', profile,
                   '--as', 'user', '--output', 'json', 'GET', path, '--header', 'Timezone-Offset: -480']
        for key, value in (query or {}).items():
            command.extend(['--query', f'{key}={value}'])
        try:
            result = subprocess.run(command, capture_output=True, text=True, timeout=120)
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise DiscoveryError(type(exc).__name__) from None
        if result.returncode:
            raise DiscoveryError(f'cli_failed_exit_{result.returncode}')
        try:
            return json.loads(result.stdout)
        except ValueError:
            raise DiscoveryError('invalid_cli_json') from None
    return call


def ident(value):
    return str(value) if isinstance(value, (str, int)) and not isinstance(value, bool) and str(value) else None


def employees(value):
    return {ident(e.get('employeeId')) for e in value or [] if isinstance(e, dict) and ident(e.get('employeeId'))}


def successful(envelope):
    if not isinstance(envelope, dict) or envelope.get('success') is not True or envelope.get('code') != 200:
        code = envelope.get('code') if isinstance(envelope, dict) else None
        raise DiscoveryError('participant_business_error_' + str(code if isinstance(code, int) else 'unknown'))
    return envelope.get('data')


def minimize_participants(envelope):
    data = successful(envelope)
    if not isinstance(data, list):
        raise DiscoveryError('invalid_participant_list')
    result = []
    for person in data:
        if not isinstance(person, dict) or not ident(person.get('employeeId')):
            raise DiscoveryError('invalid_participant_identity')
        nodes = person.get('nodeId2NameMap')
        # nodeId is just the first node; it is not a complete replacement.
        if not isinstance(nodes, dict) or not nodes or not all(ident(k) for k in nodes):
            raise DiscoveryError('missing_participant_node_map')
        result.append({'employeeId': ident(person['employeeId']), 'larkId': ident(person.get('larkId')),
                       'node_ids': sorted(nodes)})
    return {'employees': result}


def minimize_process(envelope):
    if not isinstance(envelope, dict) or envelope.get('success') is not True or envelope.get('code') != 0:
        code = envelope.get('code') if isinstance(envelope, dict) else None
        raise DiscoveryError('process_business_error_' + str(code if isinstance(code, int) else 'unknown'))
    data = (envelope.get('data') or {}).get('process_instance')
    if not isinstance(data, dict) or not isinstance(data.get('task_instance_list'), list):
        raise DiscoveryError('invalid_process_instance_tasks')
    tasks = []
    for task in data['task_instance_list']:
        if not isinstance(task, dict):
            raise DiscoveryError('invalid_raw_process_task')
        if not ident(task.get('task_instance_id')) or not ident(task.get('node_id')) or not isinstance(task.get('assignee_ids'), list):
            raise DiscoveryError('invalid_raw_process_task')
        tasks.append({key: task.get(key) for key in ('task_instance_id', 'node_id', 'command_type', 'assignee_ids')})
        tasks[-1]['assignee_ids'] = [ident(value) for value in tasks[-1]['assignee_ids']]
    return {'processInstanceId': ident(data.get('process_instance_id')),
            'processDefinitionId': ident(data.get('process_definition_id')), 'tasks': unique_raw_tasks(tasks)}


def unique_raw_tasks(tasks):
    unique = {}
    for task in tasks:
        if not isinstance(task, dict) or not ident(task.get('task_instance_id')) or not ident(task.get('node_id')) or not isinstance(task.get('assignee_ids'), list):
            raise DiscoveryError('invalid_raw_process_task')
        if any(value is not None and ident(value) is None for value in task['assignee_ids']):
            raise DiscoveryError('invalid_raw_process_assignee')
        # Null is a real missing mapping returned for some robot/deleted users.
        # Preserve it for whole-instance conflict detection; only the node
        # being proved requires complete personnel identities.
        normalized = dict(task, assignee_ids=sorted({ident(value) for value in task['assignee_ids']}, key=lambda value: (value is None, value or '')))
        key = task['task_instance_id']
        if key in unique and unique[key] != normalized:
            raise DiscoveryError('conflicting_raw_process_task')
        unique[key] = normalized
    return list(unique.values())


def minimize_form(envelope):
    data = successful(envelope)
    if not isinstance(data, dict) or not ident(data.get('processDefinitionId')) or not ident(data.get('formId')):
        raise DiscoveryError('invalid_historical_definition')
    chart = data.get('processDefinitionJson')
    if isinstance(chart, str):
        try:
            chart = json.loads(chart)
        except ValueError:
            raise DiscoveryError('invalid_historical_definition_json') from None
    if not isinstance(chart, dict) or not isinstance(chart.get('node_models'), list):
        raise DiscoveryError('missing_historical_node_models')
    nodes = []
    for node in chart['node_models']:
        if not isinstance(node, dict):
            raise DiscoveryError('invalid_historical_node')
        props = node.get('component_props') or {}
        form = props.get('clm_form') or {}
        approver = form.get('approver') or {}
        nodes.append({'node_id': node.get('node_id'), 'approvers_bo': node.get('approvers_bo'),
                      'task_mode': node.get('task_mode'),
                      'personnel_type_code': approver.get('personnel_type_code'),
                      'employee_ids': [ident(v) for v in approver.get('employee_ids') or []]})
    return {'formId': ident(data.get('formId')), 'processDefinitionId': ident(data['processDefinitionId']),
            'published': data.get('published'), 'nodes': nodes}


def in_scope(record, meta, target_ids):
    if target_ids is not None and not target_ids.intersection(employees(record.get('assigneeEmployees'))):
        return False
    tz = ZoneInfo(meta.get('timezone', 'Asia/Shanghai'))
    def parse(value):
        parsed = datetime.fromisoformat(value)
        return parsed.replace(tzinfo=tz) if parsed.tzinfo is None else parsed
    try:
        start, end = parse(meta['start']), parse(meta['end'])
        if record.get('endTime'):
            return start <= parse(record['endTime']) < end
        return record.get('taskStatusCode') == 0 and parse(record['createTime']) < end
    except (ValueError, TypeError, KeyError):
        return True  # malformed time remains a diagnostic; never silently skip it


def index_bundle(bundle):
    groups, process_requests, bindings = {}, {}, defaultdict(list)
    target_ids = None
    if 'approver_ids' in bundle:
        target_ids = {ident(e) for e in bundle['approver_ids']}
    elif 'employees' in bundle:
        target_ids = employees(bundle['employees'])
    for wrapper in bundle.get('instances', []):
        pid = ident(wrapper.get('processInstanceId'))
        metadata = wrapper.get('definition_metadata') or wrapper
        if pid and metadata.get('formId') and ident(metadata.get('processInstanceId')) == pid:
            bindings[pid].append(dict(metadata, response=wrapper['response']))
        data = successful(wrapper.get('response'))
        for section in ('done', 'todo'):
            for node in data.get(section) or []:
                for record in node.get('records') or []:
                    rpid = ident(record.get('processInstanceId'))
                    definition = ident(record.get('processDefinitionId'))
                    nid = ident(record.get('nodeId') or node.get('nodeId'))
                    if not rpid or not definition or not nid:
                        continue
                    key = (rpid, definition, nid)
                    group = groups.setdefault(key, {'records': [], 'observed': set(), 'wanted': False})
                    group['records'].append(record)
                    group['observed'].update(employees(record.get('assigneeEmployees')))
                    group['observed'].update(employees(node.get('assigneeEmployees')))
                    if node.get('nodeTypeCode') in (1, 2, 3) and in_scope(record, bundle.get('meta', {}), target_ids):
                        group['wanted'] = True
                    bid = ident(record.get('businessId'))
                    btype = record.get('businessTypeCode')
                    if rpid == pid:
                        bid = bid or ident(wrapper.get('businessId'))
                        btype = btype if btype is not None else wrapper.get('businessTypeCode')
                    if bid and isinstance(btype, int):
                        candidate = {'businessId': bid, 'businessTypeCode': btype, 'processInstanceId': rpid,
                                     'filterRobot': 'false'}
                        existing = process_requests.get(rpid)
                        if existing is not None and existing != candidate:
                            process_requests[rpid] = False
                        elif rpid not in process_requests:
                            process_requests[rpid] = candidate
    return groups, process_requests, bindings


def prove_node(key, group, participant_data, process_data, forms, bindings):
    pid, definition, nid = key
    returned = {p['employeeId'] for p in participant_data['employees'] if nid in p['node_ids']}
    observed = group['observed']
    union = observed | returned
    if not union:
        return None, 'empty_node_people'
    base = {'processInstanceId': pid, 'processDefinitionId': definition, 'nodeId': nid,
            'employee_ids': sorted(union), 'complete': False,
            'source': f'GET {PARTICIPANT_PATH}; exact processInstanceId={pid}; nodeId2NameMap; filterRobot=false'}
    if len(union) > 1:
        return base, 'multiple_historical_people'
    if not returned or not observed <= returned:
        return base, 'participant_response_missing_visible_people'
    if not process_data or process_data['processInstanceId'] != pid or process_data['processDefinitionId'] != definition:
        return base, 'raw_process_instance_not_matched'
    try:
        verified_tasks = unique_raw_tasks(process_data['tasks'])
    except DiscoveryError as exc:
        return base, str(exc)
    raw_tasks = [task for task in verified_tasks if task['node_id'] == nid]
    visible_task_ids = {ident(record.get('processTaskInstanceId')) for record in group['records']}
    if not visible_task_ids <= {task['task_instance_id'] for task in raw_tasks}:
        return base, 'raw_process_missing_visible_tasks'
    id_map = {p['larkId']: p['employeeId'] for p in participant_data['employees'] if p.get('larkId')}
    raw_ids = {value for task in raw_tasks for value in task['assignee_ids']}
    if None in raw_ids:
        return base, 'invalid_raw_process_assignee'
    if not raw_ids or not raw_ids <= set(id_map):
        return base, 'raw_process_assignee_mapping_incomplete'
    raw_people = {id_map[value] for value in raw_ids}
    if len(raw_people | union) > 1:
        base['employee_ids'] = sorted(raw_people | union)
        return base, 'multiple_raw_historical_people'
    if any(task['command_type'] not in SAFE_COMMANDS for task in raw_tasks):
        return base, 'raw_reassignment_or_unknown_command'
    for record in group['records']:
        if record.get('commandTypeCode') in MUTATIONS or record.get('parentCommandTypeCode') in MUTATIONS or record.get('parentProcessTaskInstanceId') or record.get('agentEmployees') or record.get('endorsementEmployees'):
            return base, 'reassignment_or_endorsement_requires_full_history'
    matches = []
    for binding in bindings.get(pid, []):
        form = forms.get(ident(binding.get('formId')))
        if not form or form['processDefinitionId'] != definition:
            continue
        matches.append((binding, form))
    if not matches:
        return base, 'historical_definition_not_matched'
    proofs = []
    for binding, form in matches:
        if form.get('published') is not True:
            continue
        configs = binding.get('nodeConfigMap')
        if not isinstance(configs, dict) or not isinstance(configs.get(nid), dict):
            continue
        config = configs[nid]
        if 'dynamicApproverConfig' not in config or config['dynamicApproverConfig'] is not None:
            continue
        matching_nodes = [n for n in form['nodes'] if n['node_id'] == nid]
        if len(matching_nodes) != 1:
            continue
        node = matching_nodes[0]
        expected = set()
        if node['personnel_type_code'] == 4 and node['approvers_bo'] == 'contract_submitter':
            # The exact historical instance must have a single submitter across
            # all retained submit events; current employee metadata is not used.
            data = binding['response']['data']
            for section in ('done', 'todo'):
                for parent in data.get(section) or []:
                    if parent.get('nodeTypeCode') != 0:
                        continue
                    for record in parent.get('records') or []:
                        if ident(record.get('processInstanceId')) == pid and ident(record.get('processDefinitionId')) == definition:
                            expected.update(employees(record.get('assigneeEmployees')))
        elif node['personnel_type_code'] == 0:
            fixed_ids = [value for value in node['employee_ids'] if value]
            # Published front-end display config and actual executable
            # approvers_bo can disagree; require the exact fixed expression.
            if fixed_ids and node['approvers_bo'] == 'user:' + ','.join(fixed_ids):
                expected = set(fixed_ids)
        # Configured candidates do not establish who was actually assigned:
        # runtime filtering can leave only one person. Retain the configuration
        # separately and keep this node unverified, without inventing observed
        # participants or relaxing the complete-singleton requirement.
        if len(expected) > 1:
            base['configured_employee_ids'] = sorted(expected)
            base['source'] += f'; historical formId={form["formId"]} definition matched; configured multiple candidates; actual runtime assignment unverified'
            return base, 'configured_multiple_runtime_unverified'
        if expected == union and len(expected) == 1:
            proofs.append(form['formId'])
    if len(proofs) != len(matches):
        return base, 'fixed_singleton_or_override_not_proven'
    base['complete'] = True
    base['source'] += ('; exact published historical processDefinitionId matched via GET ' + FORM_PATH
                       + '; formIds=' + ','.join(sorted(set(proofs)))
                       + '; fixed singleton assignment agrees with historical submit/participant records; fixed definition is not dynamic; current override null; full raw process task commands checked via GET '
                       + PROCESS_PATH + pid + '; no reassignment/endorsement')
    return base, 'proven_fixed_singleton'


def enrich_bundle(bundle, call, cache_dir=None, workers=4, resume=False, progress=None):
    groups, requests, bindings = index_bundle(bundle)
    wanted_pids = {key[0] for key, group in groups.items() if group['wanted']}
    cache_root = Path(cache_dir) if cache_dir else None
    if cache_root:
        cache_root.mkdir(parents=True, exist_ok=True)
    outcomes, processes, forms, diagnostics = {}, {}, {}, []
    jobs = []
    for pid in sorted(wanted_pids):
        if requests.get(pid):
            jobs.append(('participants', pid, PARTICIPANT_PATH, requests[pid], minimize_participants))
            jobs.append(('processes', pid, PROCESS_PATH + pid, {}, minimize_process))
        else:
            diagnostics.append({'processInstanceId': pid, 'reason': 'missing_or_conflicting_process_business_binding'})
    form_ids = {ident(binding['formId']) for pid in wanted_pids for binding in bindings.get(pid, [])}
    jobs += [('forms', fid, FORM_PATH, {'formId': fid}, minimize_form) for fid in sorted(form_ids)]
    def run(job):
        kind, key, path, query, minimize = job
        cache = cache_root / (kind + '-' + hashlib.sha256(key.encode()).hexdigest() + '.json') if cache_root else None
        if resume and cache and cache.exists():
            payload = json.loads(cache.read_text())
            if payload.get('path') == path and payload.get('query') == query:
                return kind, key, payload['data']
        result = minimize(call('GET', path, query))
        if kind == 'forms' and result['formId'] != key:
            raise DiscoveryError('returned_form_id_mismatch')
        if cache:
            cache.write_text(json.dumps({'path': path, 'query': query, 'data': result}, ensure_ascii=False, indent=2))
        return kind, key, result
    with ThreadPoolExecutor(max_workers=workers) as pool:
        futures = {pool.submit(run, job): job for job in jobs}
        for index, future in enumerate(as_completed(futures), 1):
            job = futures[future]
            try:
                kind, key, data = future.result()
                {'participants': outcomes, 'processes': processes, 'forms': forms}[kind][key] = data
            except (DiscoveryError, ValueError, OSError) as exc:
                diagnostics.append({'kind': job[0], 'id': job[1], 'reason': str(exc)})
            if progress:
                progress(index, len(jobs))
    evidence, reasons = [], Counter()
    for key, group in sorted(groups.items()):
        if not group['wanted']:
            continue
        if key[0] not in outcomes:
            reasons['participant_query_failed'] += 1
            continue
        item, reason = prove_node(key, group, outcomes[key[0]], processes.get(key[0]), forms, bindings)
        reasons[reason] += 1
        if item:
            evidence.append(item)
    bundle['node_participant_evidence'] = evidence
    bundle.setdefault('meta', {})['participant_evidence_collection'] = {
        'requested_process_instances': len(wanted_pids), 'queried_process_instances': len(outcomes),
        'raw_process_instances': len(processes),
        'requested_form_definitions': len(form_ids), 'queried_form_definitions': len(forms),
        'node_counts': dict(reasons), 'diagnostics': diagnostics,
        'policy': 'Exact historical definition + fixed singleton + historical participants + no reassignment; participant API alone is insufficient.'}
    return bundle


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', required=True)
    parser.add_argument('--output', required=True)
    parser.add_argument('--profile', default='contract-test', choices=['contract-test'])
    parser.add_argument('--cli', default='contract-cli')
    parser.add_argument('--cache-dir')
    parser.add_argument('--resume', action='store_true', help='Reuse minimized evidence cache from this same collection run')
    parser.add_argument('--workers', type=int, default=4)
    args = parser.parse_args()
    if not 1 <= args.workers <= 8:
        parser.error('--workers must be between 1 and 8')
    bundle = json.loads(Path(args.input).read_text())
    def progress(done, total):
        if done == total or done % 25 == 0:
            print(f'participant_evidence {done}/{total}', file=sys.stderr, flush=True)
    result = enrich_bundle(bundle, cli_caller(args.cli, args.profile), args.cache_dir, args.workers, args.resume, progress)
    Path(args.output).write_text(json.dumps(result, ensure_ascii=False, indent=2))
    print(json.dumps(result['meta']['participant_evidence_collection'], ensure_ascii=False))


if __name__ == '__main__':
    main()
