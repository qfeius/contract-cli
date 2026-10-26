#!/usr/bin/env ruby
# encoding: UTF-8
# frozen_string_literal: true

# Offline business-contract and redacted-evidence validator. This file never
# performs HTTP calls or executes a handover. Ruby 2.6+ and standard library only.
require "yaml"
require "json"
require "optparse"
require "time"

module ContractHandoverValidation
  EXIT_INVALID = 1
  EXIT_INCOMPLETE = 2
  ORDINARY = "ORDINARY_CONTRACT"
  RESULT_STATES = %w[VERIFIED_CHANGED NOT_NEW_SUCCESS EXCLUDED FAILED UNVERIFIED NOT_EXECUTED].freeze
  FORBIDDEN_KEYS = %w[access_token refresh_token app_secret device_code cookie authorization authorization_header raw_headers private_key].freeze

  class Validator
    attr_reader :errors, :gaps, :checks, :runtime_cases

    def initialize(contract)
      @contract = contract || {}
      @errors = []
      @gaps = []
      @checks = 0
      @runtime_cases = []
    end

    def check(condition, message)
      @checks += 1
      @errors << message unless condition
      !!condition
    end

    def expect(path, expected)
      actual = @contract.dig(*path.split("."))
      check(actual == expected, "#{path}: expected #{expected.inspect}, got #{actual.inspect}")
    end

    def validate_contract
      expect("business.execution_policy.selection_mode", "FROZEN_RESOURCE_MANIFEST")
      expect("business.execution_policy.all_results_strategy", "ENUMERATE_CONFIRM_SYNC_BATCHES")
      expect("business.execution_policy.async_dynamic_search_allowed", false)
      expect("business.execution_policy.max_entry_resources_per_batch", 100)
      expect("business.execution_policy.keep_group_together", true)
      expect("business.execution_policy.mixed_batch_policy", "EXPLAIN_EXCLUSIONS_THEN_CONFIRM_ELIGIBLE")
      expect("business.execution_policy.reconfirm_on_drift", true)
      expect("business.execution_policy.blind_retry_allowed", false)
      expect("business.execution_policy.cross_channel_replay_allowed", false)
      expect("business.execution_policy.batches_execution", "SERIAL")
      expect("business.execution_policy.stop_on_uncertain_write", true)
      expect("business.role_rules.current_owner_source", "CURRENT_RESOURCE")
      expect("business.role_rules.submitter_only_handover_allowed", false)
      expect("business.role_rules.owner_submitter_update.equal", { "owner" => "RECIPIENT", "submitter" => "RECIPIENT" })
      expect("business.role_rules.owner_submitter_update.different", { "owner" => "RECIPIENT", "submitter" => "PRESERVE" })
      expect("business.role_rules.contract_assembly_rules_separate", true)
      expect("business.role_rules.assembly.flags_source", "ASSEMBLY_BEFORE_STATE")
      expect("business.role_rules.assembly.third_party_actual_actor", "OLD_ASSEMBLY_OWNER")
      expect("business.role_rules.assembly.parent_department_update", "ANY_EFFECTIVE_HANDOVER")
      expect("business.role_rules.assembly.child_department_update", "OWNER_FLAG_ONLY")
      expect("business.role_rules.assembly.parent_authorization_covers_verified_children", true)
      expect("business.role_rules.initial_owner_rule.scope", "ORDINARY_CREATE_AND_DRAFT_INITIALIZATION")
      expect("business.role_rules.initial_owner_rule.equals_creator", true)
      exceptions = @contract.dig("business", "role_rules", "initial_owner_rule", "exceptions")
      check(exceptions.is_a?(Array) && !exceptions.empty?, "initial owner rule must enumerate exceptional creation paths")
      expect("business.permission_rules.server_final_decision", true)
      expect("business.permission_rules.agent_admin_override_allowed", false)
      expect("business.permission_rules.third_party_requires_not_owner_and_not_submitter", true)
      expect("business.permission_rules.relationship_alone_sufficient", false)
      %w[match_current_execution match_resource_id match_receiver].each { |key| expect("business.result_policy.#{key}", true) }
      expect("business.result_policy.missing_record", "UNVERIFIED")
      expect("business.result_policy.preexisting_target_owner", "NOT_NEW_SUCCESS")
      %w[required complete_group_membership_required assembly_membership_required draft_effects_required].each do |key|
        expect("business.impact_gate.#{key}", true)
      end
      if @contract.dig("business", "development_validation_enabled") == true
        expect("business.development_validation.environment", "test")
        %w[require_closed_scenario require_read_recipes_runtime_verified require_explicit_test_confirmation requires_dedicated_test_manifest].each do |key|
          expect("business.development_validation.#{key}", true)
        end
        expect("business.development_validation.normal_business_execution_allowed", false)
        expect("business.development_validation.promote_package_readiness_automatically", false)
      end
      if @contract.dig("business", "ordinary_core_test", "enabled") == true
        expect("business.ordinary_core_test.environment", "test")
        expect("business.ordinary_core_test.result_scope", "SELECTED_ENTRIES_ONLY")
      end

      recipes = @contract["recipes"] || {}
      write_id = @contract.dig("business", "execution_policy", "write_recipe")
      write = recipes[write_id]
      if check(write.is_a?(Hash), "execution_policy.write_recipe must identify the actual handover recipe")
        check(write.dig("operation", "read_or_write") == "WRITE", "handover recipe must be WRITE")
        check(write.dig("operation", "identity") == "USER", "handover recipe must use the authenticated USER")
        check(write.dig("operation", "confirmation") == "REQUIRED_BEFORE_EXECUTION", "handover requires concrete pre-execution confirmation")
        check(write["required"] == true && write["role"] == "PRIMARY", "handover must remain a required PRIMARY recipe")
        check(write.dig("response", "success", "predicates").is_a?(Array) && !write.dig("response", "success", "predicates").empty?, "handover must declare downstream success predicates")
      end
      required_verifies = recipes.select { |_id, r| r["required"] == true && r["role"] == "VERIFY" }
      check(!required_verifies.empty?, "at least one required VERIFY recipe must verify actual affected resources")
      required_verifies.each do |id, recipe|
        check(recipe.dig("operation", "read_or_write") == "READ", "#{id}: verification must be a read operation")
      end

      # These are delivery gaps, not made-up business failures. Never convert
      # them to PASS merely because the structural rules above are satisfied.
      @gaps << "contract_complete is not true" unless @contract.dig("readiness", "contract_complete") == true
      @gaps << "impact discovery remains unresolved" if @contract.dig("business", "impact_gate", "current_status") == "UNRESOLVED"
      status = @contract.dig("readiness", "delivery_status")
      @gaps << "delivery_status is not assigned" if status.nil?
      check([nil, "RECIPE_READY", "RUNNABLE"].include?(status), "unknown delivery_status; do not invent DRAFT in the v2 contract")
      if status == "RUNNABLE"
        check(@gaps.empty?, "RUNNABLE cannot have incomplete contract or unresolved impact gates")
        validate_runnable_evidence(required_verifies.keys)
      end
      self
    end

    def validate_runnable_evidence(required_verify_ids)
      required_ids = (@contract["recipes"] || {}).select { |_id, r| r["required"] == true }.keys
      required_ids |= required_verify_ids
      mode = @contract.dig("runtime", "execution_mode")
      allowed = { "HYBRID" => %w[cli mcp], "CLI_ONLY" => ["cli"], "MCP_ONLY" => ["mcp"] }[mode] || []
      environment = @contract.dig("runtime", "target_environment")
      samples = Array(@contract.dig("evidence", "runtime"))
      write_id = @contract.dig("business", "execution_policy", "write_recipe")
      covered = allowed.any? do |transport|
        state = @contract.dig("runtime", "transports", transport) || {}
        state["status"] == "RUNTIME_VERIFIED" && state["environment"] == environment && required_ids.all? do |recipe_id|
          samples.any? do |sample|
            sample["recipe"] == recipe_id && sample["environment"] == environment &&
              sample["scope_mode"] != "ORDINARY_SELECTED_ENTRIES" && sample["result_scope"] != "SELECTED_ENTRIES_ONLY" &&
              sample["transport"].to_s.downcase == transport &&
              sample["http_success"] == true && sample["business_success"] == true &&
              sample["synthetic"] == false && present?(sample["evidence_ref"]) &&
              (present?(sample["request_id"]) || present?(sample["trace_id"])) &&
              (recipe_id != write_id || sample["sample"] == "WRITE_AND_VERIFY")
          end
        end
      end
      check(covered, "RUNNABLE needs one permitted transport with actual evidence for EVERY required recipe, including VERIFY; the write requires WRITE_AND_VERIFY")
    end

    def validate_evidence(evidence, synthetic_self_test: false)
      initial_errors = @errors.length
      return self unless check(evidence.is_a?(Hash), "evidence must be an object")
      check(evidence["version"] == 1, "evidence.version must be 1")
      reject_sensitive_keys(evidence)
      provenance = evidence["provenance"] || {}
      if synthetic_self_test
        check(provenance["kind"] == "SYNTHETIC_SELF_TEST" && provenance["synthetic"] == true, "self-test input must explicitly identify itself as synthetic")
      else
        check(provenance["kind"] == "LIVE_CAPTURE" && provenance["synthetic"] == false, "business runtime evidence must be explicitly LIVE_CAPTURE, synthetic=false")
        %w[captured_at evidence_ref agent].each { |key| check(present?(provenance[key]), "runtime evidence provenance missing #{key}") }
        check(provenance["environment"] == "test", "this validation release accepts test runtime evidence only")
        check(%w[CLI MCP].include?(provenance["transport"]), "runtime evidence transport must be CLI or MCP")
      end
      runs = evidence["runs"]
      return self unless check(runs.is_a?(Array) && !runs.empty?, "evidence.runs must be a non-empty array")
      seen = {}
      runs.each do |run|
        before_errors = @errors.length
        id = run["id"]
        check(present?(id) && !seen[id], "run id must be present and unique")
        if run.key?("resume")
          validate_resume(run, seen)
        end
        seen[id] = run
        if !synthetic_self_test && run["scenario"] != "INITIAL_OWNER"
          check(run["transport"] == provenance["transport"], "run #{id}: transport must match the captured environment; do not combine channels into one run")
        end
        validate_run(run)
        result = { "id" => id, "scenario" => run["scenario"], "status" => @errors.length == before_errors ? "PASS" : "FAIL" }
        result["result_scope"] = "SELECTED_ENTRIES_ONLY" if ordinary_selected_entries?(run)
        @runtime_cases << result
      end
      @runtime_evidence_loaded = !synthetic_self_test
      @runtime_evidence_valid = @errors.length == initial_errors
      @actual_write_verify_covered = !synthetic_self_test && runs.any? do |run|
        !ordinary_selected_entries?(run) &&
          Array(run["results"]).any? { |row| row["status"] == "VERIFIED_CHANGED" } &&
          Array(run["calls"]).any? { |call| call["kind"] == "WRITE" } &&
          Array(run["calls"]).any? { |call| call["kind"] == "VERIFY" && call["http_success"] == true && call["business_success"] == true }
      end
      self
    end

    def validate_run(run)
      label = "run #{run['id']}"
      if run["scenario"] == "INITIAL_OWNER"
        validate_initial_owner(run, label)
        return
      end
      recipient = run["recipient"] || {}
      check(recipient["id_type"] == "INTERNAL_EMPLOYEE_ID", "#{label}: recipient must use the page contract's internal employee ID")
      check(id?(recipient["employee_id"]) && id?(recipient["tenant_id"]), "#{label}: employee and tenant IDs must be lossless strings")
      check(recipient["department_ids"].is_a?(Array) && recipient["department_ids"].all? { |id| id?(id) }, "#{label}: recipient departments must preserve the observed ordered IDs (empty means none)")
      manifest = run["manifest"] || {}
      entries = indexed(manifest["entries"], "#{label} entries")
      affected = indexed(manifest["affected"], "#{label} affected")
      exclusions = indexed(manifest["exclusions"], "#{label} exclusions")
      check((entries.keys & exclusions.keys).empty?, "#{label}: excluded entries cannot remain in selected entries")
      check((affected.keys & exclusions.keys).empty?, "#{label}: an excluded resource cannot still be affected through its group")
      exclusions.each { |id, row| check(present?(row["reason"]), "#{label}: exclusion #{id} must have an evidenced reason") }
      scope = run["selection"] || {}
      check(scope["frozen"] == true, "#{label}: confirmation must use a frozen selection")
      check(%w[EXPLICIT ALL_RESULTS].include?(scope["mode"]), "#{label}: unknown selection mode")
      if scope["mode"] == "ALL_RESULTS"
        check(scope["pagination_complete"] == true && scope["dynamic_filter_submitted"] == false,
              "#{label}: all results must be fully enumerated, then submitted as IDs, never a dynamic search")
      end
      before = indexed(run["before"], "#{label} before")
      after = indexed(run["after"], "#{label} after")
      validate_ordinary_selected_entries(run, entries, affected, before, after, label) if ordinary_selected_entries?(run)
      calls = Array(run["calls"])
      confirmations = Array(run["confirmations"])
      writes = calls.select { |call| call["kind"] == "WRITE" }
      calls.each do |call|
        recipe = (@contract["recipes"] || {})[call["recipe"]]
        check(recipe.is_a?(Hash), "#{label}: captured call must identify a declared recipe")
        check(%w[READ WRITE VERIFY].include?(call["kind"]), "#{label}: unknown captured call kind")
        if call["kind"] == "VERIFY"
          check(recipe && recipe["role"] == "VERIFY" && recipe.dig("operation", "read_or_write") == "READ", "#{label}: readback must use a declared read-only VERIFY recipe")
        end
      end
      check(calls.all? { |call| call["sequence"].is_a?(Integer) }, "#{label}: calls need integer sequence numbers")
      check(calls.map { |call| call["sequence"] }.uniq.length == calls.length, "#{label}: duplicate call sequence")
      check(calls.map { |call| call["sequence"] }.compact == calls.map { |call| call["sequence"] }.compact.sort, "#{label}: calls must preserve execution order")
      if !writes.empty?
        affected.each do |id, resource|
          prior = before[id]
          if check(prior.is_a?(Hash), "#{label}: #{id} needs a current pre-write snapshot, including uncertain executions")
            check(id?(prior["owner_id"]) && id?(prior["submitter_id"]) && present?(prior["evidence_ref"]), "#{label}: #{id} requires observed current owner/submitter and snapshot provenance")
            check(prior["resource_type"] == resource["resource_type"], "#{label}: pre-write snapshot type must match resource type")
          end
        end
      end
      if run["requested_action"] == "SUBMITTER_ONLY" && affected.values.any? { |row| ordinary_handler?(row) }
        check(writes.empty?, "#{label}: ordinary submitter-only request must not issue handover")
      end
      validate_permissions(run, writes, before, label)
      validate_writes(writes, confirmations, entries, affected, exclusions, recipient, run, label)
      validate_results(run, calls, writes, affected, exclusions, before, after, recipient, label)
      validate_batch_reconciliations(run, calls, writes, label)
      validate_scenario(run, writes, before, recipient, label)
    end

    def ordinary_selected_entries?(run)
      run["scope_mode"] == "ORDINARY_SELECTED_ENTRIES"
    end

    # This test verifies only the named ordinary entries. It neither enumerates
    # nor certifies the server-managed related effects, including draft cleanup.
    def validate_ordinary_selected_entries(run, entries, affected, before, after, label)
      policy = @contract.dig("business", "ordinary_core_test") || {}
      check(policy["enabled"] == true && policy["environment"] == "test" && policy["result_scope"] == "SELECTED_ENTRIES_ONLY",
            "#{label}: ordinary selected-entry testing must be explicitly enabled for test")
      check(run["environment"] == "test", "#{label}: ordinary selected-entry testing is test-only")
      check(Array(run["calls"]).count { |call| call["kind"] == "WRITE" } <= 1,
            "#{label}: ordinary selected-entry core testing permits only one batch")
      check(run.dig("selection", "mode") == "EXPLICIT", "#{label}: ordinary selected-entry testing cannot use ALL_RESULTS")
      check(run["result_scope"] == "SELECTED_ENTRIES_ONLY" && run["all_related_effects_verified"] == false,
            "#{label}: selected-entry results cannot certify all related effects")
      related = run["related_effects"]
      if check(related.is_a?(Hash), "#{label}: server-managed related effects must be disclosed")
        check(related["management"] == "SERVER_MANAGED" && related["disclosed"] == true && present?(related["evidence_ref"]),
              "#{label}: related-effect disclosure needs actual source evidence")
        check(related["strict_no_related_changes"] == false, "#{label}: this mode cannot promise that other records remain unchanged")
      end
      check(!entries.empty? && same_set?(entries.keys, affected.keys), "#{label}: verified resources must equal the explicit entries, not an invented related expansion")
      entries.each do |id, entry|
        check(same_set?(entry["impact_ids"], [id]), "#{label}: each selected entry must identify only itself as the verification scope")
      end
      { "entries" => entries, "affected" => affected, "before" => before, "after" => after }.each do |collection, rows|
        check((rows.keys - entries.keys).empty?, "#{label}: #{collection} exceeds the selected verification scope")
        rows.each do |id, row|
          check(row["resource_type"] == ORDINARY && row["handler"] == ORDINARY && row["parent_assembly_id"].nil?,
                "#{label}: #{collection} #{id} must be an ordinary resource, never an assembly or member")
          check(row.key?("assembly_id") && [nil, "0"].include?(row["assembly_id"]),
                "#{label}: #{collection} #{id} needs observed empty assembly membership (null or string zero)")
        end
      end
      check(Array(run.dig("manifest", "assembly_memberships")).empty?, "#{label}: assembly membership cannot use ordinary selected-entry testing")
    end

    # A run is one automatic execution sequence. An explicitly requested resume
    # is a new run and needs evidence of its new preflight, without replaying an
    # unresolved impact from the previous execution.
    def validate_resume(run, previous_runs)
      label = "run #{run['id']} resume"
      resume = run["resume"] || {}
      previous = previous_runs[resume["previous_run_id"]]
      return unless check(previous.is_a?(Hash), "#{label}: reference an earlier captured run")
      check(present?(resume["user_instruction_ref"]) && present?(resume["preflight_evidence_ref"]), "#{label}: explicit continuation instruction and refreshed preflight evidence are required")
      uncertain = Array(previous["results"]).select { |row| row["status"] == "UNVERIFIED" }.map { |row| row["resource_id"] }
      affected = Array(run.dig("manifest", "affected")).map { |row| row["resource_id"] }
      check((uncertain & affected).empty?, "#{label}: a new run cannot include any still-uncertain prior impact")
      # The normal confirmation checks still bind the receiver and full scope;
      # a continuation instruction can be their source, with no extra prompt.
    end

    def validate_batch_reconciliations(run, calls, writes, label)
      reconciliations = Array(run["batch_reconciliations"])
      final_results = Array(run["results"]).each_with_object({}) { |row, out| out[row["resource_id"]] = row["status"] }
      writes.each_cons(2) do |previous, following|
        check(previous["outcome"] == "CONFIRMED", "#{label}: rejected or uncertain transport outcomes stop automatic later batches, even if later readback resolves them")
        matches = reconciliations.select { |row| row["request_id"] == previous["request_id"] }
        next unless check(matches.length == 1, "#{label}: each automatic later batch requires one captured reconciliation of the prior batch")
        reconciliation = matches.first
        sequence = reconciliation["sequence"]
        check(sequence.is_a?(Integer) && previous["sequence"].is_a?(Integer) && following["sequence"].is_a?(Integer) &&
              previous["sequence"] < sequence && sequence < following["sequence"] && calls.none? { |call| call["sequence"] == sequence },
              "#{label}: prior-batch reconciliation must occur after its write/readbacks and before the next write")
        check(present?(reconciliation["evidence_ref"]), "#{label}: batch reconciliation needs actual captured evidence")
        rows = indexed(reconciliation["results"], "#{label} prior batch reconciliation")
        check(same_set?(rows.keys, Array(previous["impact_ids"])), "#{label}: prior-batch reconciliation must account for every affected resource")
        rows.each do |id, row|
          state = row["status"]
          check(%w[VERIFIED_CHANGED NOT_NEW_SUCCESS FAILED NOT_EXECUTED].include?(state), "#{label}: unresolved/empty/partial prior-batch results stop automatic later writes")
          check(final_results[id] == state, "#{label}: prior-batch resolution must agree with the reported per-resource outcome")
          next unless %w[VERIFIED_CHANGED NOT_NEW_SUCCESS].include?(state)
          verify = calls.find do |call|
            call["kind"] == "VERIFY" && Array(call["resource_ids"]).include?(id) &&
              call["http_success"] == true && call["business_success"] == true &&
              call["sequence"].is_a?(Integer) && sequence.is_a?(Integer) && previous["sequence"].is_a?(Integer) &&
              previous["sequence"] < call["sequence"] && call["sequence"] < sequence
          end
          check(!verify.nil?, "#{label}: changed/unchanged resource #{id} requires readback before reconciliation and the next write")
        end
      end
    end

    def validate_initial_owner(run, label)
      observations = Array(run["observations"])
      check(!observations.empty?, "#{label}: initial-owner observations are required")
      observations.each do |row|
        check(id?(row["resource_id"]) && id?(row["owner_id"]) && id?(row["creator_id"]), "#{label}: initial-owner IDs must be lossless strings")
        check(present?(row["evidence_ref"]), "#{label}: initial-owner observation needs its creation-path evidence")
        if %w[ORDINARY_CREATE DRAFT_INITIALIZATION].include?(row["creation_path"])
          check(row["owner_id"] == row["creator_id"], "#{label}: ordinary initial owner must equal creator")
        elsif %w[HISTORICAL_SNAPSHOT AUTOMATIC_COMPLETION_RECORD].include?(row["creation_path"])
          check(row["owner_id"] == row["source_owner_id"], "#{label}: exceptional path must be checked against source ownership, not creator")
        else
          check(false, "#{label}: unknown initialization path; do not infer owner")
        end
      end
      check(Array(run["calls"]).none? { |call| call["kind"] == "WRITE" }, "#{label}: this observer does not authorize or execute creation")
    end

    def validate_permissions(run, writes, before, label)
      operator = run["operator"] || {}
      check(id?(operator["employee_id"]), "#{label}: current operator ID must be a string")
      %w[personal_handover all_handover].each do |key|
        check([true, false].include?(operator[key]), "#{label}: actual #{key} permission must be observed")
      end
      if operator["personal_handover"] == false && operator["all_handover"] == false
        check(writes.empty?, "#{label}: relationship alone cannot permit a write when both permissions are absent")
      end
      # This checks only a claimed branch, not a client-side copy of backend
      # authorization. Permission never implies that the API will succeed.
      if run["permission_branch"] == "THIRD_PARTY"
        check(operator["all_handover"] == true, "#{label}: a claimed third-party branch requires observed all-handover permission")
        before.values.select { |row| row["resource_type"] == ORDINARY }.each do |row|
          check(operator["employee_id"] != row["owner_id"] && operator["employee_id"] != row["submitter_id"], "#{label}: ordinary third-party branch cannot be claimed by current owner or submitter")
        end
      end
    end

    def validate_writes(writes, confirmations, entries, affected, exclusions, recipient, run, label)
      seen_entries = []
      seen_request_ids = []
      group_batch = {}
      previous_finish = nil
      stop = false
      writes.each_with_index do |call, batch_index|
        check(!stop, "#{label}: a later batch/retry was sent after an uncertain write")
        check(call["recipe"] == @contract.dig("business", "execution_policy", "write_recipe"), "#{label}: unexpected write recipe; no direct data updates or fallback writes")
        check(call["admin_flag_supplied"] == false && call["identity_override_supplied"] == false, "#{label}: Agent must not supply admin or impersonation overrides")
        ids = Array(call["entry_ids"])
        check(!ids.empty? && ids.length <= 100 && ids.uniq.length == ids.length && ids.all? { |id| id?(id) }, "#{label}: each batch needs 1-100 unique lossless entry IDs")
        check((ids - entries.keys).empty?, "#{label}: batch contains an unconfirmed or excluded entry")
        check((ids & seen_entries).empty?, "#{label}: entry was submitted more than once")
        seen_entries.concat(ids)
        request_id = call["request_id"]
        check(present?(request_id) && !seen_request_ids.include?(request_id), "#{label}: writes need distinct captured request IDs")
        seen_request_ids << request_id
        check(call["recipient_employee_id"] == recipient["employee_id"] && call["recipient_tenant_id"] == recipient["tenant_id"], "#{label}: batch recipient drifted from the chosen person/tenant")
        if ordinary_selected_entries?(run)
          check(call["impact_complete"] == false, "#{label}: selected-entry testing must not fabricate complete related impact")
        else
          check(call["impact_complete"] == true, "#{label}: unresolved impact must block the write")
        end
        expected_impacts = ids.flat_map { |id| Array(entries.fetch(id, {})["impact_ids"]) }.uniq
        check(!expected_impacts.empty? && same_set?(call["impact_ids"], expected_impacts), "#{label}: batch resource IDs must equal the confirmed verification scope")
        check((expected_impacts - affected.keys).empty?, "#{label}: group expansion contains unconfirmed affected resources")
        ids.each do |id|
          entry = entries[id] || {}
          group = entry["group_id"]
          next if group.nil?
          check(id?(group), "#{label}: group IDs must be lossless strings")
          check(!group_batch.key?(group) || group_batch[group] == batch_index, "#{label}: one group was split across batches")
          group_batch[group] = batch_index
          all_group_entries = entries.select { |_entry_id, row| row["group_id"] == group }.keys
          check((all_group_entries - ids).empty?, "#{label}: not all frozen entries of this group are in the same batch")
        end
        confirmation = confirmations.find { |row| row["id"] == call["confirmation_id"] }
        if check(confirmation.is_a?(Hash), "#{label}: write needs a concrete matching confirmation")
          check(confirmation["sequence"].is_a?(Integer) && call["sequence"].is_a?(Integer) && confirmation["sequence"] < call["sequence"], "#{label}: confirmation must precede the write")
          check(confirmation["user_confirmed"] == true && present?(confirmation["evidence_ref"]), "#{label}: confirmation needs actual user evidence")
          check(same_set?(confirmation["entry_ids"], entries.keys) && same_set?(confirmation["impact_ids"], affected.keys), "#{label}: confirmed manifest does not equal the frozen scope")
          check(same_set?(confirmation["excluded_ids"], exclusions.keys), "#{label}: exclusions must be disclosed before confirmation")
          check(confirmation["recipient_employee_id"] == recipient["employee_id"], "#{label}: confirmation has a different recipient")
          check(call["confirmed_snapshot_unchanged"] == true, "#{label}: changed scope/impact/roles requires another confirmation")
          if ordinary_selected_entries?(run)
            check(confirmation["related_effects_accepted"] == true, "#{label}: the same concrete confirmation must accept disclosed server-managed related effects")
          end
        end
        started = parse_time(call["started_at"])
        finished = parse_time(call["finished_at"])
        check(started && finished && finished >= started, "#{label}: write needs valid start/finish times")
        check(!previous_finish || (started && started >= previous_finish), "#{label}: batches overlap instead of executing serially")
        previous_finish = finished if finished
        check(%w[CONFIRMED REJECTED UNCERTAIN].include?(call["outcome"]), "#{label}: unexpected synchronous write outcome")
        if call["outcome"] == "CONFIRMED"
          check(call["http_success"] == true && call["business_success"] == true, "#{label}: CONFIRMED transport result needs actual HTTP and business success (still not per-resource success)")
        end
        stop = true if call["outcome"] == "UNCERTAIN"
        check(call["transport"] == run["transport"], "#{label}: one run must not switch channels to replay")
      end
    end

    def validate_results(run, calls, writes, affected, exclusions, before, after, recipient, label)
      results = indexed(run["results"], "#{label} results")
      expected = (affected.keys + exclusions.keys).uniq
      check(same_set?(results.keys, expected), "#{label}: every affected/excluded resource needs exactly one result")
      records = Array(run["records"])
      records.each do |record|
        # These correlation fields are written by the evidence collector. They
        # are NOT claimed to be native Authorization response fields.
        check(present?(record["evidence_ref"]) && present?(record["attribution"]), "#{label}: normalized records need source evidence and attribution; do not invent server status fields")
      end
      results.each do |id, result|
        state = result["status"]
        check(RESULT_STATES.include?(state), "#{label}: unknown per-resource status for #{id}")
        if exclusions.key?(id)
          check(state == "EXCLUDED", "#{label}: disclosed exclusions must remain EXCLUDED, not counted as handed over")
          next
        end
        write = writes.find { |call| Array(call["impact_ids"]).include?(id) }
        proof_resource_id = authorization_resource_id(affected[id], run, label)
        current_records = records.select do |record|
          write && [id, proof_resource_id].include?(record["resource_id"]) && record["request_id"] == write["request_id"] &&
            record["execution_id"] == run["id"] && record["receiver_employee_id"] == recipient["employee_id"]
        end
        if state == "VERIFIED_CHANGED"
          check(write && write["outcome"] != "REJECTED", "#{label}: #{id} cannot succeed from a rejected/unsent request")
          check(current_records.any? do |record|
            record["status"] == "SUCCESS" &&
              (record["attribution"] == "AUTHORIZATION_IN_CURRENT_RESPONSE" ||
               (record["attribution"] == "NEW_AUTHORIZATION_IN_READBACK" && record["absent_from_before"] == true && present?(record["before_evidence_ref"]) && id?(record["authorization_id"])))
          end, "#{label}: #{id} needs this execution's authorization evidence; a historical record/current owner is insufficient")
          verify = calls.find do |call|
            call["kind"] == "VERIFY" && Array(call["resource_ids"]).include?(id) &&
              call["http_success"] == true && call["business_success"] == true && present?(call["request_id"])
          end
          check(verify && write && verify["sequence"].to_i > write["sequence"].to_i, "#{label}: #{id} needs successful post-write readback")
          prior = before[id]
          final = after[id]
          if check(prior.is_a?(Hash) && final.is_a?(Hash), "#{label}: #{id} needs actual before/after snapshots")
            check(id?(prior["owner_id"]) && id?(prior["submitter_id"]) && id?(final["owner_id"]) && id?(final["submitter_id"]), "#{label}: #{id} role IDs must remain lossless strings")
            check(present?(final["evidence_ref"]), "#{label}: #{id} after snapshot requires readback provenance")
            if ordinary_handler?(affected[id] || {})
              check(prior["resource_type"] == ORDINARY && final["resource_type"] == ORDINARY, "#{label}: snapshot resource type disagrees with ordinary processor")
              check(prior["owner_id"] != recipient["employee_id"], "#{label}: preexisting target owner is not a new handover success")
              check(final["owner_id"] == recipient["employee_id"], "#{label}: ordinary owner did not become recipient")
              expected_submitter = prior["owner_id"] == prior["submitter_id"] ? recipient["employee_id"] : prior["submitter_id"]
              check(final["submitter_id"] == expected_submitter, "#{label}: ordinary submitter change violates the equal/different rule")
              unless ordinary_selected_entries?(run)
                check(same_set?(prior["demand_employee_ids"], final["demand_employee_ids"]), "#{label}: ordinary demand-person list must be read and unchanged")
                validate_department(prior, final, recipient, true, label, id)
              end
            elsif assembly_handler?(affected[id] || {})
              validate_assembly_transition(id, affected[id], before, after, recipient, run, label)
            else
              check(false, "#{label}: #{id} uses an unknown handler; never apply ordinary role rules by default")
            end
          end
        elsif state == "FAILED"
          check(current_records.any? { |record| record["status"] == state && present?(record["reason"]) }, "#{label}: #{id} #{state} requires a current, explicit reason record")
        elsif state == "NOT_EXECUTED"
          check(write.nil? || current_records.any? { |record| %w[NOT_EXECUTED SKIPPED].include?(record["status"]) && present?(record["reason"]) }, "#{label}: #{id} cannot be labelled not executed merely because its record is missing")
        elsif state == "NOT_NEW_SUCCESS"
          prior, final = before[id], after[id]
          check(prior.is_a?(Hash) && prior["owner_id"] == recipient["employee_id"], "#{label}: NOT_NEW_SUCCESS needs observed preexisting target ownership")
          if write
            unchanged_fields = %w[owner_id submitter_id]
            unchanged_fields << "department_id" unless assembly_child?(affected[id] || {}) || ordinary_selected_entries?(run)
            check(final.is_a?(Hash) && prior.is_a?(Hash) && unchanged_fields.all? { |field| final[field] == prior[field] }, "#{label}: sent request needs unchanged observed fields before reporting no new change")
          end
          check(current_records.none? { |record| record["status"] == "SUCCESS" }, "#{label}: do not discard an evidenced new change as a preexisting state")
        elsif state == "EXCLUDED"
          check(false, "#{label}: EXCLUDED is valid only for a pre-disclosed exclusion")
        end
      end
      success_count = results.values.count { |row| row["status"] == "VERIFIED_CHANGED" }
      check(run["reported_success_count"] == success_count, "#{label}: reported success count must count confirmed resources, not selected entries")
      if run["reported_all_success"] == true
        check(!results.empty? && results.values.all? { |row| row["status"] == "VERIFIED_CHANGED" }, "#{label}: mixed/unknown/unexecuted results cannot be reported as all successful")
      end
    end

    def ordinary_handler?(resource)
      resource["resource_type"] == ORDINARY && resource["parent_assembly_id"].nil? &&
        [nil, "ORDINARY_CONTRACT"].include?(resource["handler"])
    end

    def assembly_handler?(resource)
      resource["resource_type"] == "CONTRACT_ASSEMBLY" || resource["handler"] == "CONTRACT_ASSEMBLY"
    end

    def assembly_child?(resource)
      resource["resource_type"] == ORDINARY && resource["handler"] == "CONTRACT_ASSEMBLY" && id?(resource["parent_assembly_id"])
    end

    def authorization_resource_id(resource, run, label)
      return nil unless resource.is_a?(Hash)
      parent_id = resource["parent_assembly_id"]
      return resource["resource_id"] if parent_id.nil?
      membership = Array(run.dig("manifest", "assembly_memberships")).find { |row| row["assembly_id"] == parent_id }
      if check(membership.is_a?(Hash), "#{label}: assembly child requires captured parent/member relationship")
        members = Array(membership["member_resource_ids"])
        check(membership["complete"] == true && present?(membership["evidence_ref"]), "#{label}: parent authorization can cover a child only with complete membership evidence")
        check(membership["expected_member_count"].is_a?(Integer) && membership["expected_member_count"] == members.uniq.length && members.uniq.length == members.length, "#{label}: assembly member enumeration must match the observed child count")
        check(members.include?(resource["resource_id"]), "#{label}: child is not in the observed assembly membership")
        declared_children = Array(run.dig("manifest", "affected")).select { |row| row["parent_assembly_id"] == parent_id }.map { |row| row["resource_id"] }
        check(same_set?(declared_children, members), "#{label}: all observed assembly children must be included in the impact manifest")
      end
      parent_id
    end

    def validate_assembly_transition(id, resource, before, after, recipient, run, label)
      parent_id = resource["parent_assembly_id"] || id
      parent = before[parent_id]
      return unless check(parent.is_a?(Hash) && parent["resource_type"] == "CONTRACT_ASSEMBLY", "#{label}: assembly rule needs its actual parent pre-state")
      operator = run["operator"] || {}
      actor = operator["employee_id"]
      third_party = operator["all_handover"] == true && actor != parent["owner_id"] && actor != parent["submitter_id"]
      actor = parent["owner_id"] if third_party
      # A compact expected-transition oracle, not an executor or permission
      # engine: parent role flags govern BOTH parent and children independently.
      owner_changes = actor == parent["owner_id"] && recipient["employee_id"] != parent["owner_id"]
      submitter_changes = actor == parent["submitter_id"] && recipient["employee_id"] != parent["submitter_id"]
      check(owner_changes || submitter_changes, "#{label}: assembly SUCCESS requires at least one parent role to change")
      prior, final = before[id], after[id]
      check(final["owner_id"] == (owner_changes ? recipient["employee_id"] : prior["owner_id"]), "#{label}: assembly owner transition must follow the parent's ownerHandover flag")
      check(final["submitter_id"] == (submitter_changes ? recipient["employee_id"] : prior["submitter_id"]), "#{label}: assembly submitter transition must follow the parent's submitterHandover flag")
      parent_resource = resource["resource_type"] == "CONTRACT_ASSEMBLY"
      # Child departments are outside this Skill's acceptance criteria. The
      # parent's department check and all child role/member checks still apply.
      validate_department(prior, final, recipient, true, label, id) if parent_resource
      unless parent_resource
        check(same_set?(prior["demand_employee_ids"], final["demand_employee_ids"]), "#{label}: assembly child demand-person list must be read and unchanged")
      end
    end

    def validate_department(prior, final, recipient, update_department, label, id)
      check(prior.key?("department_id") && final.key?("department_id"), "#{label}: #{id} department before/after must be observed (null is permitted)")
      departments = Array(recipient["department_ids"])
      expected = update_department && !departments.empty? ? departments.first : prior["department_id"]
      check(final["department_id"] == expected, "#{label}: #{id} department transition does not match the relevant handler and ordered recipient departments")
    end

    def validate_scenario(run, writes, before, recipient, label)
      ordinary = before.values.select { |row| row["resource_type"] == ORDINARY }
      case run["scenario"]
      when "CURRENT_OWNER_DIFFERS_CREATOR"
        check(ordinary.any? { |row| id?(row["creator_id"]) && row["creator_id"] != row["owner_id"] }, "#{label}: scenario needs creator != current owner")
      when "ORDINARY_EQUAL_ROLES"
        check(ordinary.any? { |row| row["owner_id"] == row["submitter_id"] }, "#{label}: equal-role scenario has no matching input")
      when "ORDINARY_DIFFERENT_ROLES"
        check(ordinary.any? { |row| row["owner_id"] != row["submitter_id"] }, "#{label}: different-role scenario has no matching input")
      when "SUBMITTER_ONLY"
        check(run["requested_action"] == "SUBMITTER_ONLY" && writes.empty?, "#{label}: submitter-only scenario must not issue handover")
      when "PERMISSION_DENIED"
        operator = run["operator"] || {}
        check(operator["personal_handover"] == false && operator["all_handover"] == false && writes.empty?, "#{label}: permission-denied scenario must observe both absent and zero writes")
      when "SERVER_REJECT_AFTER_PRECHECK"
        check(run["precheck_passed"] == true && writes.any? { |call| call["outcome"] == "REJECTED" }, "#{label}: scenario needs actual final server rejection after precheck")
      when "TIMEOUT_STOP"
        check(writes.any? { |call| call["outcome"] == "UNCERTAIN" }, "#{label}: timeout scenario needs an uncertain sent request")
      when "PREEXISTING_TARGET_OWNER"
        check(ordinary.any? { |row| row["owner_id"] == recipient["employee_id"] }, "#{label}: scenario needs preexisting target ownership")
      when "ASSEMBLY_OWNER_ONLY", "ASSEMBLY_SUBMITTER_ONLY", "ASSEMBLY_BOTH_ROLES"
        parent = before.values.find { |row| row["resource_type"] == "CONTRACT_ASSEMBLY" }
        if check(parent.is_a?(Hash), "#{label}: assembly scenario requires parent pre-state")
          actor = run.dig("operator", "employee_id")
          if run.dig("operator", "all_handover") == true && actor != parent["owner_id"] && actor != parent["submitter_id"]
            actor = parent["owner_id"]
          end
          flags = [actor == parent["owner_id"] && parent["owner_id"] != recipient["employee_id"], actor == parent["submitter_id"] && parent["submitter_id"] != recipient["employee_id"]]
          expected = { "ASSEMBLY_OWNER_ONLY" => [true, false], "ASSEMBLY_SUBMITTER_ONLY" => [false, true], "ASSEMBLY_BOTH_ROLES" => [true, true] }[run["scenario"]]
          check(flags == expected, "#{label}: assembly scenario does not exercise its claimed parent flags")
        end
      when "MIXED_RESULTS", "EMPTY_RECORDS", "HISTORICAL_RECORD", "FROZEN_ALL_RESULTS", "SERIAL_GROUP_BATCHES", "THIRD_PARTY"
        validate_additional_scenario(run, writes, label)
      else
        check(false, "#{label}: unknown scenario; do not claim coverage from an unclassified run")
      end
    end

    def validate_additional_scenario(run, writes, label)
      results = Array(run["results"])
      case run["scenario"]
      when "MIXED_RESULTS"
        check(results.map { |row| row["status"] }.uniq.length > 1, "#{label}: mixed scenario needs different resource outcomes")
      when "EMPTY_RECORDS"
        check(Array(run["records"]).empty? && results.none? { |row| row["status"] == "VERIFIED_CHANGED" }, "#{label}: empty-record scenario cannot invent completed writes")
      when "HISTORICAL_RECORD"
        check(Array(run["records"]).any? { |record| record["execution_id"] != run["id"] }, "#{label}: historical-record scenario needs a different execution")
      when "FROZEN_ALL_RESULTS"
        check(run.dig("selection", "mode") == "ALL_RESULTS", "#{label}: all-results scenario needs that selection mode")
      when "SERIAL_GROUP_BATCHES"
        check(writes.length > 1, "#{label}: serial batching scenario needs more than one write")
      when "THIRD_PARTY"
        check(run["permission_branch"] == "THIRD_PARTY", "#{label}: third-party scenario must identify the observed branch")
      end
    end

    def indexed(value, label)
      return {} unless check(value.is_a?(Array), "#{label} must be an array")
      output = {}
      value.each do |row|
        unless row.is_a?(Hash)
          check(false, "#{label} rows must be objects")
          next
        end
        id = row["resource_id"]
        check(id?(id) && !output.key?(id), "#{label}: resource IDs must be unique lossless strings")
        output[id] = row
      end
      output
    end

    def reject_sensitive_keys(value, path = "evidence")
      if value.is_a?(Hash)
        value.each do |key, item|
          check(!FORBIDDEN_KEYS.include?(key.to_s.downcase), "#{path}: credential/header field is forbidden (value not displayed)")
          reject_sensitive_keys(item, "#{path}.#{key}")
        end
      elsif value.is_a?(Array)
        value.each { |item| reject_sensitive_keys(item, path) }
      end
    end

    def present?(value)
      !value.nil? && !value.to_s.strip.empty?
    end

    def id?(value)
      value.is_a?(String) && !value.empty?
    end

    def same_set?(left, right)
      left.is_a?(Array) && right.is_a?(Array) && left.uniq.length == left.length && right.uniq.length == right.length && left.sort == right.sort
    rescue ArgumentError
      false
    end

    def parse_time(value)
      value.is_a?(String) ? Time.iso8601(value) : nil
    rescue ArgumentError
      nil
    end

    def report
      runtime_status = if !@runtime_evidence_loaded
                         "NOT_RUN"
                       elsif @runtime_evidence_valid
                         "PROVIDED_CASES_PASS"
                       else
                         "FAIL"
                       end
      if @contract.dig("readiness", "delivery_status") == "RUNNABLE" && !@runtime_evidence_loaded
        @errors << "RUNNABLE requires --evidence with actual handover/readback records; static flags alone do not prove execution" unless @errors.include?("RUNNABLE requires --evidence with actual handover/readback records; static flags alone do not prove execution")
      end
      if @contract.dig("readiness", "delivery_status") == "RUNNABLE" && @runtime_evidence_loaded && !@actual_write_verify_covered
        @errors << "RUNNABLE requires a captured, confirmed change with full-scope write and post-write verification; selected-entry, initialization or negative-only cases are insufficient"
      end
      {
        "validator" => "contract-handover-business-v1",
        "contract_status" => @errors.empty? ? (@gaps.empty? ? "PASS" : "CONTRACT_INCOMPLETE") : "FAIL",
        "checks" => @checks,
        "business_runtime" => { "status" => runtime_status, "cases" => @runtime_cases,
                                "coverage_note" => "Only supplied captured cases are checked. Omitted scenarios, handlers, agents and transports remain NOT_RUN. This validator cannot independently authenticate a capture's provenance." },
        "errors" => @errors,
        "gaps" => @gaps,
        "side_effects" => "NONE: no network, no CLI business command, no credential loading"
      }
    end
  end

  def self.main(argv)
    options = { contract: File.expand_path("../references/api-contract.yaml", __dir__) }
    parser = OptionParser.new do |opts|
      opts.banner = "Usage: ruby tests/validate_contract.rb [--contract FILE] [--evidence FILE] [--self-test]"
      opts.on("--contract FILE", "Validate this v2 API contract") { |value| options[:contract] = value }
      opts.on("--evidence FILE", "Validate redacted LIVE_CAPTURE JSON; never performs an API request") { |value| options[:evidence] = value }
      opts.on("--self-test", "Synthetic verifier regression only, NOT business acceptance") { options[:self_test] = true }
      opts.on("-h", "--help") { puts opts; return 0 }
    end
    parser.parse!(argv)
    if options[:self_test]
      require_relative "self_test"
      return ContractHandoverValidation::SelfTest.run
    end
    contract = YAML.safe_load(File.read(options[:contract], encoding: "UTF-8"), aliases: false)
    validator = Validator.new(contract).validate_contract
    if options[:evidence]
      evidence = JSON.parse(File.read(options[:evidence], encoding: "UTF-8"))
      validator.validate_evidence(evidence)
    end
    result = validator.report
    puts JSON.pretty_generate(result)
    return EXIT_INVALID unless result["errors"].empty?
    result["gaps"].empty? ? 0 : EXIT_INCOMPLETE
  rescue OptionParser::ParseError, Psych::Exception, JSON::ParserError, Errno::ENOENT, TypeError, NoMethodError => e
    # Never print input file contents or credential values in a parser failure.
    puts JSON.pretty_generate({ "validator" => "contract-handover-business-v1", "contract_status" => "FAIL", "business_runtime" => { "status" => "NOT_RUN" }, "errors" => ["Cannot validate input (#{e.class}); use --help and the evidence format documentation."] })
    EXIT_INVALID
  end
end

exit ContractHandoverValidation.main(ARGV) if $PROGRAM_NAME == __FILE__
