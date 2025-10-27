# Open Source Release Checklist

This checklist ensures the repository is ready for public release.

## ✅ Pre-Release Preparation

### Legal & Licensing
- [x] **LICENSE file** - MIT License added
- [x] **Copyright notices** - Year 2025, attribution correct
- [x] **Third-party licenses** - All dependencies use permissive licenses (checked go.mod)

### Security
- [x] **No sensitive data** - Scanned for API keys, credentials, secrets
- [x] **Environment variables** - All examples use env vars for credentials
- [x] **SECURITY.md** - Security policy and reporting process documented
- [x] **.gitignore** - Comprehensive ignore patterns for sensitive files
- [ ] **Security audit** - Consider running `gosec` before release

### Documentation
- [x] **README.md** - Complete, accurate, up-to-date
- [x] **CONTRIBUTING.md** - Contribution guidelines present
- [x] **CLAUDE.md** - Project context for AI assistance
- [x] **User guides** - All 8 guides in docs/guides/ complete
- [x] **API documentation** - docs/api/ present
- [x] **FAQ.md** - Common questions documented
- [x] **Examples** - 11 working examples with proper documentation

### Code Quality
- [x] **All tests pass** - `make test` succeeds
- [x] **Code builds** - `make build` succeeds
- [x] **Examples build** - `make examples` succeeds (11/11)
- [x] **No build artifacts** - Cleaned with `make clean`
- [ ] **Linter passes** - Run `make lint` (if golangci-lint available)
- [ ] **Code formatting** - Run `make fmt`
- [ ] **Go vet passes** - Run `make vet`

### Repository Structure
- [x] **Makefile** - Comprehensive build system
- [x] **go.mod** - Proper module path and dependencies
- [x] **Package structure** - Clear organization (graph/, examples/, docs/)
- [x] **.gitignore** - Comprehensive ignore patterns
- [x] **No local artifacts** - .specify/memory/local/ ignored

## ✅ Verification Steps

### Functionality Tests
- [x] **Unit tests** - All graph/* tests pass
- [x] **Integration tests** - MySQL integration tests pass
- [x] **Benchmark tests** - Benchmarks run successfully
- [ ] **Example verification** - Manually run each example (optional)

### Documentation Tests
- [x] **README examples** - Quick Start code is accurate
- [x] **Links work** - All documentation links are valid
- [x] **Code samples** - All code samples in docs compile

### Security Verification
- [x] **No hardcoded secrets** - Verified all examples use env vars
- [x] **Dependency security** - No known vulnerabilities in dependencies
- [x] **SQL injection** - MySQL store uses parameterized queries

## 🚀 Release Process

### Pre-Release
1. [ ] Create release branch: `git checkout -b release/v0.1.0`
2. [ ] Update version references in documentation
3. [ ] Run full test suite: `make test`
4. [ ] Run linter: `make lint`
5. [ ] Build all examples: `make examples`
6. [ ] Review CHANGELOG (if exists) or create one

### Tagging
1. [ ] Create annotated git tag: `git tag -a v0.1.0 -m "Release v0.1.0"`
2. [ ] Push tag: `git push origin v0.1.0`
3. [ ] Verify tag on GitHub

### GitHub Release
1. [ ] Create GitHub release from tag
2. [ ] Write release notes highlighting:
   - Core features implemented
   - Breaking changes (if any)
   - Known limitations
   - Migration guide (if needed)
3. [ ] Attach any release artifacts (optional)

### Post-Release
1. [ ] Announce on relevant channels (if applicable)
2. [ ] Update project status in README if needed
3. [ ] Monitor issues for bug reports
4. [ ] Prepare for community contributions

## 📋 Issue Templates

Consider adding GitHub issue templates:
- [ ] Bug report template (.github/ISSUE_TEMPLATE/bug_report.md)
- [ ] Feature request template (.github/ISSUE_TEMPLATE/feature_request.md)
- [ ] Question template (.github/ISSUE_TEMPLATE/question.md)

## 🤝 Community Setup

For better community engagement:
- [ ] Enable GitHub Discussions (optional)
- [ ] Add CODE_OF_CONDUCT.md (optional but recommended)
- [ ] Set up GitHub Actions for CI/CD (optional)
- [ ] Add status badges to README (build, coverage, etc.)

## ⚠️ Known TODOs for Future Releases

These are acceptable for initial release but should be tracked:

1. **graph/model/openai/openai.go:295** - Proper JSON parsing for tool arguments
2. **graph/store/mysql.go:415** - Reflection-based batch execution
3. **graph/node.go:42** - Events field (currently in comments, not implemented)

## 📊 Release Metrics

Track for release announcement:
- **11 examples** demonstrating all features
- **8 comprehensive guides** for users
- **8 test packages** with good coverage
- **5 LLM providers** supported (OpenAI, Anthropic, Google + Mock)
- **2 store implementations** (Memory, MySQL)
- **3 emitter types** (Log, Buffered, Null)

## ✅ Final Checks Before Public Release

1. [ ] All items marked [x] in this checklist
2. [ ] Repository is clean (`git status` shows no unexpected files)
3. [ ] All tests pass on a clean checkout
4. [ ] Documentation reviewed by at least one other person (optional)
5. [ ] Ready to accept issues and PRs from community

---

**Date Prepared:** October 27, 2025
**Release Target:** v0.1.0 (Initial Public Release)
