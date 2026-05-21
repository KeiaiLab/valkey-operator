## PR Title Convention

Follow **Conventional Commits**: `type(scope): description`

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style (formatting, whitespace)
- `refactor`: Code refactoring
- `test`: Test additions/updates
- `chore`: Maintenance tasks

**Examples:**
- `feat(controller): add support for arbiter nodes`
- `fix(backup): handle S3 connection timeout`
- `docs(readme): update installation instructions`

---

## Description

Provide a clear, concise summary of your changes:

**Why is this change needed?**

**What does this change do?**

**How was it tested?**

---

## Type of Change

- [ ] Bug fix (non-breaking change fixing an issue)
- [ ] New feature (non-breaking change adding functionality)
- [ ] Breaking change (fix or feature that breaks existing functionality)
- [ ] Documentation update
- [ ] Code refactoring (no functional change)
- [ ] Performance improvement
- [ ] Tests (test coverage or test improvements)

---

## Related Issues

Link to related issue(s): Closes #issue-number

---

## Checklist

- [ ] Tests added/updated for new functionality
- [ ] Documentation updated (README, godoc, Helm chart) if applicable
- [ ] All tests pass locally: `make test`
- [ ] Code follows Go style guidelines: `make lint`
- [ ] Commit messages follow Conventional Commits format
- [ ] Self-review completed (checked for typos, clarity)
- [ ] PR title follows Conventional Commits convention

---

## Additional Notes

Any additional context, screenshots, or links to relevant discussions.

---

**📚 For detailed guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md)**
