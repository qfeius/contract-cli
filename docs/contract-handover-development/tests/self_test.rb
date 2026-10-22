# encoding: UTF-8
# frozen_string_literal: true

# These synthetic examples exercise the evidence checker, not the service or an
# Agent. They must never be copied into api-contract.yaml as runtime evidence.
module ContractHandoverValidation
  module SelfTest
    def self.contract
      {
        "business" => {
          "execution_policy" => {
            "selection_mode" => "FROZEN_RESOURCE_MANIFEST", "all_results_strategy" => "ENUMERATE_CONFIRM_SYNC_BATCHES",
            "async_dynamic_search_allowed" => false, "max_entry_resources_per_batch" => 100, "keep_group_together" => true,
            "mixed_batch_policy" => "EXPLAIN_EXCLUSIONS_THEN_CONFIRM_ELIGIBLE", "reconfirm_on_drift" => true,
            "blind_retry_allowed" => false, "cross_channel_replay_allowed" => false, "batches_execution" => "SERIAL",
            "stop_on_uncertain_write" => true, "write_recipe" => "handover"
          },
          "role_rules" => {
            "current_owner_source" => "CURRENT_RESOURCE", "submitter_only_handover_allowed" => false,
            "owner_submitter_update" => { "equal" => { "owner" => "RECIPIENT", "submitter" => "RECIPIENT" }, "different" => { "owner" => "RECIPIENT", "submitter" => "PRESERVE" } },
            "contract_assembly_rules_separate" => true,
            "assembly" => { "flags_source" => "ASSEMBLY_BEFORE_STATE", "third_party_actual_actor" => "OLD_ASSEMBLY_OWNER", "parent_department_update" => "ANY_EFFECTIVE_HANDOVER", "child_department_update" => "OWNER_FLAG_ONLY", "parent_authorization_covers_verified_children" => true },
            "initial_owner_rule" => { "scope" => "ORDINARY_CREATE_AND_DRAFT_INITIALIZATION", "equals_creator" => true, "exceptions" => %w[HISTORICAL_SNAPSHOT AUTOMATIC_COMPLETION_RECORD] }
          },
          "permission_rules" => { "server_final_decision" => true, "agent_admin_override_allowed" => false, "third_party_requires_not_owner_and_not_submitter" => true, "relationship_alone_sufficient" => false },
          "result_policy" => { "match_current_execution" => true, "match_resource_id" => true, "match_receiver" => true, "missing_record" => "UNVERIFIED", "preexisting_target_owner" => "NOT_NEW_SUCCESS" },
          "impact_gate" => { "required" => true, "complete_group_membership_required" => true, "assembly_membership_required" => true, "draft_effects_required" => true, "current_status" => "UNRESOLVED" },
          "ordinary_core_test" => { "enabled" => true, "environment" => "test", "result_scope" => "SELECTED_ENTRIES_ONLY" }
        },
        "runtime" => { "execution_mode" => "HYBRID", "target_environment" => "test", "transports" => {} },
        "recipes" => {
          "handover" => { "required" => true, "role" => "PRIMARY", "operation" => { "read_or_write" => "WRITE", "identity" => "USER", "confirmation" => "REQUIRED_BEFORE_EXECUTION" }, "response" => { "success" => { "predicates" => [{ "path" => "success", "equals" => true }] } } },
          "verify_contract_detail" => { "required" => true, "role" => "VERIFY", "operation" => { "read_or_write" => "READ" } }
        },
        "readiness" => { "contract_complete" => false, "delivery_status" => nil }, "evidence" => { "runtime" => [] }
      }
    end

    def self.evidence
      {
        "version" => 1, "provenance" => { "kind" => "SYNTHETIC_SELF_TEST", "synthetic" => true },
        "runs" => [{
          "id" => "synthetic-run-1", "scenario" => "ORDINARY_EQUAL_ROLES", "requested_action" => "HANDOVER", "transport" => "CLI",
          "recipient" => { "employee_id" => "receiver-2", "tenant_id" => "tenant-test", "id_type" => "INTERNAL_EMPLOYEE_ID", "department_ids" => [] },
          "operator" => { "employee_id" => "owner-1", "personal_handover" => true, "all_handover" => false },
          "selection" => { "mode" => "EXPLICIT", "frozen" => true },
          "manifest" => { "entries" => [{ "resource_id" => "entry-1", "group_id" => "group-1", "impact_ids" => ["contract-1"] }], "affected" => [{ "resource_id" => "contract-1", "resource_type" => ORDINARY }], "exclusions" => [] },
          "before" => [{ "resource_id" => "contract-1", "resource_type" => ORDINARY, "creator_id" => "owner-1", "owner_id" => "owner-1", "submitter_id" => "owner-1", "demand_employee_ids" => ["demand-1"], "department_id" => "department-old", "evidence_ref" => "synthetic-before" }],
          "after" => [{ "resource_id" => "contract-1", "resource_type" => ORDINARY, "owner_id" => "receiver-2", "submitter_id" => "receiver-2", "demand_employee_ids" => ["demand-1"], "department_id" => "department-old", "evidence_ref" => "synthetic-after" }],
          "confirmations" => [{ "id" => "confirm-1", "sequence" => 1, "user_confirmed" => true, "evidence_ref" => "synthetic-only", "entry_ids" => ["entry-1"], "impact_ids" => ["contract-1"], "excluded_ids" => [], "recipient_employee_id" => "receiver-2" }],
          "calls" => [
            { "sequence" => 2, "kind" => "WRITE", "recipe" => "handover", "request_id" => "request-1", "entry_ids" => ["entry-1"], "impact_ids" => ["contract-1"], "recipient_employee_id" => "receiver-2", "recipient_tenant_id" => "tenant-test", "admin_flag_supplied" => false, "identity_override_supplied" => false, "impact_complete" => true, "confirmation_id" => "confirm-1", "confirmed_snapshot_unchanged" => true, "started_at" => "2026-10-08T10:00:00+08:00", "finished_at" => "2026-10-08T10:00:01+08:00", "outcome" => "CONFIRMED", "transport" => "CLI", "http_success" => true, "business_success" => true },
            { "sequence" => 3, "kind" => "VERIFY", "recipe" => "verify_contract_detail", "request_id" => "verify-1", "resource_ids" => ["contract-1"], "http_success" => true, "business_success" => true }
          ],
          "records" => [{ "resource_id" => "contract-1", "request_id" => "request-1", "execution_id" => "synthetic-run-1", "receiver_employee_id" => "receiver-2", "status" => "SUCCESS", "evidence_ref" => "synthetic-response-authorization", "attribution" => "AUTHORIZATION_IN_CURRENT_RESPONSE" }],
          "results" => [{ "resource_id" => "contract-1", "status" => "VERIFIED_CHANGED" }], "reported_success_count" => 1, "reported_all_success" => true
        }]
      }
    end

    def self.ordinary_selected_entries!(run)
      run["scope_mode"] = "ORDINARY_SELECTED_ENTRIES"
      run["environment"] = "test"
      run["result_scope"] = "SELECTED_ENTRIES_ONLY"
      run["all_related_effects_verified"] = false
      run["related_effects"] = { "management" => "SERVER_MANAGED", "disclosed" => true, "strict_no_related_changes" => false,
                                 "evidence_ref" => "synthetic-related-effects-disclosure" }
      run["manifest"]["entries"][0]["resource_id"] = "contract-1"
      %w[entries affected].each do |collection|
        run["manifest"][collection].each { |row| row.merge!("resource_type" => ORDINARY, "handler" => ORDINARY, "assembly_id" => "0") }
      end
      %w[before after].each do |collection|
        run[collection].each { |row| row.merge!("handler" => ORDINARY, "assembly_id" => "0") }
      end
      run["confirmations"][0].merge!("entry_ids" => ["contract-1"], "related_effects_accepted" => true)
      run["calls"][0].merge!("entry_ids" => ["contract-1"], "impact_complete" => false)
    end

    def self.assembly!(run, branch)
      both = branch == "ASSEMBLY_BOTH_ROLES"
      submitter_only = branch == "ASSEMBLY_SUBMITTER_ONLY"
      run["scenario"] = branch
      run["recipient"]["department_ids"] = ["department-target", "department-secondary"]
      run["operator"]["employee_id"] = submitter_only ? "submitter-3" : "owner-1"
      run["manifest"] = {
        "entries" => [{ "resource_id" => "assembly-1", "group_id" => "group-1", "impact_ids" => %w[assembly-1 child-1] }],
        "affected" => [{ "resource_id" => "assembly-1", "resource_type" => "CONTRACT_ASSEMBLY", "handler" => "CONTRACT_ASSEMBLY" }, { "resource_id" => "child-1", "resource_type" => ORDINARY, "handler" => "CONTRACT_ASSEMBLY", "parent_assembly_id" => "assembly-1" }],
        "exclusions" => [],
        "assembly_memberships" => [{ "assembly_id" => "assembly-1", "member_resource_ids" => ["child-1"], "expected_member_count" => 1, "complete" => true, "evidence_ref" => "synthetic-complete-membership" }]
      }
      run["before"] = [
        { "resource_id" => "assembly-1", "resource_type" => "CONTRACT_ASSEMBLY", "owner_id" => "owner-1", "submitter_id" => both ? "owner-1" : "submitter-3", "department_id" => "parent-department", "evidence_ref" => "synthetic-parent-before" },
        { "resource_id" => "child-1", "resource_type" => ORDINARY, "owner_id" => "child-owner", "submitter_id" => "child-submitter", "department_id" => "child-department", "demand_employee_ids" => [], "evidence_ref" => "synthetic-child-before" }
      ]
      run["after"] = [
        { "resource_id" => "assembly-1", "resource_type" => "CONTRACT_ASSEMBLY", "owner_id" => submitter_only ? "owner-1" : "receiver-2", "submitter_id" => (both || submitter_only) ? "receiver-2" : "submitter-3", "department_id" => "department-target", "evidence_ref" => "synthetic-parent-after" },
        { "resource_id" => "child-1", "resource_type" => ORDINARY, "owner_id" => submitter_only ? "child-owner" : "receiver-2", "submitter_id" => (both || submitter_only) ? "receiver-2" : "child-submitter", "department_id" => submitter_only ? "child-department" : "department-target", "demand_employee_ids" => [], "evidence_ref" => "synthetic-child-after" }
      ]
      run["confirmations"][0]["entry_ids"] = ["assembly-1"]
      run["confirmations"][0]["impact_ids"] = %w[assembly-1 child-1]
      run["calls"][0]["entry_ids"] = ["assembly-1"]
      run["calls"][0]["impact_ids"] = %w[assembly-1 child-1]
      run["calls"][1]["resource_ids"] = %w[assembly-1 child-1]
      # The service returns the parent Authorization, not one per child.
      run["records"][0]["resource_id"] = "assembly-1"
      run["results"] = %w[assembly-1 child-1].map { |id| { "resource_id" => id, "status" => "VERIFIED_CHANGED" } }
      run["reported_success_count"] = 2
    end

    def self.assembly_no_change!(run)
      assembly!(run, "ASSEMBLY_OWNER_ONLY")
      run["scenario"] = "PREEXISTING_TARGET_OWNER"
      run["before"].each { |row| row["owner_id"] = run["recipient"]["employee_id"] }
      run["after"] = run["before"].map { |row| row.merge("evidence_ref" => "synthetic-unchanged-after") }
      run["records"] = []
      run["results"].each { |row| row["status"] = "NOT_NEW_SUCCESS" }
      run["reported_success_count"] = 0
      run["reported_all_success"] = false
    end

    def self.unknown!(run)
      run["records"] = []
      run["after"] = []
      run["results"] = run["manifest"]["affected"].map { |row| { "resource_id" => row["resource_id"], "status" => "UNVERIFIED" } }
      run["reported_success_count"] = 0
      run["reported_all_success"] = false
    end

    def self.no_write!(run)
      run["calls"] = []
      run["confirmations"] = []
      run["records"] = []
      run["after"] = []
      run["results"] = run["manifest"]["affected"].map { |row| { "resource_id" => row["resource_id"], "status" => "NOT_EXECUTED" } }
      run["reported_success_count"] = 0
      run["reported_all_success"] = false
    end

    def self.batches!(run, count, size = 100)
      ids = (1..count).map { |i| "entry-#{i}" }
      run["manifest"]["entries"] = ids.each_with_index.map { |id, i| { "resource_id" => id, "group_id" => "group-#{i / 2}", "impact_ids" => [id] } }
      run["manifest"]["affected"] = ids.map { |id| { "resource_id" => id, "resource_type" => ORDINARY } }
      before = run["before"][0]
      run["before"] = ids.map { |id| before.merge("resource_id" => id) }
      after = run["after"][0]
      run["after"] = ids.map { |id| after.merge("resource_id" => id) }
      run["confirmations"][0]["entry_ids"] = ids
      run["confirmations"][0]["impact_ids"] = ids
      write = run["calls"][0]
      verify = run["calls"][1]
      record = run["records"][0]
      start = Time.iso8601("2026-10-08T10:00:00+08:00")
      run["calls"] = []
      run["records"] = []
      run["batch_reconciliations"] = []
      ids.each_slice(size).each_with_index do |batch, i|
        request_id = "request-#{i + 1}"
        run["calls"] << write.merge("sequence" => i * 3 + 2, "request_id" => request_id, "entry_ids" => batch, "impact_ids" => batch,
                                     "started_at" => (start + i * 2).iso8601, "finished_at" => (start + i * 2 + 1).iso8601)
        run["calls"] << verify.merge("sequence" => i * 3 + 3, "request_id" => "verify-#{i + 1}", "resource_ids" => batch)
        run["records"].concat(batch.map { |id| record.merge("resource_id" => id, "request_id" => request_id) })
        run["batch_reconciliations"] << { "request_id" => request_id, "sequence" => i * 3 + 4, "evidence_ref" => "synthetic-batch-reconciliation-#{i + 1}",
                                          "results" => batch.map { |id| { "resource_id" => id, "status" => "VERIFIED_CHANGED" } } }
      end
      run["results"] = ids.map { |id| { "resource_id" => id, "status" => "VERIFIED_CHANGED" } }
      run["reported_success_count"] = ids.length
      run["reported_all_success"] = true
      run["scenario"] = ids.length > size ? "SERIAL_GROUP_BATCHES" : "ORDINARY_EQUAL_ROLES"
    end

    def self.run
      results = []
      check = lambda do |name, expected, mutation|
        sample = evidence
        mutation.call(sample["runs"][0], sample) if mutation
        validator = Validator.new(contract)
        validator.validate_evidence(sample, synthetic_self_test: true)
        passed = validator.errors.empty? == expected
        results << { "name" => name, "status" => passed ? "PASS" : "FAIL", "expected_acceptance" => expected, "actual_acceptance" => validator.errors.empty? }
        results.last["diagnostics"] = validator.errors unless passed
      end
      check.call("ordinary equal roles", true, nil)
      check.call("ordinary different roles preserve submitter", true, lambda { |r, _| r["scenario"] = "ORDINARY_DIFFERENT_ROLES"; r["before"][0]["submitter_id"] = "submitter-3"; r["after"][0]["submitter_id"] = "submitter-3" })
      check.call("different-role submitter overwrite rejected", false, lambda { |r, _| r["scenario"] = "ORDINARY_DIFFERENT_ROLES"; r["before"][0]["submitter_id"] = "submitter-3" })
      check.call("ordinary selected-entry test accepts disclosed server-managed effects", true, lambda { |r, _| ordinary_selected_entries!(r) })
      check.call("selected-entry different roles preserve submitter without department or demand acceptance", true, lambda do |r, _|
        ordinary_selected_entries!(r)
        r["scenario"] = "ORDINARY_DIFFERENT_ROLES"
        r["before"][0]["submitter_id"] = r["after"][0]["submitter_id"] = "submitter-3"
        %w[before after].each do |collection|
          r[collection][0].delete("department_id")
          r[collection][0].delete("demand_employee_ids")
          r[collection][0]["assembly_id"] = nil
        end
      end)
      check.call("selected-entry missing related-effect disclosure rejected", false, lambda { |r, _| ordinary_selected_entries!(r); r["related_effects"].delete("evidence_ref") })
      check.call("selected-entry missing related-effect acceptance rejected", false, lambda { |r, _| ordinary_selected_entries!(r); r["confirmations"][0].delete("related_effects_accepted") })
      check.call("selected-entry ALL_RESULTS rejected even with pagination complete", false, lambda do |r, _|
        ordinary_selected_entries!(r)
        r["selection"].merge!("mode" => "ALL_RESULTS", "pagination_complete" => true, "dynamic_filter_submitted" => false)
      end)
      check.call("selected-entry actual assembly membership cannot be hidden by manifest", false, lambda { |r, _| ordinary_selected_entries!(r); r["before"][0]["assembly_id"] = "assembly-1" })
      check.call("selected-entry missing assembly observation rejected", false, lambda { |r, _| ordinary_selected_entries!(r); r["before"][0].delete("assembly_id") })
      check.call("selected-entry extra related verification resource rejected", false, lambda do |r, _|
        ordinary_selected_entries!(r)
        r["manifest"]["affected"] << r["manifest"]["affected"][0].merge("resource_id" => "unselected-related")
        r["results"] << { "resource_id" => "unselected-related", "status" => "UNVERIFIED" }
        r["before"] << r["before"][0].merge("resource_id" => "unselected-related")
        r["reported_all_success"] = false
      end)
      check.call("selected-entry fabricated complete impact rejected", false, lambda { |r, _| ordinary_selected_entries!(r); r["calls"][0]["impact_complete"] = true })
      check.call("selected-entry guarantee of unchanged related records rejected", false, lambda { |r, _| ordinary_selected_entries!(r); r["related_effects"]["strict_no_related_changes"] = true })
      check.call("selected-entry non-test environment rejected", false, lambda { |r, _| ordinary_selected_entries!(r); r["environment"] = "production" })
      check.call("selected-entry whole-related-scope success claim rejected", false, lambda { |r, _| ordinary_selected_entries!(r); r["all_related_effects_verified"] = true })
      check.call("creator differs from current owner", true, lambda { |r, _| r["scenario"] = "CURRENT_OWNER_DIFFERS_CREATOR"; r["before"][0]["creator_id"] = "old-creator" })
      check.call("creator cannot replace current owner", false, lambda { |r, _| r["before"][0]["creator_id"] = "old-creator"; r["after"][0]["owner_id"] = "old-creator" })
      check.call("preexisting target is not new success", false, lambda { |r, _| r["scenario"] = "PREEXISTING_TARGET_OWNER"; r["before"][0]["owner_id"] = "receiver-2" })
      check.call("historical record cannot prove current success", false, lambda { |r, _| r["scenario"] = "HISTORICAL_RECORD"; r["records"][0]["execution_id"] = "old-run" })
      check.call("HTTP success with empty records remains unknown", true, lambda { |r, _| r["scenario"] = "EMPTY_RECORDS"; unknown!(r) })
      check.call("missing evidence cannot be called skipped", false, lambda { |r, _| unknown!(r); r["results"][0]["status"] = "NOT_EXECUTED" })
      check.call("timeout with no replay remains unknown", true, lambda { |r, _| r["scenario"] = "TIMEOUT_STOP"; r["calls"][0]["outcome"] = "UNCERTAIN"; unknown!(r) })
      check.call("timeout then replay rejected", false, lambda { |r, _| r["scenario"] = "TIMEOUT_STOP"; r["calls"][0]["outcome"] = "UNCERTAIN"; replay = Marshal.load(Marshal.dump(r["calls"][0])); replay["sequence"] = 4; replay["request_id"] = "replay"; r["calls"] << replay; unknown!(r) })
      check.call("unconfirmed scope drift rejected", false, lambda { |r, _| r["calls"][0]["confirmed_snapshot_unchanged"] = false })
      check.call("identity override rejected", false, lambda { |r, _| r["calls"][0]["admin_flag_supplied"] = true })
      check.call("no-permission relationship cannot permit writes", false, lambda { |r, _| r["operator"]["personal_handover"] = false })
      check.call("submitter-only blocks writes", false, lambda { |r, _| r["scenario"] = "SUBMITTER_ONLY"; r["requested_action"] = "SUBMITTER_ONLY" })
      check.call("submitter-only correctly produces no write", true, lambda { |r, _| r["scenario"] = "SUBMITTER_ONLY"; r["requested_action"] = "SUBMITTER_ONLY"; no_write!(r) })
      check.call("absent permissions correctly produce no write", true, lambda { |r, _| r["scenario"] = "PERMISSION_DENIED"; r["operator"]["personal_handover"] = false; no_write!(r) })
      check.call("server rejection wins over precheck", true, lambda do |r, _|
        r["scenario"] = "SERVER_REJECT_AFTER_PRECHECK"
        r["precheck_passed"] = true
        r["calls"] = [r["calls"][0].merge("outcome" => "REJECTED", "business_success" => false)]
        r["records"][0].merge!("status" => "FAILED", "reason" => "explicit synthetic rejection", "attribution" => "EXPLICIT_SERVICE_ERROR")
        r["results"][0]["status"] = "FAILED"
        r["after"] = []
        r["reported_success_count"] = 0
        r["reported_all_success"] = false
      end)
      check.call("third-party branch uses observed permissions and roles", true, lambda { |r, _| r["scenario"] = "THIRD_PARTY"; r["permission_branch"] = "THIRD_PARTY"; r["operator"] = { "employee_id" => "third-person", "personal_handover" => false, "all_handover" => true } })
      check.call("ordinary initialization and inherited exception remain distinct", true, lambda do |r, _|
        r["scenario"] = "INITIAL_OWNER"
        r["calls"] = []
        r["observations"] = [
          { "resource_id" => "init-1", "creation_path" => "DRAFT_INITIALIZATION", "owner_id" => "creator-1", "creator_id" => "creator-1", "evidence_ref" => "synthetic-init" },
          { "resource_id" => "snapshot-1", "creation_path" => "HISTORICAL_SNAPSHOT", "owner_id" => "old-owner", "creator_id" => "copy-maker", "source_owner_id" => "old-owner", "evidence_ref" => "synthetic-inheritance" }
        ]
      end)
      check.call("excluded entries disclosed before confirmation", true, lambda do |r, _|
        r["scenario"] = "MIXED_RESULTS"
        r["manifest"]["exclusions"] = [{ "resource_id" => "excluded-entry", "reason" => "observed unsupported state" }]
        r["confirmations"][0]["excluded_ids"] = ["excluded-entry"]
        r["results"] << { "resource_id" => "excluded-entry", "status" => "EXCLUDED" }
        r["reported_all_success"] = false
      end)
      check.call("frozen all-results accepted", true, lambda { |r, _| r["scenario"] = "FROZEN_ALL_RESULTS"; r["selection"] = { "mode" => "ALL_RESULTS", "frozen" => true, "pagination_complete" => true, "dynamic_filter_submitted" => false } })
      check.call("dynamic full-selection payload rejected", false, lambda { |r, _| r["scenario"] = "FROZEN_ALL_RESULTS"; r["selection"] = { "mode" => "ALL_RESULTS", "frozen" => true, "pagination_complete" => true, "dynamic_filter_submitted" => true } })
      check.call("whole group cannot be split", false, lambda do |r, _|
        r["manifest"]["entries"] << { "resource_id" => "entry-2", "group_id" => "group-1", "impact_ids" => ["contract-1"] }
        r["confirmations"][0]["entry_ids"] << "entry-2"
      end)
      check.call("expanded count is not the 100-entry limit", true, lambda do |r, _|
        impacts = (1..101).map { |i| "contract-#{i}" }
        r["manifest"]["entries"][0]["impact_ids"] = impacts
        r["manifest"]["affected"] = impacts.map { |id| { "resource_id" => id, "resource_type" => ORDINARY } }
        r["confirmations"][0]["impact_ids"] = impacts
        r["calls"][0]["impact_ids"] = impacts
        original = r["before"][0]
        r["before"] = impacts.map { |id| original.merge("resource_id" => id) }
        unknown!(r)
      end)
      check.call("100 entry boundary accepted", true, lambda { |r, _| batches!(r, 100) })
      check.call("101 entries in one batch rejected", false, lambda { |r, _| batches!(r, 101, 101) })
      check.call("101 entries in intact serial groups accepted", true, lambda { |r, _| batches!(r, 101) })
      check.call("overlapping batches rejected", false, lambda { |r, _| batches!(r, 101); r["calls"][2]["started_at"] = r["calls"][0]["started_at"] })
      check.call("later distinct batch after timeout rejected", false, lambda { |r, _| batches!(r, 101); r["calls"][0]["outcome"] = "UNCERTAIN" })
      check.call("HTTP success with empty records cannot continue automatically", false, lambda { |r, _| batches!(r, 101); unknown!(r); r["batch_reconciliations"] = [] })
      check.call("partially unresolved batch cannot continue automatically", false, lambda do |r, _|
        batches!(r, 101)
        r["records"].reject! { |row| row["resource_id"] == "entry-1" }
        r["after"].reject! { |row| row["resource_id"] == "entry-1" }
        r["results"][0]["status"] = "UNVERIFIED"
        r["batch_reconciliations"][0]["results"][0]["status"] = "UNVERIFIED"
        r["reported_success_count"] -= 1
        r["reported_all_success"] = false
      end)
      check.call("missing prior-batch reconciliation blocks later write", false, lambda { |r, _| batches!(r, 101); r["batch_reconciliations"] = [] })
      check.call("reconciliation after the next write cannot justify it", false, lambda { |r, _| batches!(r, 101); r["batch_reconciliations"][0]["sequence"] = 7 })
      check.call("readback after the next write cannot justify continuation", false, lambda do |r, _|
        batches!(r, 101)
        r["calls"][1]["sequence"] = 8
        r["calls"].sort_by! { |call| call["sequence"] }
      end)
      check.call("rejected prior batch stops automatic continuation", false, lambda { |r, _| batches!(r, 101); r["calls"][0]["outcome"] = "REJECTED"; r["calls"][0]["business_success"] = false })
      check.call("explicit independent resume may use a new confirmed run", true, lambda do |r, sample|
        r["scenario"] = "TIMEOUT_STOP"
        r["calls"][0]["outcome"] = "UNCERTAIN"
        unknown!(r)
        next_run = evidence["runs"][0]
        next_run = JSON.parse(JSON.generate(next_run).gsub("entry-1", "resume-entry-2").gsub("contract-1", "resume-contract-2").gsub("group-1", "resume-group-2").gsub("synthetic-run-1", "synthetic-resume-2"))
        next_run["resume"] = { "previous_run_id" => r["id"], "user_instruction_ref" => "synthetic-explicit-continue", "preflight_evidence_ref" => "synthetic-fresh-independent-preflight" }
        next_run["confirmations"][0]["evidence_ref"] = "synthetic-explicit-continue"
        sample["runs"] << next_run
      end)
      check.call("explicit resume cannot replay unresolved prior impacts", false, lambda do |r, sample|
        r["scenario"] = "TIMEOUT_STOP"
        r["calls"][0]["outcome"] = "UNCERTAIN"
        unknown!(r)
        next_run = evidence["runs"][0]
        next_run["id"] = "synthetic-resume-overlap"
        next_run["records"][0]["execution_id"] = next_run["id"]
        next_run["resume"] = { "previous_run_id" => r["id"], "user_instruction_ref" => "synthetic-explicit-continue", "preflight_evidence_ref" => "synthetic-fresh-preflight" }
        sample["runs"] << next_run
      end)
      check.call("numeric ID precision risk rejected", false, lambda { |r, _| r["recipient"]["employee_id"] = 9007199254740993 })
      check.call("ordinary rules cannot certify assembly", false, lambda { |r, _| r["manifest"]["affected"][0]["resource_type"] = "CONTRACT_ASSEMBLY" })
      %w[ASSEMBLY_OWNER_ONLY ASSEMBLY_SUBMITTER_ONLY ASSEMBLY_BOTH_ROLES].each do |branch|
        check.call("#{branch} parent record and child readback accepted", true, lambda { |r, _| assembly!(r, branch) })
      end
      check.call("assembly cannot reuse each child's own role relationship", false, lambda { |r, _| assembly!(r, "ASSEMBLY_BOTH_ROLES"); r["after"][1]["submitter_id"] = "child-submitter" })
      check.call("assembly submitter-only child department is outside role acceptance", true, lambda { |r, _| assembly!(r, "ASSEMBLY_SUBMITTER_ONLY"); r["after"][1]["department_id"] = "department-target" })
      %w[missing null unexpected].each do |department_case|
        %w[VERIFIED_CHANGED NOT_NEW_SUCCESS].each do |state|
          check.call("assembly child #{department_case} department permits #{state}", true, lambda do |r, _|
            state == "VERIFIED_CHANGED" ? assembly!(r, "ASSEMBLY_BOTH_ROLES") : assembly_no_change!(r)
            case department_case
            when "missing"
              r["after"][1].delete("department_id")
            when "null"
              r["after"][1]["department_id"] = nil
            when "unexpected"
              r["after"][1]["department_id"] = "unrelated-department"
            end
          end)
        end
      end
      check.call("assembly child department snapshots are not required", true, lambda do |r, _|
        assembly!(r, "ASSEMBLY_OWNER_ONLY")
        r["before"][1].delete("department_id")
        r["after"][1].delete("department_id")
      end)
      check.call("assembly child wrong owner remains rejected without department", false, lambda do |r, _|
        assembly!(r, "ASSEMBLY_OWNER_ONLY")
        r["after"][1].delete("department_id")
        r["after"][1]["owner_id"] = "child-owner"
      end)
      check.call("assembly child changed demand-person list remains rejected", false, lambda { |r, _| assembly!(r, "ASSEMBLY_BOTH_ROLES"); r["after"][1]["demand_employee_ids"] = ["new-demand-person"] })
      check.call("assembly parent department transition remains required", false, lambda { |r, _| assembly!(r, "ASSEMBLY_OWNER_ONLY"); r["after"][0]["department_id"] = "parent-department" })
      check.call("ordinary department transition remains required", false, lambda { |r, _| r["after"][0]["department_id"] = "unexpected-department" })
      check.call("assembly parent no-change still checks department", false, lambda { |r, _| assembly_no_change!(r); r["after"][0]["department_id"] = "changed-parent-department" })
      check.call("assembly child no-change still checks submitter", false, lambda { |r, _| assembly_no_change!(r); r["after"][1]["submitter_id"] = "different-submitter" })
      check.call("parent authorization needs complete member evidence", false, lambda { |r, _| assembly!(r, "ASSEMBLY_OWNER_ONLY"); r["manifest"]["assembly_memberships"][0]["complete"] = false })
      check.call("sensitive evidence field rejected without value output", false, lambda { |_r, sample| sample["access_token"] = "synthetic-forbidden-placeholder" })

      bundled_contract = YAML.safe_load(File.read(File.expand_path("../references/api-contract.yaml", __dir__), encoding: "UTF-8"), aliases: false)
      cli_transport = bundled_contract.dig("runtime", "transports", "cli")
      results << { "name" => "bundled CLI contract requires capability without pinning a version or package",
                   "status" => cli_transport.is_a?(Hash) && (cli_transport.keys & %w[version package package_version download_url install_url]).empty? &&
                     bundled_contract.dig("runtime", "required_capability") == "universal_open_web_call" &&
                     cli_transport["entrypoint"] == "contract-cli api call --channel web" ? "PASS" : "FAIL" }

      candidate = contract
      candidate["readiness"] = { "contract_complete" => true, "delivery_status" => "RUNNABLE" }
      candidate["business"]["impact_gate"]["current_status"] = "CONFIRMED"
      candidate["runtime"]["transports"]["cli"] = { "status" => "RUNTIME_VERIFIED", "environment" => "test" }
      candidate["evidence"]["runtime"] = [{ "recipe" => "handover", "transport" => "CLI", "environment" => "test", "http_success" => true, "business_success" => true, "synthetic" => false, "evidence_ref" => "synthetic-test-of-validator-only", "request_id" => "synthetic", "sample" => "WRITE_AND_VERIFY" }]
      validator = Validator.new(candidate).validate_contract
      results << { "name" => "RUNNABLE missing required VERIFY is rejected", "status" => validator.errors.empty? ? "FAIL" : "PASS" }
      candidate["evidence"]["runtime"] << candidate["evidence"]["runtime"][0].merge("recipe" => "verify_contract_detail", "sample" => "READ")
      candidate["evidence"]["runtime"].each { |sample| sample.merge!("scope_mode" => "ORDINARY_SELECTED_ENTRIES", "result_scope" => "SELECTED_ENTRIES_ONLY") }
      validator = Validator.new(candidate).validate_contract
      results << { "name" => "selected-entry recipe evidence cannot upgrade the whole package to RUNNABLE", "status" => validator.errors.empty? ? "FAIL" : "PASS" }
      validator = Validator.new(contract).validate_evidence(evidence)
      results << { "name" => "synthetic captures cannot count as business runtime evidence", "status" => validator.errors.empty? ? "FAIL" : "PASS" }
      # Deliberately do not call report with synthetic cases to fabricate runtime
      # PASS. Every case above checks only the validator itself.
      report = { "suite" => "SYNTHETIC_VERIFIER_SELF_TEST", "business_runtime" => "NOT_RUN", "service_or_agent_exercised" => false, "cases" => results, "passed" => results.count { |row| row["status"] == "PASS" }, "total" => results.length }
      puts JSON.pretty_generate(report)
      results.all? { |row| row["status"] == "PASS" } ? 0 : 1
    end
  end
end
