# iterate Improvement Roadmap

## Executive Summary
Based on deep analysis of leading autonomous coding agents (Devin, AutoGPT, Aider, MetaGPT), iterate has excellent safety architecture but needs operational maturity. This plan addresses critical gaps.

---

## Phase 1: Critical Fixes (Week 1)

### 1.1 Mandatory Test Generation
**Priority: CRITICAL**
- **Current Issue**: Evolution makes code changes without tests
- **Solution**: Add requirement that every code fix must include tests
- **Implementation**:
  - Update `internal/evolution/prompts.go` to require test generation
  - Add `hasTestChanges()` verification like `hasCodeChanges()`
  - Fail evolution if tests not included

### 1.2 Code Review Before Merge
**Priority: CRITICAL**
- **Current Issue**: PR #16 merged with critical bugs (files closed before use)
- **Solution**: Human-in-the-loop or enhanced AI review
- **Implementation**:
  - Add `REQUIRE_HUMAN_APPROVAL` flag for code changes
  - Or improve AI review to catch obvious bugs
  - Block merge if review finds issues

### 1.3 Fix Current Bugs
**Priority: CRITICAL**
- Fix `features.go` bugs introduced by evolution #16
- Verify all current code works

---

## Phase 2: Testing Infrastructure (Week 2)

### 2.1 Sandboxed Test Execution
**Priority: HIGH**
- **Current Issue**: Tests run in same environment (dangerous)
- **Solution**: Docker-based test isolation
- **Implementation**:
  - Create `Dockerfile.test`
  - Run `go test` in container
  - Mount repo as read-only, copy for writing

### 2.2 Test-First Enforcement
**Priority: HIGH**
- Learn from Aider: "Write test first"
- Implementation:
  - Add skill: `/test-first` workflow
  - Prompt: "Write test that fails → Fix code → Verify test passes"
  - Track test files created vs code files modified

### 2.3 Coverage Regression Detection
**Priority: MEDIUM**
- Track coverage per evolution
- Fail if coverage drops
- Store in `memory/coverage_history.jsonl`

---

## Phase 3: Benchmarking & Evaluation (Week 3)

### 3.1 SWE-bench Lite Integration
**Priority: HIGH**
- **Why**: Industry standard for evaluating coding agents
- Devin scored 13.86%, iterate should measure baseline
- **Implementation**:
  - Add `scripts/benchmark/swe_bench.py`
  - Run on subset of GitHub issues
  - Track success rate over time

### 3.2 Self-Benchmarking
**Priority: MEDIUM**
- Create synthetic bugs in test repo
- See if evolution can fix them
- Track metrics: bug detection rate, fix success rate, test quality

### 3.3 Performance Dashboard
**Priority: LOW**
- Add metrics to health dashboard:
  - Evolution success rate
  - Bug detection rate
  - Test coverage trend
  - SWE-bench score

---

## Phase 4: Code Quality & Context (Week 4)

### 4.1 Codebase Mapping
**Priority: HIGH**
- Learn from Aider: Map entire repo for context
- **Implementation**:
  - Create `internal/analysis/repo_map.go`
  - Build tree of files, functions, types
  - Provide to agent for better context

### 4.2 Static Analysis Integration
**Priority: MEDIUM**
- Run `go vet`, `staticcheck`, `gosec` on all changes
- Block merge if issues found
- Auto-fix where possible

### 4.3 Code Review Skills
**Priority: MEDIUM**
- Create specialized review agent
- Check for common bugs:
  - defer in loops
  - ignored errors
  - nil pointer risks
  - resource leaks

---

## Phase 5: Multi-Agent Architecture (Month 2)

### 5.1 Role Separation
**Priority: MEDIUM**
- Learn from MetaGPT: Separate roles
- **Roles**:
  - **Planner**: Analyzes codebase, plans fix
  - **Implementer**: Writes code
  - **Tester**: Writes tests
  - **Reviewer**: Reviews code

### 5.2 Cross-Agent Review
**Priority: MEDIUM**
- Agents review each other's work
- Block if reviewer finds issues
- Consensus required for merge

### 5.3 Specialized Agents
**Priority: LOW**
- Bug-finder agent (analyzes only)
- Test-writer agent
- Documentation agent
- Performance agent

---

## Phase 6: Learning & Memory (Month 2)

### 6.1 Failure Pattern Recognition
**Priority: MEDIUM**
- Track common failure modes
- Update prompts to avoid them
- Example: "Remember to close files AFTER use, not before"

### 6.2 Cross-Project Learning
**Priority: LOW**
- Learn from other Go repos
- Import common patterns
- Avoid common anti-patterns

### 6.3 Self-Assessment Improvements
**Priority: MEDIUM**
- Before evolution: Check past failures
- Update plan based on learnings
- Don't repeat mistakes

---

## Success Metrics

### Phase 1 Success:
- [ ] No code changes without tests
- [ ] No critical bugs merged
- [ ] All current bugs fixed

### Phase 2 Success:
- [ ] Tests run in Docker container
- [ ] Test-first workflow working
- [ ] Coverage tracked per evolution

### Phase 3 Success:
- [ ] SWE-bench Lite running
- [ ] Baseline score established
- [ ] Performance dashboard live

### Phase 4 Success:
- [ ] Repo map generated
- [ ] Static analysis passing
- [ ] Review agent catching bugs

### Phase 5 Success:
- [ ] Multi-agent roles working
- [ ] Cross-review implemented
- [ ] Fewer bugs in output

### Phase 6 Success:
- [ ] Learning from failures
- [ ] Not repeating mistakes
- [ ] Improved success rate

---

## Implementation Order

### This Week (Critical):
1. ✅ Fix features.go bugs (DONE)
2. Add mandatory test generation
3. Add human approval gate for code changes

### Next Week (Testing):
4. Docker-based test sandbox
5. Test-first workflow
6. Coverage tracking

### Week 3 (Benchmarking):
7. SWE-bench integration
8. Self-benchmarking
9. Performance dashboard

### Week 4 (Quality):
10. Repo mapping
11. Static analysis
12. Review agent

### Month 2 (Advanced):
13. Multi-agent architecture
14. Learning systems

---

## Risk Mitigation

### Risk: Evolution breaks things
**Mitigation**: 
- Protected files (already implemented)
- Mandatory tests (Phase 1)
- Sandboxed testing (Phase 2)
- Human approval (Phase 1)

### Risk: Tests don't catch bugs
**Mitigation**:
- Static analysis (Phase 4)
- Code review agent (Phase 4)
- SWE-bench validation (Phase 3)

### Risk: Evolution too slow
**Mitigation**:
- Multi-agent parallelism (Phase 5)
- Caching (already implemented)
- Incremental improvements

---

## Conclusion

This roadmap transforms iterate from "safety-first but buggy" to "safety-first AND reliable". The key is Phase 1 - without mandatory tests and human approval, we risk more buggy code like PR #16.

**Start with Phase 1 immediately.**
