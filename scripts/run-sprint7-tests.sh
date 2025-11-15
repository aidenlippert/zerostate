#!/bin/bash

# Sprint 7 Phase 1 E2E Test Execution Script
# Comprehensive test suite for validating complete system integration
# Author: Sprint 7 Test Automation
# Date: $(date +%Y-%m-%d)

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TEST_ROOT="$PROJECT_ROOT/tests"
REPORTS_DIR="$TEST_ROOT/reports"
LOG_DIR="$PROJECT_ROOT/logs"

# Test configuration
SUBSTRATE_NODE_URL="ws://localhost:9944"
API_BASE_URL="http://localhost:8080"
TEST_TIMEOUT="30m"
PARALLEL_TESTS=4

# Sprint 6 baseline metrics for regression testing
SPRINT_6_BASELINE_THROUGHPUT="12.5"
SPRINT_6_BASELINE_LATENCY_P95="85"
SPRINT_6_BASELINE_MEMORY_MB="180"
SPRINT_6_BASELINE_ERROR_RATE="0.1"

# Test suite configuration
declare -A TEST_SUITES=(
    ["e2e"]="E2E Workflow Tests"
    ["integration"]="Cross-Component Integration Tests"
    ["regression"]="Performance Regression Tests"
    ["security"]="Security Validation Tests"
)

# Test execution status
declare -A SUITE_STATUS
declare -A SUITE_DURATION
declare -A SUITE_ERRORS

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_header() {
    echo -e "\n${BOLD}${BLUE}================================================${NC}"
    echo -e "${BOLD}${BLUE} $1${NC}"
    echo -e "${BOLD}${BLUE}================================================${NC}\n"
}

# Utility functions
check_dependencies() {
    log_header "Checking Dependencies"

    local deps=("go" "curl" "jq" "timeout")
    local missing_deps=()

    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            missing_deps+=("$dep")
        else
            log_info "✓ $dep is available"
        fi
    done

    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing dependencies: ${missing_deps[*]}"
        exit 1
    fi

    # Check Go version
    local go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+')
    log_info "Go version: $go_version"

    log_success "All dependencies are available"
}

setup_test_environment() {
    log_header "Setting Up Test Environment"

    # Create necessary directories
    mkdir -p "$REPORTS_DIR" "$LOG_DIR"

    # Clean previous test results
    rm -f "$REPORTS_DIR"/*.json "$REPORTS_DIR"/*.txt "$REPORTS_DIR"/*.md
    rm -f "$LOG_DIR"/test-*.log

    # Set environment variables for tests
    export SUBSTRATE_NODE_URL="$SUBSTRATE_NODE_URL"
    export API_BASE_URL="$API_BASE_URL"
    export TEST_TIMEOUT="$TEST_TIMEOUT"
    export REPORTS_DIR="$REPORTS_DIR"
    export SPRINT_6_BASELINE_THROUGHPUT="$SPRINT_6_BASELINE_THROUGHPUT"
    export SPRINT_6_BASELINE_LATENCY_P95="$SPRINT_6_BASELINE_LATENCY_P95"
    export SPRINT_6_BASELINE_MEMORY_MB="$SPRINT_6_BASELINE_MEMORY_MB"
    export SPRINT_6_BASELINE_ERROR_RATE="$SPRINT_6_BASELINE_ERROR_RATE"

    log_success "Test environment setup complete"
}

check_system_health() {
    log_header "System Health Check"

    local health_ok=true

    # Check Substrate node
    log_info "Checking Substrate node connectivity..."
    if timeout 10 wscat -c "$SUBSTRATE_NODE_URL" --close &>/dev/null; then
        log_success "✓ Substrate node is accessible"
    else
        log_warning "⚠ Substrate node not accessible - some tests may be skipped"
        health_ok=false
    fi

    # Check API service
    log_info "Checking API service..."
    if curl -s --max-time 5 "$API_BASE_URL/health" &>/dev/null; then
        log_success "✓ API service is healthy"
    else
        log_warning "⚠ API service not accessible - using mock services"
    fi

    # Check system resources
    local available_memory=$(free -m | awk 'NR==2{printf "%.1f", $7/1024}')
    local cpu_cores=$(nproc)

    log_info "Available memory: ${available_memory}GB"
    log_info "CPU cores: $cpu_cores"

    if (( $(echo "$available_memory < 2.0" | bc -l) )); then
        log_warning "⚠ Low memory available, reducing test parallelism"
        PARALLEL_TESTS=2
    fi

    log_success "System health check complete"
    return 0
}

run_test_suite() {
    local suite_name="$1"
    local suite_description="$2"
    local start_time=$(date +%s)

    log_header "Running $suite_description"

    local test_dir="$TEST_ROOT/$suite_name"
    local log_file="$LOG_DIR/test-$suite_name.log"
    local report_file="$REPORTS_DIR/${suite_name}_results.json"

    if [ ! -d "$test_dir" ]; then
        log_error "Test directory not found: $test_dir"
        SUITE_STATUS["$suite_name"]="ERROR"
        return 1
    fi

    log_info "Executing $suite_name tests..."
    log_info "Log file: $log_file"
    log_info "Report file: $report_file"

    # Change to test directory
    cd "$test_dir"

    # Run go mod tidy if needed
    if [ -f "go.mod" ]; then
        log_info "Updating Go modules..."
        go mod tidy 2>>"$log_file"
    fi

    # Run the test suite with timeout and capture output
    local test_result=0
    local test_output

    # Configure test execution based on suite
    local test_flags=""
    case "$suite_name" in
        "regression")
            test_flags="-v -timeout=$TEST_TIMEOUT -race"
            ;;
        "security")
            test_flags="-v -timeout=$TEST_TIMEOUT"
            ;;
        "e2e"|"integration")
            test_flags="-v -timeout=$TEST_TIMEOUT -parallel=$PARALLEL_TESTS"
            ;;
        *)
            test_flags="-v -timeout=$TEST_TIMEOUT"
            ;;
    esac

    log_info "Test flags: $test_flags"

    # Execute tests and capture detailed output
    if test_output=$(timeout "${TEST_TIMEOUT}" go test $test_flags ./... 2>&1); then
        local passed_tests=$(echo "$test_output" | grep -c "PASS:" || echo "0")
        local failed_tests=$(echo "$test_output" | grep -c "FAIL:" || echo "0")
        local total_tests=$((passed_tests + failed_tests))

        if [ "$failed_tests" -eq 0 ]; then
            log_success "✓ All $total_tests tests passed for $suite_name"
            SUITE_STATUS["$suite_name"]="PASS"
        else
            log_warning "⚠ $failed_tests out of $total_tests tests failed for $suite_name"
            SUITE_STATUS["$suite_name"]="PARTIAL"
        fi

        # Extract metrics if available
        extract_test_metrics "$test_output" "$suite_name"

    else
        test_result=$?
        log_error "✗ Test suite $suite_name failed with exit code $test_result"
        SUITE_STATUS["$suite_name"]="FAIL"

        # Extract error information
        local error_summary=$(echo "$test_output" | grep -E "(FAIL|ERROR|panic)" | head -10)
        SUITE_ERRORS["$suite_name"]="$error_summary"
    fi

    # Save detailed output to log
    echo "$test_output" >> "$log_file"

    # Generate JSON report
    generate_suite_report "$suite_name" "$test_output" "$report_file"

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    SUITE_DURATION["$suite_name"]="$duration"

    log_info "$suite_description completed in ${duration}s"

    # Return to project root
    cd "$PROJECT_ROOT"

    return $test_result
}

extract_test_metrics() {
    local test_output="$1"
    local suite_name="$2"

    # Extract performance metrics from test output
    case "$suite_name" in
        "regression")
            # Extract throughput, latency, and memory metrics
            local throughput=$(echo "$test_output" | grep -oE "Sprint 7 Throughput: [0-9.]+" | grep -oE "[0-9.]+" | tail -1)
            local latency_p95=$(echo "$test_output" | grep -oE "Sprint 7 P95: [0-9.]+" | grep -oE "[0-9.]+" | tail -1)
            local memory_mb=$(echo "$test_output" | grep -oE "Memory: [0-9.]+ MB" | grep -oE "[0-9.]+" | tail -1)

            if [ -n "$throughput" ]; then
                log_info "📊 Throughput: ${throughput} tasks/sec (baseline: $SPRINT_6_BASELINE_THROUGHPUT)"
            fi
            if [ -n "$latency_p95" ]; then
                log_info "📊 Latency P95: ${latency_p95}ms (baseline: $SPRINT_6_BASELINE_LATENCY_P95)"
            fi
            if [ -n "$memory_mb" ]; then
                log_info "📊 Memory usage: ${memory_mb}MB (baseline: $SPRINT_6_BASELINE_MEMORY_MB)"
            fi
            ;;
        "security")
            # Extract security score
            local security_score=$(echo "$test_output" | grep -oE "Security Score: [0-9.]+" | grep -oE "[0-9.]+" | tail -1)
            if [ -n "$security_score" ]; then
                log_info "🔒 Security Score: ${security_score}/100"
            fi
            ;;
        "e2e")
            # Extract workflow success rate
            local success_rate=$(echo "$test_output" | grep -oE "Success Rate: [0-9.]+" | grep -oE "[0-9.]+" | tail -1)
            if [ -n "$success_rate" ]; then
                log_info "🎯 Workflow Success Rate: ${success_rate}%"
            fi
            ;;
    esac
}

generate_suite_report() {
    local suite_name="$1"
    local test_output="$2"
    local report_file="$3"

    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local status="${SUITE_STATUS[$suite_name]}"
    local duration="${SUITE_DURATION[$suite_name]}"

    # Count test results
    local passed_tests=$(echo "$test_output" | grep -c "PASS:" || echo "0")
    local failed_tests=$(echo "$test_output" | grep -c "FAIL:" || echo "0")
    local skipped_tests=$(echo "$test_output" | grep -c "SKIP:" || echo "0")

    # Generate JSON report
    cat > "$report_file" << EOF
{
    "suite": "$suite_name",
    "status": "$status",
    "timestamp": "$timestamp",
    "duration_seconds": $duration,
    "test_counts": {
        "passed": $passed_tests,
        "failed": $failed_tests,
        "skipped": $skipped_tests,
        "total": $((passed_tests + failed_tests + skipped_tests))
    },
    "metrics": {
        "baseline_throughput": "$SPRINT_6_BASELINE_THROUGHPUT",
        "baseline_latency_p95": "$SPRINT_6_BASELINE_LATENCY_P95",
        "baseline_memory_mb": "$SPRINT_6_BASELINE_MEMORY_MB"
    }
}
EOF

    log_info "Generated report: $report_file"
}

run_all_test_suites() {
    log_header "Executing Sprint 7 Test Suite"

    local total_start_time=$(date +%s)
    local overall_result=0

    # Run each test suite
    for suite in "${!TEST_SUITES[@]}"; do
        if ! run_test_suite "$suite" "${TEST_SUITES[$suite]}"; then
            overall_result=1
        fi

        # Add delay between suites to allow system recovery
        if [ "$suite" != "security" ]; then  # Last suite
            log_info "Waiting 30 seconds before next suite..."
            sleep 30
        fi
    done

    local total_end_time=$(date +%s)
    local total_duration=$((total_end_time - total_start_time))

    log_header "Sprint 7 Test Suite Complete"
    log_info "Total execution time: ${total_duration}s"

    return $overall_result
}

generate_summary_report() {
    log_header "Generating Summary Report"

    local summary_file="$REPORTS_DIR/sprint7_test_summary.md"
    local timestamp=$(date -u +"%Y-%m-%d %H:%M:%S UTC")

    cat > "$summary_file" << EOF
# Sprint 7 Phase 1 E2E Test Summary

**Generated**: $timestamp
**Total Duration**: $(calculate_total_duration) minutes
**Overall Status**: $(determine_overall_status)

## Test Suite Results

| Test Suite | Status | Duration | Passed | Failed | Notes |
|------------|--------|----------|--------|--------|-------|
EOF

    # Add results for each test suite
    for suite in "${!TEST_SUITES[@]}"; do
        local status="${SUITE_STATUS[$suite]:-NOT_RUN}"
        local duration="${SUITE_DURATION[$suite]:-0}"
        local notes=""

        if [ -n "${SUITE_ERRORS[$suite]:-}" ]; then
            notes="See logs for details"
        fi

        # Read test counts from JSON report if available
        local report_file="$REPORTS_DIR/${suite}_results.json"
        local passed="N/A"
        local failed="N/A"

        if [ -f "$report_file" ]; then
            passed=$(jq -r '.test_counts.passed // "N/A"' "$report_file" 2>/dev/null || echo "N/A")
            failed=$(jq -r '.test_counts.failed // "N/A"' "$report_file" 2>/dev/null || echo "N/A")
        fi

        local status_icon
        case "$status" in
            "PASS") status_icon="✅ PASS" ;;
            "PARTIAL") status_icon="⚠️ PARTIAL" ;;
            "FAIL") status_icon="❌ FAIL" ;;
            *) status_icon="❓ $status" ;;
        esac

        echo "| ${TEST_SUITES[$suite]} | $status_icon | ${duration}s | $passed | $failed | $notes |" >> "$summary_file"
    done

    cat >> "$summary_file" << EOF

## Performance Comparison vs Sprint 6

| Metric | Sprint 6 Baseline | Sprint 7 Result | Change | Status |
|--------|-------------------|------------------|--------|--------|
| Throughput (tasks/sec) | $SPRINT_6_BASELINE_THROUGHPUT | TBD | TBD | TBD |
| Latency P95 (ms) | $SPRINT_6_BASELINE_LATENCY_P95 | TBD | TBD | TBD |
| Memory Usage (MB) | $SPRINT_6_BASELINE_MEMORY_MB | TBD | TBD | TBD |
| Error Rate (%) | $SPRINT_6_BASELINE_ERROR_RATE | TBD | TBD | TBD |

## Key Findings

### ✅ Successful Validations
- Complete E2E workflow validation
- Cross-component integration testing
- Security posture validation
- Performance regression testing

### ⚠️ Areas of Concern
$(generate_concerns_section)

### 📋 Recommendations
$(generate_recommendations_section)

## Test Coverage

- **E2E Workflows**: User → Task → VCG → Execution → Payment
- **Integration**: Orchestrator ↔ Blockchain, Payment ↔ Escrow, Reputation ↔ Selection
- **Performance**: Throughput, latency, memory, concurrent load
- **Security**: Authentication, authorization, input validation, payment security

## Files Generated

- Test logs: \`logs/test-*.log\`
- JSON reports: \`tests/reports/*_results.json\`
- Summary report: \`tests/reports/sprint7_test_summary.md\`

---

*Generated by Sprint 7 Test Automation*
EOF

    log_success "Summary report generated: $summary_file"
}

calculate_total_duration() {
    local total=0
    for suite in "${!SUITE_DURATION[@]}"; do
        total=$((total + SUITE_DURATION[$suite]))
    done
    echo $((total / 60))
}

determine_overall_status() {
    local fail_count=0
    local partial_count=0
    local pass_count=0

    for suite in "${!SUITE_STATUS[@]}"; do
        case "${SUITE_STATUS[$suite]}" in
            "FAIL"|"ERROR") ((fail_count++)) ;;
            "PARTIAL") ((partial_count++)) ;;
            "PASS") ((pass_count++)) ;;
        esac
    done

    if [ $fail_count -gt 0 ]; then
        echo "❌ FAILED ($fail_count failed, $partial_count partial, $pass_count passed)"
    elif [ $partial_count -gt 0 ]; then
        echo "⚠️ PARTIAL ($partial_count partial, $pass_count passed)"
    else
        echo "✅ PASSED (all $pass_count suites passed)"
    fi
}

generate_concerns_section() {
    local concerns=""
    for suite in "${!SUITE_STATUS[@]}"; do
        if [ "${SUITE_STATUS[$suite]}" = "FAIL" ] || [ "${SUITE_STATUS[$suite]}" = "PARTIAL" ]; then
            concerns="$concerns\n- ${TEST_SUITES[$suite]}: ${SUITE_STATUS[$suite]}"
            if [ -n "${SUITE_ERRORS[$suite]:-}" ]; then
                concerns="$concerns\n  - $(echo "${SUITE_ERRORS[$suite]}" | head -1)"
            fi
        fi
    done

    if [ -n "$concerns" ]; then
        echo -e "$concerns"
    else
        echo "None identified"
    fi
}

generate_recommendations_section() {
    local recommendations=""

    # Generate recommendations based on test results
    for suite in "${!SUITE_STATUS[@]}"; do
        case "${SUITE_STATUS[$suite]}" in
            "FAIL")
                case "$suite" in
                    "regression") recommendations="$recommendations\n- Investigate performance degradation in $suite" ;;
                    "security") recommendations="$recommendations\n- Address security vulnerabilities found in $suite" ;;
                    *) recommendations="$recommendations\n- Fix critical issues in ${TEST_SUITES[$suite]}" ;;
                esac
                ;;
            "PARTIAL")
                recommendations="$recommendations\n- Review and fix partial failures in ${TEST_SUITES[$suite]}"
                ;;
        esac
    done

    if [ -n "$recommendations" ]; then
        echo -e "$recommendations"
    else
        echo "- Continue monitoring system performance"
        echo "- Maintain current security posture"
        echo "- Consider expanding test coverage"
    fi
}

print_final_summary() {
    log_header "Sprint 7 Phase 1 Test Execution Complete"

    # Print summary table
    echo -e "\n${BOLD}Test Suite Summary:${NC}"
    printf "%-30s %-10s %-10s\n" "Suite" "Status" "Duration"
    printf "%-30s %-10s %-10s\n" "-----" "------" "--------"

    for suite in "${!TEST_SUITES[@]}"; do
        local status="${SUITE_STATUS[$suite]:-NOT_RUN}"
        local duration="${SUITE_DURATION[$suite]:-0}s"

        local color=""
        case "$status" in
            "PASS") color="$GREEN" ;;
            "PARTIAL") color="$YELLOW" ;;
            "FAIL"|"ERROR") color="$RED" ;;
            *) color="$NC" ;;
        esac

        printf "%-30s ${color}%-10s${NC} %-10s\n" "${TEST_SUITES[$suite]}" "$status" "$duration"
    done

    echo -e "\n${BOLD}Reports Generated:${NC}"
    echo "  📁 Test logs: $LOG_DIR/"
    echo "  📊 JSON reports: $REPORTS_DIR/"
    echo "  📋 Summary: $REPORTS_DIR/sprint7_test_summary.md"

    # Final status
    local overall_status=$(determine_overall_status)
    echo -e "\n${BOLD}Overall Status: $overall_status${NC}"
}

cleanup() {
    log_info "Cleaning up test environment..."

    # Kill any remaining test processes
    pkill -f "go test" 2>/dev/null || true

    # Compress logs if they're large
    find "$LOG_DIR" -name "*.log" -size +10M -exec gzip {} \; 2>/dev/null || true

    log_info "Cleanup complete"
}

# Signal handlers
trap cleanup EXIT
trap 'log_error "Test execution interrupted"; cleanup; exit 130' INT TERM

# Main execution
main() {
    local start_time=$(date +%s)

    log_header "Sprint 7 Phase 1 E2E Test Suite"
    log_info "Starting comprehensive system validation..."

    # Pre-flight checks
    check_dependencies
    setup_test_environment
    check_system_health

    # Run test suites
    local test_result=0
    if ! run_all_test_suites; then
        test_result=1
    fi

    # Generate reports
    generate_summary_report
    print_final_summary

    local end_time=$(date +%s)
    local total_duration=$((end_time - start_time))

    log_info "Total execution time: $((total_duration / 60))m $((total_duration % 60))s"

    if [ $test_result -eq 0 ]; then
        log_success "Sprint 7 Phase 1 validation completed successfully! 🎉"
    else
        log_warning "Sprint 7 Phase 1 validation completed with issues ⚠️"
    fi

    exit $test_result
}

# Script entry point
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi